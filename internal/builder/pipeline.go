package builder

import (
	"context"
	"fmt"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/app"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/pipeline"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	mangaKitCfg "github.com/shouni/go-manga-kit/pkg/config"
	mangaKitDom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/workflow"
)

// buildPipeline は、提供されたランナーを使用して新しいパイプラインを初期化して返します。
func buildPipeline(cfg *config.Config, runners *workflow.Runners, rio *app.RemoteIO, slack adapters.SlackNotifier) (pipeline.Pipeline, error) {
	p, err := pipeline.NewMangaPipeline(cfg, runners, rio.Writer, slack)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// buildWorkflow は、各 Runner を事前にビルドします。
func buildWorkflow(ctx context.Context, cfg *config.Config, httpClient httpkit.ClientInterface, rio *app.RemoteIO) (*workflow.Manager, error) {
	charsMap, err := mangaKitDom.LoadCharacterMap(ctx, rio.Reader, cfg.CharacterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load character map: %w", err)
	}

	args := workflow.ManagerArgs{
		Config: mangaKitCfg.Config{
			GeminiAPIKey:       cfg.GeminiAPIKey,
			GeminiModel:        cfg.GeminiModel,
			ImageStandardModel: cfg.ImageStandardModel,
			ImageQualityModel:  cfg.ImageQualityModel,
			StyleSuffix:        cfg.StyleSuffix,
			RateInterval:       config.DefaultRateLimit,
			MaxPanelsPerPage:   config.DefaultMaxPanelsPerPage,
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

	return mgr, nil
}
