package builder

import (
	"context"
	"fmt"

	"ap-manga-web/internal/config"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-manga-kit/pkg/generator"
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
	// MangaGenerator はマンガ生成のコア機能を提供します。
	MangaGenerator generator.MangaGenerator
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
	mangaGenerator generator.MangaGenerator,
) AppContext {
	return AppContext{
		Config:         cfg,
		aiClient:       aiClient,
		httpClient:     httpClient,
		Reader:         reader,
		Writer:         writer,
		MangaGenerator: mangaGenerator,
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

	// MangaGeneratorを一度だけ初期化
	mangaGenerator, err := initializeMangaGenerator(httpClient, aiClient, cfg.ImageModel, cfg.CharacterConfig)
	if err != nil {
		return nil, fmt.Errorf("MangaGeneratorの初期化に失敗しました: %w", err)
	}

	appCtx := NewAppContext(cfg, httpClient, aiClient, reader, writer, mangaGenerator)
	return &appCtx, nil
}
