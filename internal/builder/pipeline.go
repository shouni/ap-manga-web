package builder

import (
	"github.com/shouni/go-remote-io/remoteio"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
	"ap-manga-web/internal/pipeline"
)

// buildPipeline は、提供された設定と各コンポーネントを使用して新しいパイプラインを初期化して返します。
func buildPipeline(cfg *config.Config, workflows domain.Workflows, writer remoteio.OutputWriter, slack domain.Notifier) (domain.Pipeline, error) {
	p, err := pipeline.NewMangaPipeline(cfg, workflows, writer, slack)
	if err != nil {
		return nil, err
	}

	return p, nil
}
