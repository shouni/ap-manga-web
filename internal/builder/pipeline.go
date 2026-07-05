package builder

import (
	"github.com/shouni/ap-manga-web/internal/config"
	"github.com/shouni/ap-manga-web/internal/domain"
	"github.com/shouni/ap-manga-web/internal/pipeline"
)

// buildPipeline は、提供された設定と各コンポーネントを使用して新しいパイプラインを初期化して返します。
func buildPipeline(cfg *config.Config, workflows domain.Workflows, slack domain.Notifier) (domain.Pipeline, error) {
	p, err := pipeline.NewMangaPipeline(cfg, workflows, slack)
	if err != nil {
		return nil, err
	}

	return p, nil
}
