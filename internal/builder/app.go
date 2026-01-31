package builder

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/app"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"

	"github.com/shouni/gcp-kit/tasks"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	mangaKitCfg "github.com/shouni/go-manga-kit/pkg/config"
	mangaKitDom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/workflow"
	"github.com/shouni/go-remote-io/pkg/gcsfactory"
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

	// 1. HttpClient (全アダプターの基盤)
	httpClient := httpkit.New(config.DefaultHTTPTimeout)

	// 2. I/O Infrastructure (GCS)
	rio, err := buildRemoteIO(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize IO components: %w", err)
	}
	resources = append(resources, rio)

	// 3. Task Enqueuer
	enqueuer, err := buildTaskEnqueuer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task enqueuer: %w", err)
	}
	resources = append(resources, enqueuer)

	// 4. Workflow (Core Logic)
	wf, err := buildWorkflow(ctx, cfg, httpClient, rio)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	// 5. Slack Adapter
	slack, err := adapters.NewSlackAdapter(httpClient, cfg.SlackWebhookURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Slack adapter: %w", err)
	}

	return &app.Container{
		Config:        cfg,
		RemoteIO:      rio,
		TaskEnqueuer:  enqueuer,
		Workflow:      wf,
		HTTPClient:    httpClient,
		SlackNotifier: slack,
	}, nil
}

// buildRemoteIO は、GCS ベースの I/O コンポーネントを初期化します。
func buildRemoteIO(ctx context.Context) (*app.RemoteIO, error) {
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
	return &app.RemoteIO{
		Factory: factory,
		Reader:  r,
		Writer:  w,
		Signer:  s,
	}, nil
}

// buildTaskEnqueuer は、Cloud Tasks エンキューアを初期化します。
func buildTaskEnqueuer(ctx context.Context, cfg *config.Config) (*tasks.Enqueuer[domain.GenerateTaskPayload], error) {
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

// buildWorkflow は、各 Runner を事前にビルドします。
func buildWorkflow(ctx context.Context, cfg *config.Config, httpClient httpkit.ClientInterface, rio *app.RemoteIO) (*workflow.Runners, error) {
	charsMap, err := mangaKitDom.LoadCharacterMap(ctx, rio.Reader, cfg.CharacterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load character map: %w", err)
	}

	args := workflow.ManagerArgs{
		Config: mangaKitCfg.Config{
			GeminiAPIKey:     cfg.GeminiAPIKey,
			GeminiModel:      cfg.GeminiModel,
			ImageModel:       cfg.ImageModel,
			StyleSuffix:      cfg.StyleSuffix,
			RateInterval:     config.DefaultRateLimit,
			MaxPanelsPerPage: config.DefaultMaxPanelsPerPage,
		},
		HTTPClient:    httpClient,
		Reader:        rio.Reader,
		Writer:        rio.Writer,
		CharactersMap: charsMap,
	}

	mgr, err := workflow.New(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow manager: %w", err)
	}

	return mgr.BuildRunners()
}
