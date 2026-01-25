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
type AppContext struct {
	Config config.Config

	// I/O and Storage
	IOFactory remoteio.IOFactory
	Reader    remoteio.InputReader
	Writer    remoteio.OutputWriter
	Signer    remoteio.URLSigner

	// Asynchronous Task
	TaskEnqueuer *tasks.Enqueuer[domain.GenerateTaskPayload]

	// Business Logic
	Workflow workflow.Workflow

	// External Adapters
	HTTPClient    httpkit.ClientInterface
	SlackNotifier adapters.SlackNotifier
}

// BuildAppContext は外部サービスとの接続を確立し、依存関係を組み立てます。
func BuildAppContext(ctx context.Context, cfg config.Config) (*AppContext, error) {
	// 1. HttpClient (全アダプターの基盤)
	httpClient := httpkit.New(config.DefaultHTTPTimeout)

	// 2. I/O Infrastructure (GCS)
	io, err := buildIO(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize IO components: %w", err)
	}

	// 3. Task Enqueuer
	enqueuer, err := buildTaskEnqueuer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task enqueuer: %w", err)
	}

	// 4. Workflow (Core Logic)
	wf, err := buildWorkflow(ctx, cfg, httpClient, io.reader, io.writer)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow builder: %w", err)
	}

	// 5. Slack Adapter
	slack, err := adapters.NewSlackAdapter(httpClient, cfg.SlackWebhookURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Slack adapter: %w", err)
	}

	return &AppContext{
		Config:        cfg,
		IOFactory:     io.factory,
		Reader:        io.reader,
		Writer:        io.writer,
		Signer:        io.signer,
		TaskEnqueuer:  enqueuer,
		Workflow:      wf,
		HTTPClient:    httpClient,
		SlackNotifier: slack,
	}, nil
}

// --- Helpers ---

type ioComponents struct {
	factory remoteio.IOFactory
	reader  remoteio.InputReader
	writer  remoteio.OutputWriter
	signer  remoteio.URLSigner
}

// buildIO は、リモート I/O 操作を処理するための ioComponents インスタンスを初期化して返します
func buildIO(ctx context.Context) (*ioComponents, error) {
	factory, err := gcsfactory.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS factory: %w", err)
	}
	r, err := factory.InputReader()
	if err != nil {
		return nil, fmt.Errorf("failed to create input reader: %w", err)
	}
	w, err := factory.OutputWriter()
	if err != nil {
		return nil, fmt.Errorf("failed to create output writer: %w", err)
	}
	s, err := factory.URLSigner()
	if err != nil {
		return nil, fmt.Errorf("failed to create URL signer: %w", err)
	}
	return &ioComponents{factory: factory, reader: r, writer: w, signer: s}, nil
}

// buildTaskEnqueuer は、Cloud Tasks エンキューアを初期化して返します。
func buildTaskEnqueuer(ctx context.Context, cfg config.Config) (*tasks.Enqueuer[domain.GenerateTaskPayload], error) {
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
		Audience:            cfg.TaskAudienceURL,
	}
	return tasks.NewEnqueuer[domain.GenerateTaskPayload](ctx, taskCfg)
}

// buildWorkflow は、ワークフローを初期化および構築します。
func buildWorkflow(ctx context.Context, cfg config.Config, httpClient httpkit.ClientInterface, reader remoteio.InputReader, writer remoteio.OutputWriter) (workflow.Workflow, error) {
	charsMap, err := mangaKitDom.LoadCharacterMap(ctx, reader, cfg.CharacterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load character map (path: %s): %w", cfg.CharacterConfig, err)
	}

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
	}
	return workflow.New(ctx, args)
}

// Close は、AppContextが保持するすべてのリソースを解放します。
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
