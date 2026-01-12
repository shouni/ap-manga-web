package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"strings"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/domain"

	imagedom "github.com/shouni/gemini-image-kit/pkg/domain"
	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/publisher"
)

// runScriptStep はスクリプト生成フェーズを実行するのだ
func (p *MangaPipeline) runScriptStep(ctx context.Context, payload domain.GenerateTaskPayload) (mangadom.MangaResponse, string, error) {
	runner, err := builder.BuildScriptRunner(ctx, p.appCtx)
	if err != nil {
		return mangadom.MangaResponse{}, "", err
	}

	manga, err := runner.Run(ctx, payload.ScriptURL, payload.Mode)
	if err != nil {
		return mangadom.MangaResponse{}, "", err
	}

	// 内部で一貫したタイムスタンプを持つディレクトリ名を生成するのだ
	outputPath := fmt.Sprintf("gs://%s/output/%s/script.json", p.appCtx.Config.GCSBucket, p.resolveSafeTitle(manga.Title))

	data, err := json.MarshalIndent(manga, "", "  ")
	if err != nil {
		return mangadom.MangaResponse{}, "", fmt.Errorf("failed to marshal manga script to JSON: %w", err)
	}

	if err := p.appCtx.Writer.Write(ctx, outputPath, bytes.NewReader(data), "application/json"); err != nil {
		return manga, "", err
	}
	return manga, outputPath, nil
}

// runPanelStep は各パネルの画像を生成するのだ
func (p *MangaPipeline) runPanelStep(ctx context.Context, manga mangadom.MangaResponse, payload domain.GenerateTaskPayload) ([]*imagedom.ImageResponse, error) {
	indices := p.parseTargetPanels(ctx, payload.TargetPanels, len(manga.Pages))
	runner, err := builder.BuildPanelImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, err
	}
	return runner.Run(ctx, manga, indices)
}

// runPublishStep は生成された画像と台本をパブリッシュするのだ
func (p *MangaPipeline) runPublishStep(ctx context.Context, manga mangadom.MangaResponse, images []*imagedom.ImageResponse) (publisher.PublishResult, error) {
	runner, err := builder.BuildPublishRunner(ctx, p.appCtx)
	if err != nil {
		return publisher.PublishResult{}, err
	}

	safeTitle := p.resolveSafeTitle(manga.Title)
	outputDir := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, safeTitle)

	return runner.Run(ctx, manga, images, outputDir)
}

// runPageStepWithAsset は既存のアセット（Markdown）から最終ページ画像を生成するのだ
func (p *MangaPipeline) runPageStepWithAsset(ctx context.Context, assetPath string) ([]string, error) {
	runner, err := builder.BuildPageImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build page image runner: %w", err)
	}

	resps, err := runner.Run(ctx, assetPath)
	if err != nil {
		return nil, fmt.Errorf("page runner failed for asset %s: %w", assetPath, err)
	}

	// URLとしてパースして gs:// を保護したまま親ディレクトリを取得するのだ
	u, err := url.Parse(assetPath)
	if err != nil {
		return nil, fmt.Errorf("invalid asset path format: %w", err)
	}
	u.Path = path.Dir(u.Path)
	dir := u.String()

	var savedPaths []string
	for i, resp := range resps {
		pagePath := fmt.Sprintf("%s/final_page_%d.png", dir, i+1)

		slog.InfoContext(ctx, "Saving final page image", "index", i+1, "path", pagePath)

		if err := p.appCtx.Writer.Write(ctx, pagePath, bytes.NewReader(resp.Data), resp.MimeType); err != nil {
			return savedPaths, fmt.Errorf("failed to write page image to %s: %w", pagePath, err)
		}
		savedPaths = append(savedPaths, pagePath)
	}

	return savedPaths, nil
}

// runPageStep はURLから直接ページ生成を行うのだ
func (p *MangaPipeline) runPageStep(ctx context.Context, payload domain.GenerateTaskPayload) error {
	runner, err := builder.BuildPageImageRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}
	_, err = runner.Run(ctx, payload.ScriptURL)
	return err
}

// runDesignStep はキャラクターのデザインシートを生成するのだ
func (p *MangaPipeline) runDesignStep(ctx context.Context, payload domain.GenerateTaskPayload) (string, int64, error) {
	runner, err := builder.BuildDesignRunner(ctx, p.appCtx)
	if err != nil {
		return "", 0, err
	}
	charIDs := p.parseCSV(payload.InputText)
	if len(charIDs) == 0 {
		return "", 0, fmt.Errorf("character ID required")
	}

	// ★修正ポイント：path.Join ではなく strings.Join を使用してフラットなディレクトリ名を作るのだ！
	dirName := "design_" + strings.Join(charIDs, "_")
	dir := p.resolveSafeTitle(dirName)

	outputDir := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, dir)
	return runner.Run(ctx, charIDs, payload.Seed, outputDir)
}
