package builder

import (
	"context"
	"fmt"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	mangaKitCfg "github.com/shouni/go-manga-kit/pkg/config"
	"github.com/shouni/go-manga-kit/pkg/workflow"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/app"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
	"ap-manga-web/internal/pipeline"
)

// buildPipeline は、提供されたランナーを使用して新しいパイプラインを初期化して返します。
func buildPipeline(cfg *config.Config, runners *workflow.Runners, rio *app.RemoteIO, slack domain.Notifier) (domain.Pipeline, error) {
	p, err := pipeline.NewMangaPipeline(cfg, runners, rio.Writer, slack)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// buildWorkflow は、各 Runner を事前にビルドします。
func buildWorkflow(ctx context.Context, cfg *config.Config, httpClient httpkit.HTTPClient, rio *app.RemoteIO) (*workflow.Manager, error) {
	aiClient, err := adapters.NewAIAdapter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create aiClient: %w", err)
	}
	promptDeps, err := adapters.NewPromptDependencies(ctx, rio.Reader, cfg.CharacterConfig, cfg.StyleSuffix)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize prompt dependencies: %w", err)
	}

	args := workflow.ManagerArgs{
		Config: mangaKitCfg.Config{
			GeminiModel:        cfg.GeminiModel,
			ImageStandardModel: cfg.ImageStandardModel,
			ImageQualityModel:  cfg.ImageQualityModel,
			StyleSuffix:        cfg.StyleSuffix,
			MaxConcurrency:     cfg.MaxConcurrency,
			RateInterval:       cfg.RateInterval,
			MaxPanelsPerPage:   cfg.MaxPanelsPerPage,
		},
		HTTPClient:    httpClient,
		Reader:        rio.Reader,
		Writer:        rio.Writer,
		AIClient:      aiClient,
		CharactersMap: promptDeps.CharactersMap,
		ScriptPrompt:  promptDeps.ScriptPrompt,
		ImagePrompt:   promptDeps.ImagePrompt,
	}

	mgr, err := workflow.New(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow manager: %w", err)
	}

	return mgr, nil
}
