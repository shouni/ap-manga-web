package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"

	imagedom "github.com/shouni/gemini-image-kit/pkg/domain"
	mngdom "github.com/shouni/go-manga-kit/pkg/domain"
)

// MangaPipeline は、CLI版のフェーズ管理ロジックをWeb版に適合させたものです。
type MangaPipeline struct {
	cfg config.Config
}

func NewMangaPipeline(cfg config.Config) *MangaPipeline {
	return &MangaPipeline{cfg: cfg}
}

// Execute は Cloud Tasks ワーカーから呼び出され、全工程を実行します。
func (p *MangaPipeline) Execute(ctx context.Context, payload domain.GenerateTaskPayload) error {
	// 1. AppContext の構築 (CLI版 builder.BuildAppContext の流用)
	appCtx, err := builder.BuildAppContext(ctx, &p.cfg)
	if err != nil {
		return fmt.Errorf("failed to build AppContext: %w", err)
	}

	// --- Phase 1: Script Phase (台本生成) ---
	// CLI版の runScriptStep と同等の処理
	manga, err := p.runScriptStep(ctx, appCtx, payload)
	if err != nil {
		return err
	}

	// --- Phase 2: Image Phase (パネル画像作成) ---
	// CLI版の runImageStep と同等の処理
	images, err := p.runImageStep(ctx, appCtx, manga)
	if err != nil {
		return err
	}

	// --- Phase 3: Publish Phase (保存・通知) ---
	// CLI版の runPublishStep と同等の処理
	err = p.runPublishStep(ctx, appCtx, manga, images)
	if err != nil {
		return err
	}

	slog.Info("漫画生成パイプラインが正常に完了しました！", "url", payload.ScriptURL)
	return nil
}

// 内部ステップ（CLI版のロジックをそのままメソッド化）
func (p *MangaPipeline) runScriptStep(ctx context.Context, appCtx *builder.AppContext, payload domain.GenerateTaskPayload) (mngdom.MangaResponse, error) {
	slog.Info("Phase 1: 台本生成を開始します...")
	scriptRunner, err := builder.BuildScriptRunner(ctx, appCtx)
	if err != nil {
		return mngdom.MangaResponse{}, err
	}
	// payload.ScriptURL を利用して解析
	return scriptRunner.Run(ctx, payload.ScriptURL)
}

func (p *MangaPipeline) runImageStep(ctx context.Context, appCtx *builder.AppContext, manga mngdom.MangaResponse) ([]*imagedom.ImageResponse, error) {
	slog.Info("Phase 2: 画像生成を開始します...", "pages", len(manga.Pages))
	imageRunner, err := builder.BuildImageRunner(ctx, appCtx)
	if err != nil {
		return nil, err
	}
	return imageRunner.Run(ctx, manga)
}

func (p *MangaPipeline) runPublishStep(ctx context.Context, appCtx *builder.AppContext, manga mngdom.MangaResponse, images []*imagedom.ImageResponse) error {
	slog.Info("Phase 3: 公開処理を開始します...")
	publishRunner, err := builder.BuildPublisherRunner(ctx, appCtx)
	if err != nil {
		return err
	}
	// TODO::パスを指定する
	return publishRunner.Run(ctx, manga, images, "test", "test")
}
