package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"

	imagedom "github.com/shouni/gemini-image-kit/pkg/domain"
	mngdom "github.com/shouni/go-manga-kit/pkg/domain"
)

// ファイル名として安全でない文字を置換するための正規表現
var invalidPathChars = regexp.MustCompile(`[\\/:\*\?"<>\|]`)

type MangaPipeline struct {
	cfg config.Config
}

func NewMangaPipeline(cfg config.Config) *MangaPipeline {
	return &MangaPipeline{cfg: cfg}
}

// Execute は Payload の Command に応じて、適切なワークフローを実行するのだ。
func (p *MangaPipeline) Execute(ctx context.Context, payload domain.GenerateTaskPayload) error {
	appCtx, err := builder.BuildAppContext(ctx, &p.cfg)
	if err != nil {
		return fmt.Errorf("failed to build AppContext: %w", err)
	}

	slog.Info("Pipeline started", "command", payload.Command, "mode", payload.Mode)

	switch payload.Command {
	case "generate":
		manga, err := p.runScriptStep(ctx, appCtx, payload)
		if err != nil {
			return err
		}
		images, err := p.runImageStep(ctx, appCtx, manga, payload.PanelLimit)
		if err != nil {
			return err
		}
		return p.runPublishStep(ctx, appCtx, manga, images)

	case "design":
		return p.runDesignStep(ctx, appCtx, payload)

	case "script":
		_, err := p.runScriptStep(ctx, appCtx, payload)
		return err

	case "image":
		var manga mngdom.MangaResponse
		if err := json.Unmarshal([]byte(payload.InputText), &manga); err != nil {
			return fmt.Errorf("failed to parse input JSON for image mode: %w", err)
		}
		images, err := p.runImageStep(ctx, appCtx, manga, payload.PanelLimit)
		if err != nil {
			return err
		}
		return p.runPublishStep(ctx, appCtx, manga, images)

	case "story":
		return p.runStoryStep(ctx, appCtx, payload)

	default:
		return fmt.Errorf("unsupported command: %s", payload.Command)
	}
}

// --- 内部ステップ群 ---

func (p *MangaPipeline) runScriptStep(ctx context.Context, appCtx *builder.AppContext, payload domain.GenerateTaskPayload) (mngdom.MangaResponse, error) {
	slog.Info("Phase: Script generation starting...")
	runner, err := builder.BuildScriptRunner(ctx, appCtx)
	if err != nil {
		return mngdom.MangaResponse{}, err
	}
	return runner.Run(ctx, payload.ScriptURL)
}

func (p *MangaPipeline) runImageStep(ctx context.Context, appCtx *builder.AppContext, manga mngdom.MangaResponse, limit int) ([]*imagedom.ImageResponse, error) {
	slog.Info("Phase: Image generation starting...", "panels", len(manga.Pages))
	runner, err := builder.BuildImageRunner(ctx, appCtx)
	if err != nil {
		return nil, err
	}

	return runner.Run(ctx, manga, limit)
}

func (p *MangaPipeline) runDesignStep(ctx context.Context, appCtx *builder.AppContext, payload domain.GenerateTaskPayload) error {
	slog.Info("Phase: Design sheet generation starting...")
	runner, err := builder.BuildDesignRunner(ctx, appCtx)
	if err != nil {
		return err
	}
	// InputText をキャラIDのカンマ区切りとして扱う等の仕様に合わせる
	charIDs := []string{payload.InputText}
	_, _, err = runner.Run(ctx, &p.cfg, charIDs, 0, p.cfg.GCSBucket)
	return err
}

func (p *MangaPipeline) runStoryStep(ctx context.Context, appCtx *builder.AppContext, payload domain.GenerateTaskPayload) error {
	slog.Info("Phase: Story (Markdown to Pages) starting...")
	runner, err := builder.BuildMangaPageRunner(ctx, appCtx)
	if err != nil {
		return err
	}
	_, err = runner.Run(ctx, payload.ScriptURL, payload.InputText)
	return err
}

func (p *MangaPipeline) runPublishStep(ctx context.Context, appCtx *builder.AppContext, manga mngdom.MangaResponse, images []*imagedom.ImageResponse) error {
	slog.Info("Phase: Publishing...")
	runner, err := builder.BuildPublisherRunner(ctx, appCtx)
	if err != nil {
		return err
	}

	// タイトルのサニタイズ処理を追加
	safeTitle := invalidPathChars.ReplaceAllString(manga.Title, "_")
	if safeTitle == "" {
		safeTitle = "untitled"
	}
	outputDir := fmt.Sprintf("output/%s", safeTitle)

	return runner.Run(ctx, manga, images, "index.html", outputDir)
}
