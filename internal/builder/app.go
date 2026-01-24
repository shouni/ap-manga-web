package builder

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"

	"github.com/shouni/gcp-kit/tasks"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	mangaKitCfg "github.com/shouni/go-manga-kit/pkg/config"
	mangaKitDom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/workflow"
	"github.com/shouni/go-remote-io/pkg/gcsfactory"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

// AppContext はアプリケーションの依存関係を保持します。
// 各フィールドをインターフェースで定義することで、将来的なモック利用を容易にします。
type AppContext struct {
	Config        config.Config
	HTTPClient    httpkit.ClientInterface
	IOFactory     remoteio.IOFactory
	Reader        remoteio.InputReader
	Writer        remoteio.OutputWriter
	Signer        remoteio.URLSigner
	TaskEnqueuer  *tasks.Enqueuer[domain.GenerateTaskPayload]
	Workflow      workflow.Workflow
	SlackNotifier adapters.SlackNotifier
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

	// 3. Cloud Tasks Enqueuer の初期化
	workerURL, err := url.JoinPath(cfg.ServiceURL, "/tasks/generate")
	if err != nil {
		return nil, fmt.Errorf("failed to build worker URL: %w", err)
	}
	taskCfg := tasks.Config{
		ProjectID:           cfg.ProjectID,
		LocationID:          cfg.LocationID,
		QueueID:             cfg.QueueID,
		WorkerURL:           workerURL,
		ServiceAccountEmail: cfg.ServiceAccountEmail,
		Audience:            cfg.ServiceURL,
	}
	taskEnqueuer, err := tasks.NewEnqueuer[domain.GenerateTaskPayload](ctx, taskCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task enqueuer: %w", err)
	}

	// 4. キャラクターマップの読み込み
	charsMap, err := mangaKitDom.LoadCharacterMap(ctx, reader, cfg.CharacterConfig)
	if err != nil {
		return nil, fmt.Errorf("キャラクターマップの読み込みに失敗しました (path: %s): %w", cfg.CharacterConfig, err)
	}

	// 5. ワークフロービルダーの構築
	args := workflow.ManagerArgs{
		Config: mangaKitCfg.Config{
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

	// 5. Slack アダプターの初期化
	slack, err := adapters.NewSlackAdapter(httpClient, cfg.SlackWebhookURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Slack adapter: %w", err)
	}

	return &AppContext{
		Config:        cfg,
		HTTPClient:    httpClient,
		IOFactory:     ioFactory,
		Reader:        reader,
		Writer:        writer,
		Signer:        signer,
		TaskEnqueuer:  taskEnqueuer,
		Workflow:      workflowManager,
		SlackNotifier: slack,
	}, nil
}

// Close は、AppContextが保持するすべてのリソース（クライアント接続など）を解放します。
func (a *AppContext) Close() {
	if a.IOFactory != nil {
		if err := a.IOFactory.Close(); err != nil {
			slog.Error("failed to close IOFactory", "error", err)
		}
	}
	if a.TaskEnqueuer != nil {
		if err := a.TaskEnqueuer.Close(); err != nil {
			slog.Error("failed to close task enqueuer", "error", err)
		}
	}
}
