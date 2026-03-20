package builder

import (
	"ap-manga-web/internal/app"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
	"ap-manga-web/internal/pipeline"
)

// buildPipeline は、提供されたランナーを使用して新しいパイプラインを初期化して返します。
func buildPipeline(cfg *config.Config, workflows domain.Workflows, rio *app.RemoteIO, slack domain.Notifier) (domain.Pipeline, error) {
	p, err := pipeline.NewMangaPipeline(cfg, workflows, rio.Writer, slack)
	if err != nil {
		return nil, err
	}

	return p, nil
}
