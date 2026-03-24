package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/shouni/go-manga-kit/ports"
)

// runScriptStep はスクリプト生成フェーズを実行し、生成された台本をJSONとしてGCSに保存します。
func (e *mangaExecution) runScriptStep(ctx context.Context) (*ports.MangaResponse, string, error) {
	manga, err := e.workflows.Script(ctx, e.payload.ScriptURL, e.payload.Mode)
	if err != nil {
		return nil, "", fmt.Errorf("ScriptRunnerの実行に失敗しました: %w", err)
	}

	plotFile := e.resolvePlotFileURL(manga)
	data, err := json.MarshalIndent(manga, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal manga script to JSON: %w", err)
	}

	if err := e.writer.Write(ctx, plotFile, bytes.NewReader(data), "application/json"); err != nil {
		return manga, "", err
	}
	return manga, plotFile, nil
}

// runPanelStep は台本に基づき画像を生成・保存し、更新された台本を返します。
func (e *mangaExecution) runPanelStep(ctx context.Context, manga *ports.MangaResponse) (*ports.MangaResponse, error) {
	plotFile := e.resolvePlotFileURL(manga)

	return e.workflows.Panel(ctx, manga, plotFile)
}

// runPublishStep は漫画データを統合し、HTML等を出力します。
func (e *mangaExecution) runPublishStep(ctx context.Context, manga *ports.MangaResponse) (*ports.PublishResult, error) {
	return e.workflows.Publish(ctx, manga, e.resolveOutputURL(manga))
}

// runPanelAndPublishSteps は一連の流れを管理します。
func (e *mangaExecution) runPanelAndPublishSteps(ctx context.Context, manga *ports.MangaResponse) (*ports.PublishResult, error) {
	// 1. パネル生成＆保存（画像パスが書き込まれた新しい台本を受け取る）
	updatedManga, err := e.runPanelStep(ctx, manga)
	if err != nil {
		return nil, fmt.Errorf("panel generation step failed: %w", err)
	}

	// 2. パブリッシュ（更新された台本をそのまま渡す）
	publishResult, err := e.runPublishStep(ctx, updatedManga)
	if err != nil {
		return nil, fmt.Errorf("publish step failed: %w", err)
	}
	return publishResult, nil
}

// runPageStep はMangaResponseからページ画像を生成します。
func (e *mangaExecution) runPageStep(ctx context.Context, manga *ports.MangaResponse) ([]string, error) {
	if manga == nil {
		return nil, fmt.Errorf("manga data is nil")
	}
	plotFile := e.resolvePlotFileURL(manga)
	pagePaths, err := e.workflows.Page(ctx, manga, plotFile)
	if err != nil {
		return nil, fmt.Errorf("PageImageRunner による生成と保存に失敗しました: %w", err)
	}

	return pagePaths, nil
}

// runDesignStep はデザインシート生成します。
func (e *mangaExecution) runDesignStep(ctx context.Context) (string, int64, error) {
	charIDs := e.parseCSV(e.payload.InputText)
	if len(charIDs) == 0 {
		return "", 0, fmt.Errorf("キャラクターIDが必要です")
	}

	outputDir := e.cfg.GetGCSObjectURL(e.cfg.BaseOutputDir)

	return e.workflows.Design(ctx, charIDs, e.payload.Seed, outputDir)
}
