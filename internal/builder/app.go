package builder

import (
	"context"
	"fmt"
	"io"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/app"
	"ap-manga-web/internal/config"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-remote-io/remoteio/gcs"
)

// BuildContainer は外部サービスとの接続を確立し、依存関係を組み立てた app.Container を返します。
func BuildContainer(ctx context.Context, cfg *config.Config) (container *app.Container, err error) {
	var resources []io.Closer
	defer func() {
		if err != nil {
			for _, r := range resources {
				if r != nil {
					_ = r.Close()
				}
			}
		}
	}()

	// 1. I/O Infrastructure (GCS)
	storage, err := gcs.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS factory: %w", err)
	}
	resources = append(resources, storage)
	rio, err := buildRemoteIO(storage)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize IO components: %w", err)
	}

	// 2. Task Enqueuer
	enqueuer, err := buildTaskEnqueuer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task enqueuer: %w", err)
	}
	resources = append(resources, enqueuer)

	httpClient := httpkit.New(config.DefaultHTTPTimeout)
	slack, err := adapters.NewSlackAdapter(httpClient, cfg.SlackWebhookURL)
	if err != nil {
		return nil, err
	}

	geminiAI, errGemini := adapters.NewGeminiAIAdapter(ctx, cfg)
	vertexAI, errVertex := adapters.NewVertexAIAdapter(ctx, cfg)

	// 両方とも初期化に失敗した場合は起動不可
	if errGemini != nil && errVertex != nil {
		return nil, fmt.Errorf("failed to initialize any AI client: gemini err: %v, vertex err: %v", errGemini, errVertex)
	}

	// 片方のみ成功した場合は、もう一方の用途にもフォールバックとして使用する
	if geminiAI == nil {
		geminiAI = vertexAI
	}
	if vertexAI == nil {
		vertexAI = geminiAI
	}

	workflows, err := adapters.NewWorkflowsAdapter(cfg, httpClient, rio, geminiAI, vertexAI)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize manga workflow: %w", err)
	}
	// 3. Pipeline (Core Logic)
	mangaPipeline, err := buildPipeline(cfg, workflows, slack)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize manga pipeline: %w", err)
	}

	appCtx := &app.Container{
		Config:       cfg,
		RemoteIO:     rio,
		TaskEnqueuer: enqueuer,
		Pipeline:     mangaPipeline,
		HTTPClient:   httpClient,
		Notifier:     slack,
	}

	return appCtx, nil
}
