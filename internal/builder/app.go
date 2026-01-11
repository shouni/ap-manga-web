package builder

import (
	"context"
	"fmt"
	"io"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	mngkitCfg "github.com/shouni/go-manga-kit/pkg/config"
	"github.com/shouni/go-manga-kit/pkg/workflow"
	"github.com/shouni/go-remote-io/pkg/gcsfactory"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

// AppContext は、アプリケーション実行に必要な共通コンテキストを保持する
// これを各Build関数に渡すことで、依存関係の注入を簡素化します。
type AppContext struct {
	// Config は環境変数から読み込まれたグローバル設定です。
	Config config.Config
	// Reader は外部データソースからの読み込みに使用します。
	Reader remoteio.InputReader
	// Writer は生成されたコンテンツの書き込みに使用します。
	Writer remoteio.OutputWriter
	// wkBuilder はマンガ生成ワークフロービルダーを提供します。
	wkBuilder *workflow.Builder
	// SlackNotifier はslack通知のアダプターです。
	SlackNotifier adapters.SlackNotifier
	// aiClient はGemini APIとの通信に使用するクライアントです。
	aiClient gemini.GenerativeModel
	// httpClient は外部HTTPリソースへのアクセスに使用するクライアントです。
	httpClient httpkit.ClientInterface
}

// NewAppContext は AppContext の新しいインスタンスを生成する
func NewAppContext(
	cfg config.Config,
	httpClient httpkit.ClientInterface,
	aiClient gemini.GenerativeModel,
	reader remoteio.InputReader,
	writer remoteio.OutputWriter,
	wkBuilder *workflow.Builder,
	slackNotifier adapters.SlackNotifier,

) AppContext {
	return AppContext{
		Config:        cfg,
		aiClient:      aiClient,
		httpClient:    httpClient,
		Reader:        reader,
		Writer:        writer,
		wkBuilder:     wkBuilder,
		SlackNotifier: slackNotifier,
	}
}

// BuildAppContext は、アプリケーションの実行に必要な依存関係（HTTPクライアント、AIクライアント、GCS I/Oなど）を
// 初期化し、AppContextとして返します。
// この関数は、各コマンドやパイプライン実行の前に呼び出され、一貫したコンテキストを提供します。
func BuildAppContext(ctx context.Context, cfg config.Config) (*AppContext, error) {
	httpClient := httpkit.New(config.DefaultHTTPTimeout)
	aiClient, err := initializeAIClient(ctx, cfg.GeminiAPIKey)
	if err != nil {
		return nil, fmt.Errorf("AI clientの初期化に失敗しました: %w", err)
	}

	gcsFactory, err := gcsfactory.NewGCSClientFactory(ctx)
	if err != nil {
		return nil, fmt.Errorf("GCS client factoryの初期化に失敗しました: %w", err)
	}

	reader, err := gcsFactory.NewInputReader()
	if err != nil {
		return nil, err
	}
	writer, err := gcsFactory.NewOutputWriter()
	if err != nil {
		return nil, err
	}

	// 2. CharacterConfig を []byte として読み込み
	characterConfig := cfg.CharacterConfig
	rc, err := reader.Open(ctx, characterConfig)
	if err != nil {
		return nil, fmt.Errorf("キャラクター設定ファイルのオープンに失敗しました (path: %s): %w", characterConfig, err)
	}
	defer rc.Close()

	charData, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("キャラクター設定ファイルの読み込みに失敗しました (path: %s): %w", characterConfig, err)
	}

	// 3. Workflow Builder の初期化
	// workflow パッケージ内でのマッピング例
	wfCfg := mngkitCfg.Config{
		GeminiAPIKey: cfg.GeminiAPIKey,
		GeminiModel:  cfg.GeminiModel,
		ImageModel:   cfg.ImageModel,
		//		StyleSuffix:  cfg.StyleSuffix, // TODO::あと設定する
		RateInterval: config.DefaultRateLimit,
	}
	wfBuilder, err := workflow.NewBuilder(
		wfCfg,
		httpClient,
		aiClient,
		reader,
		writer,
		charData,
	)

	slack, err := adapters.NewSlackAdapter(httpClient, cfg.SlackWebhookURL)
	if err != nil {
		return nil, fmt.Errorf("SlackAdapterの初期化に失敗しました: %w", err)
	}

	appCtx := NewAppContext(cfg, httpClient, aiClient, reader, writer, wfBuilder, slack)
	return &appCtx, nil
}
