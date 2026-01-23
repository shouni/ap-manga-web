package builder

import (
	"context"
	"fmt"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	mngkitCfg "github.com/shouni/go-manga-kit/pkg/config"
	"github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/workflow"
	"github.com/shouni/go-remote-io/pkg/gcsfactory"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

// AppContext はアプリケーションの依存関係を保持します。
// 各フィールドをインターフェースで定義することで、将来的なモック利用を容易にします。
type AppContext struct {
	Config        config.Config
	Reader        remoteio.InputReader
	Writer        remoteio.OutputWriter
	Signer        remoteio.URLSigner
	Workflow      workflow.Workflow
	SlackNotifier adapters.SlackNotifier
	HTTPClient    httpkit.ClientInterface
}

// BuildAppContext は外部サービスとの接続を確立し、依存関係を組み立てます。
func BuildAppContext(ctx context.Context, cfg config.Config) (*AppContext, error) {
	// 1. 基盤クライアントの初期化
	httpClient := httpkit.New(config.DefaultHTTPTimeout)
	// 2. I/O インフラ (GCS等) の初期化
	ioFactory, err := gcsfactory.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS factory: %w", err)
	}

	reader, err := ioFactory.InputReader()
	if err != nil {
		return nil, fmt.Errorf("failed to create input reader: %w", err)
	}
	writer, err := ioFactory.OutputWriter()
	if err != nil {
		return nil, fmt.Errorf("failed to create output writer: %w", err)
	}
	signer, err := ioFactory.URLSigner()
	if err != nil {
		return nil, fmt.Errorf("failed to create URL signer: %w", err)
	}

	// 3. キャラクター設定の読み込み
	// キャラクターマップの読み込み
	charsMap, err := domain.LoadCharacterMap(ctx, reader, cfg.CharacterConfig)
	if err != nil {
		return nil, fmt.Errorf("キャラクターマップの読み込みに失敗しました (path: %s): %w", cfg.CharacterConfig, err)
	}

	// 4. ワークフロービルダーの構築
	args := workflow.ManagerArgs{
		Config: mngkitCfg.Config{
			GeminiAPIKey: cfg.GeminiAPIKey,
			GeminiModel:  cfg.GeminiModel,
			ImageModel:   cfg.ImageModel,
			StyleSuffix:  cfg.StyleSuffix,
			RateInterval: config.DefaultRateLimit,
		},
		HTTPClient:    httpClient,
		Reader:        reader,
		Writer:        writer,
		CharactersMap: charsMap,
		ScriptPrompt:  nil,
		ImagePrompt:   nil,
	}

	workflowManager, err := workflow.New(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow builder: %w", err)
	}

	// 5. アダプターの初期化
	slack, err := adapters.NewSlackAdapter(httpClient, cfg.SlackWebhookURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Slack adapter: %w", err)
	}

	return &AppContext{
		Config:        cfg,
		Reader:        reader,
		Writer:        writer,
		Signer:        signer,
		Workflow:      workflowManager,
		SlackNotifier: slack,
		HTTPClient:    httpClient,
	}, nil
}
