package builder

import (
	"ap-manga-web/internal/app"
	"ap-manga-web/internal/pipeline"
)

// BuildPipeline は ReviewPipeline の新しいインスタンスを生成します。
func BuildPipeline(appCtx *app.Container) pipeline.Pipeline {
	return pipeline.NewMangaPipeline(appCtx)
}
