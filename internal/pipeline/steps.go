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

// runScriptStep はスクリプト生成フェーズを実行し、生成された台本をJSONとしてGCSに保存します。
func (p *MangaPipeline) runScriptStep(ctx context.Context, exec *mangaExecution) (mangadom.MangaResponse, string, error) {
	runner, err := builder.BuildScriptRunner(ctx, p.appCtx)
	if err != nil {
		return mangadom.MangaResponse{}, "", err
	}

	manga, err := runner.Run(ctx, exec.payload.ScriptURL, exec.payload.Mode)
	if err != nil {
		return mangadom.MangaResponse{}, "", err
	}

	// 実行コンテキストから一貫したディレクトリ名を生成します。
	safeTitle := exec.resolveSafeTitle(manga.Title)
	outputPath := fmt.Sprintf("gs://%s/output/%s/script.json", p.appCtx.Config.GCSBucket, safeTitle)

	data, err := json.MarshalIndent(manga, "", "  ")
	if err != nil {
		return mangadom.MangaResponse{}, "", fmt.Errorf("failed to marshal manga script to JSON: %w", err)
	}

	if err := p.appCtx.Writer.Write(ctx, outputPath, bytes.NewReader(data), "application/json"); err != nil {
		return manga, "", err
	}
	return manga, outputPath, nil
}

// runPanelStep は台本に基づき、指定されたインデックスのパネル画像を生成します。
func (p *MangaPipeline) runPanelStep(ctx context.Context, manga mangadom.MangaResponse, exec *mangaExecution) ([]*imagedom.ImageResponse, error) {
	indices := p.parseTargetPanels(ctx, exec.payload.TargetPanels, len(manga.Pages))
	runner, err := builder.BuildPanelImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, err
	}
	return runner.Run(ctx, manga, indices)
}

// runPublishStep は生成された画像と台本を統合し、指定のディレクトリに出力します。
func (p *MangaPipeline) runPublishStep(ctx context.Context, manga mangadom.MangaResponse, images []*imagedom.ImageResponse, exec *mangaExecution) (publisher.PublishResult, error) {
	runner, err := builder.BuildPublishRunner(ctx, p.appCtx)
	if err != nil {
		return publisher.PublishResult{}, err
	}

	safeTitle := exec.resolveSafeTitle(manga.Title)
	outputDir := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, safeTitle)

	return runner.Run(ctx, manga, images, outputDir)
}

// runPanelAndPublishSteps はパネル画像生成とパブリッシュ処理を連続して実行する共通ロジックです。
func (p *MangaPipeline) runPanelAndPublishSteps(ctx context.Context, manga mangadom.MangaResponse, exec *mangaExecution) (publisher.PublishResult, error) {
	images, err := p.runPanelStep(ctx, manga, exec)
	if err != nil {
		return publisher.PublishResult{}, fmt.Errorf("panel generation step failed: %w", err)
	}

	publishResult, err := p.runPublishStep(ctx, manga, images, exec)
	if err != nil {
		return publisher.PublishResult{}, fmt.Errorf("publish step failed: %w", err)
	}
	return publishResult, nil
}

// runPageStepWithAsset は既存のアセット（Markdownなど）を基に、最終的なページ画像を生成します。
func (p *MangaPipeline) runPageStepWithAsset(ctx context.Context, assetPath string) ([]string, error) {
	runner, err := builder.BuildPageImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build page image runner: %w", err)
	}

	resps, err := runner.Run(ctx, assetPath)
	if err != nil {
		return nil, fmt.Errorf("page runner failed for asset %s: %w", assetPath, err)
	}

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

// runPageStep は指定されたURLから直接ページ画像を生成します。
func (p *MangaPipeline) runPageStep(ctx context.Context, payload domain.GenerateTaskPayload) error {
	runner, err := builder.BuildPageImageRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}
	_, err = runner.Run(ctx, payload.ScriptURL)
	return err
}

// runDesignStep はキャラクターのデザインシートを生成し、一意のディレクトリに保存します。
func (p *MangaPipeline) runDesignStep(ctx context.Context, exec *mangaExecution) (string, int64, error) {
	runner, err := builder.BuildDesignRunner(ctx, p.appCtx)
	if err != nil {
		return "", 0, err
	}
	charIDs := p.parseCSV(exec.payload.InputText)
	if len(charIDs) == 0 {
		return "", 0, fmt.Errorf("character ID required")
	}

	// 複数のIDを結合し、安全なディレクトリ名を構築します。
	dirName := "design_" + strings.Join(charIDs, "_")
	dir := exec.resolveSafeTitle(dirName)

	outputDir := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, dir)
	return runner.Run(ctx, charIDs, exec.payload.Seed, outputDir)
}
