package runner

import (
	"context"
	"log/slog"

	"ap-manga-web/internal/config"

	imagedom "github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/generator"
)

// ImageRunner は、漫画の台本データを基に画像を生成するためのインターフェース。
type ImageRunner interface {
	Run(ctx context.Context, manga domain.MangaResponse, limit int) ([]*imagedom.ImageResponse, error)
}

// MangaImageRunner は、Webリクエストから渡された台本を元に並列画像生成を管理します。
type MangaImageRunner struct {
	groupGen *generator.GroupGenerator // 並列生成とレートリミットを管理するコア
}

// NewMangaImageRunner は、依存関係を注入して初期化します。
func NewMangaImageRunner(mangaGen generator.MangaGenerator, styleSuffix string) *MangaImageRunner {
	// 1秒間に生成するリクエスト数などは config から取得するようにします
	groupGen := generator.NewGroupGenerator(mangaGen, styleSuffix, config.DefaultRateLimit)

	return &MangaImageRunner{
		groupGen: groupGen,
	}
}

// Run は、台本(MangaResponse)を受け取り、各ページの VisualAnchor から画像を生成します。
func (r *MangaImageRunner) Run(ctx context.Context, manga domain.MangaResponse, limit int) ([]*imagedom.ImageResponse, error) {
	pages := manga.Pages

	// 1. パネル数制限の適用
	if limit > 0 && len(pages) > limit {
		slog.Info("Applying panel limit", "limit", limit, "total", len(pages))
		pages = pages[:limit]
	}

	slog.Info("Starting parallel image generation",
		"title", manga.Title,
		"count", len(pages),
	)

	// 2. 既存の manga-kit のロジックに委譲
	images, err := r.groupGen.ExecutePanelGroup(ctx, pages)
	if err != nil {
		slog.Error("Image generation pipeline failed", "error", err)
		return nil, err
	}

	slog.Info("Successfully generated all panels", "count", len(images))
	return images, nil
}
