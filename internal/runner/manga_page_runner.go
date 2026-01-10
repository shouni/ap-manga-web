package runner

import (
	"context"
	"fmt"

	imagedom "github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/generator"
	"github.com/shouni/go-manga-kit/pkg/parser"
)

// PageRunner は MarkdownのパースとPipelineの実行を管理するインターフェースなのだ。
type PageRunner interface {
	Run(ctx context.Context, scriptURL, markdownContent string) ([]*imagedom.ImageResponse, error)
}

// MangaPageRunner は Markdownのパースと複数ページの生成（チャンク処理）を管理するのだ。
type MangaPageRunner struct {
	pageGen *generator.PageGenerator
}

// NewMangaPageRunner は生成エンジン、スタイル設定、パーサーを依存性として注入して初期化するのだ。
func NewMangaPageRunner(mangaGen generator.MangaGenerator, styleSuffix string) *MangaPageRunner {
	return &MangaPageRunner{
		pageGen: generator.NewPageGenerator(mangaGen, styleSuffix),
	}
}

// Run は提供された Markdown コンテンツを解析し、複数枚の漫画ページ画像を生成するのだ。
func (r *MangaPageRunner) Run(ctx context.Context, scriptURL, markdownContent string) ([]*imagedom.ImageResponse, error) {

	markdownParser := parser.NewParser(scriptURL)

	// 1. Markdown テキストをドメインモデルに変換
	//	manga, err := r.markdownParser.Parse(markdownContent)
	manga, err := markdownParser.Parse(markdownContent)
	if err != nil {
		return nil, fmt.Errorf("Markdownコンテンツのパースに失敗しました: %w", err)
	}
	if manga == nil {
		return nil, fmt.Errorf("マンガのパース結果が nil になりました")
	}

	// 2. ページ生成エンジンを実行して、画像バイナリ群を取得
	return r.pageGen.ExecuteMangaPages(ctx, *manga)
}
