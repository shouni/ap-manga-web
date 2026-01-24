package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"ap-manga-web/internal/builder"

	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/publisher"
)

// runScriptStep はスクリプト生成フェーズを実行し、生成された台本をJSONとしてGCSに保存します。
func (p *MangaPipeline) runScriptStep(ctx context.Context, exec *mangaExecution) (*mangadom.MangaResponse, string, error) {
	runner, err := builder.BuildScriptRunner(ctx, p.appCtx)
	if err != nil {
		return nil, "", err
	}

	manga, err := runner.Run(ctx, exec.payload.ScriptURL, exec.payload.Mode)
	if err != nil {
		return nil, "", err
	}

	plotFile := exec.resolvePlotFileURL(manga)
	data, err := json.MarshalIndent(manga, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal manga script to JSON: %w", err)
	}

	if err := p.appCtx.Writer.Write(ctx, plotFile, bytes.NewReader(data), "application/json"); err != nil {
		return manga, "", err
	}
	return manga, plotFile, nil
}

// runPanelStep は台本に基づき画像を生成・保存し、更新された台本を返します。
func (p *MangaPipeline) runPanelStep(ctx context.Context, manga *mangadom.MangaResponse, exec *mangaExecution) (*mangadom.MangaResponse, error) {
	runner, err := builder.BuildPanelImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, err
	}
	plotFile := exec.resolvePlotFileURL(manga)

	return runner.RunAndSave(ctx, manga, plotFile)
}

// runPublishStep は漫画データを統合し、HTML等を出力します。
func (p *MangaPipeline) runPublishStep(ctx context.Context, manga *mangadom.MangaResponse, exec *mangaExecution) (publisher.PublishResult, error) {
	runner, err := builder.BuildPublishRunner(ctx, p.appCtx)
	if err != nil {
		return publisher.PublishResult{}, err
	}

	return runner.Run(ctx, manga, exec.resolveOutputURL(manga))
}

// runPanelAndPublishSteps は一連の流れを管理します。
func (p *MangaPipeline) runPanelAndPublishSteps(ctx context.Context, manga *mangadom.MangaResponse, exec *mangaExecution) (publisher.PublishResult, error) {
	// 1. パネル生成＆保存（画像パスが書き込まれた新しい台本を受け取る）
	updatedManga, err := p.runPanelStep(ctx, manga, exec)
	if err != nil {
		return publisher.PublishResult{}, fmt.Errorf("panel generation step failed: %w", err)
	}

	// 2. パブリッシュ（更新された台本をそのまま渡すのだ）
	publishResult, err := p.runPublishStep(ctx, updatedManga, exec)
	if err != nil {
		return publisher.PublishResult{}, fmt.Errorf("publish step failed: %w", err)
	}
	return publishResult, nil
}

// runPageStep はMangaResponseからページ画像を生成します。
func (p *MangaPipeline) runPageStep(ctx context.Context, manga *mangadom.MangaResponse, exec *mangaExecution) ([]string, error) {
	if manga == nil {
		return nil, fmt.Errorf("manga data is nil")
	}
	// 1. ビルダーを介して Runner を構築
	pageRunner, err := builder.BuildPageImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, fmt.Errorf("PageImageRunnerの構築に失敗しました: %w", err)
	}

	plotFile := exec.resolvePlotFileURL(manga)
	pagePaths, err := pageRunner.RunAndSave(ctx, manga, plotFile)
	if err != nil {
		return nil, fmt.Errorf("PageImageRunner による生成と保存に失敗しました: %w", err)
	}

	return pagePaths, nil
}

// runDesignStep はデザインシート生成します。
func (p *MangaPipeline) runDesignStep(ctx context.Context, exec *mangaExecution) (string, int64, error) {
	runner, err := builder.BuildDesignRunner(ctx, p.appCtx)
	if err != nil {
		return "", 0, err
	}

	charIDs := p.parseCSV(exec.payload.InputText)
	if len(charIDs) == 0 {
		return "", 0, fmt.Errorf("キャラクターIDが必要です")
	}

	outputDir := p.appCtx.Config.GetGCSObjectURL(p.appCtx.Config.BaseOutputDir)

	return runner.Run(ctx, charIDs, exec.payload.Seed, outputDir)
}
