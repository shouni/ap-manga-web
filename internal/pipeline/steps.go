package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/domain"

	imagedom "github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/asset"
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

	// config のヘルパーを使用してパスを決定
	safeTitle := exec.resolveSafeTitle(manga.Title)
	workDir := p.appCtx.Config.GetWorkDir(safeTitle)
	outputObjectPath := fmt.Sprintf("%s/script.json", workDir)
	outputFullURL := p.appCtx.Config.GetGCSObjectURL(outputObjectPath)

	data, err := json.MarshalIndent(manga, "", "  ")
	if err != nil {
		return mangadom.MangaResponse{}, "", fmt.Errorf("failed to marshal manga script to JSON: %w", err)
	}

	if err := p.appCtx.Writer.Write(ctx, outputFullURL, bytes.NewReader(data), "application/json"); err != nil {
		return manga, "", err
	}
	return manga, outputFullURL, nil
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
	workDir := p.appCtx.Config.GetWorkDir(safeTitle)
	outputFullURL := p.appCtx.Config.GetGCSObjectURL(workDir)

	return runner.Run(ctx, manga, images, outputFullURL)
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

// runPageStepWithAsset は既存のMarkdownを基に、最終的なページ画像を生成します。
func (p *MangaPipeline) runPageStepWithAsset(ctx context.Context, plotMarkdownPath string) ([]string, error) {
	pageRunner, err := builder.BuildPageImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, fmt.Errorf("PageImageRunnerの構築に失敗しました: %w", err)
	}
	imageDir, err := asset.ResolveOutputPath(plotMarkdownPath, asset.DefaultImageDir)
	if err != nil {
		return nil, fmt.Errorf("画像出力ディレクトリの解決に失敗しました: %w", err)
	}
	pagePaths, err := pageRunner.RunAndSave(ctx, plotMarkdownPath, imageDir)
	if err != nil {
		return nil, fmt.Errorf("漫画ページの生成と保存に失敗しました: %w", err)
	}

	return pagePaths, nil
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

	workDir := p.appCtx.Config.GetWorkDir(dir)
	outputFullURL := p.appCtx.Config.GetGCSObjectURL(workDir)

	return runner.Run(ctx, charIDs, exec.payload.Seed, outputFullURL)
}
