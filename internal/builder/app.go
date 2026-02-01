package builder

import (
	"context"
	"fmt"
	"io"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/app"
	"ap-manga-web/internal/config"

	"github.com/shouni/go-http-kit/pkg/httpkit"
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
	rio, err := buildRemoteIO(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize IO components: %w", err)
	}
	resources = append(resources, rio)

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

	// 3. Pipeline (Core Logic)
	mangaPipeline, err := buildPipeline(ctx, cfg, rio, httpClient, slack)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MangaPipeline: %w", err)
	}

	appCtx := &app.Container{
		Config:       cfg,
		RemoteIO:     rio,
		TaskEnqueuer: enqueuer,
		Pipeline:     mangaPipeline,
	}

	return appCtx, nil
}
