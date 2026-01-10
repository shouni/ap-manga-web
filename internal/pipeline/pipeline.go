package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/domain"

	imagedom "github.com/shouni/gemini-image-kit/pkg/domain"
	mngdom "github.com/shouni/go-manga-kit/pkg/domain"
)

var invalidPathChars = regexp.MustCompile(`[\\/:\*\?"<>\|]`)

type MangaPipeline struct {
	appCtx *builder.AppContext
}

func NewMangaPipeline(appCtx *builder.AppContext) *MangaPipeline {
	return &MangaPipeline{
		appCtx: appCtx,
	}
}

func (p *MangaPipeline) Execute(ctx context.Context, payload domain.GenerateTaskPayload) error {
	slog.Info("Pipeline started", "command", payload.Command, "mode", payload.Mode)

	var err error
	var manga mngdom.MangaResponse
	var images []*imagedom.ImageResponse

	switch payload.Command {
	case "generate":
		if manga, err = p.runScriptStep(ctx, payload); err != nil {
			return err
		}
		if images, err = p.runImageStep(ctx, manga, payload.PanelLimit); err != nil {
			return err
		}
		err = p.runPublishStep(ctx, manga, images)

	case "design":
		err = p.runDesignStep(ctx, payload)

	case "script":
		_, err = p.runScriptStep(ctx, payload)

	case "image":
		if err := json.Unmarshal([]byte(payload.InputText), &manga); err != nil {
			slog.WarnContext(ctx, "Failed to parse input JSON for image mode. Task will not be retried.",
				"error", err, "command", payload.Command)
			return nil
		}
		if images, err = p.runImageStep(ctx, manga, payload.PanelLimit); err != nil {
			return err
		}
		err = p.runPublishStep(ctx, manga, images)

	case "story":
		err = p.runStoryStep(ctx, payload)

	default:
		return fmt.Errorf("unsupported command: %s", payload.Command)
	}

	// 全ての処理が成功した後に Slack 通知を送るのだ
	if err == nil && payload.Command != "design" {
		p.notifySlack(ctx, payload, manga)
	}

	return err
}

// --- 内部ステップ群 ---

func (p *MangaPipeline) runScriptStep(ctx context.Context, payload domain.GenerateTaskPayload) (mngdom.MangaResponse, error) {
	slog.Info("Phase: Script generation starting...")
	runner, err := builder.BuildScriptRunner(ctx, p.appCtx)
	if err != nil {
		return mngdom.MangaResponse{}, err
	}
	return runner.Run(ctx, payload.ScriptURL, payload.Mode)
}

func (p *MangaPipeline) runImageStep(ctx context.Context, manga mngdom.MangaResponse, limit int) ([]*imagedom.ImageResponse, error) {
	slog.Info("Phase: Image generation starting...", "panels", len(manga.Pages))
	runner, err := builder.BuildImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, err
	}
	return runner.Run(ctx, manga, limit)
}

func (p *MangaPipeline) runDesignStep(ctx context.Context, payload domain.GenerateTaskPayload) error {
	slog.Info("Phase: Design sheet generation starting...")
	runner, err := builder.BuildDesignRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}

	rawIDs := strings.Split(payload.InputText, ",")
	var charIDs []string
	for _, id := range rawIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			charIDs = append(charIDs, trimmed)
		}
	}

	outputURL, finalSeed, err := runner.Run(
		ctx,
		charIDs,
		payload.Seed,
		p.appCtx.Config.GCSBucket,
	)
	if err != nil {
		return err
	}

	// 生成された Seed と URL を使って Slack に通知するのだ
	p.notifySlackForDesign(ctx, payload, outputURL, finalSeed)

	return nil
}

func (p *MangaPipeline) runStoryStep(ctx context.Context, payload domain.GenerateTaskPayload) error {
	slog.Info("Phase: Story starting...")
	runner, err := builder.BuildMangaPageRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}
	_, err = runner.Run(ctx, payload.ScriptURL, payload.InputText)
	return err
}

func (p *MangaPipeline) runPublishStep(ctx context.Context, manga mngdom.MangaResponse, images []*imagedom.ImageResponse) error {
	slog.Info("Phase: Publishing...")
	runner, err := builder.BuildPublisherRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}

	safeTitle := invalidPathChars.ReplaceAllString(manga.Title, "_")
	if safeTitle == "" {
		safeTitle = "untitled"
	}
	outputDir := fmt.Sprintf("output/%s", safeTitle)

	return runner.Run(ctx, manga, images, "index.html", outputDir)
}

// notifySlack は生成完了を Slack に通知するヘルパーなのだ
func (p *MangaPipeline) notifySlack(ctx context.Context, payload domain.GenerateTaskPayload, manga mngdom.MangaResponse) {
	// 通知メッセージ用のリンク作成（構成に合わせて調整してほしいのだ）
	publicURL := fmt.Sprintf("%s/outputs/%s", p.appCtx.Config.ServiceURL, manga.Title)
	storageURI := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, manga.Title)

	// SlackAdapter の Notify インターフェースに合わせて情報を詰めるのだ
	err := p.appCtx.SlackNotifier.Notify(ctx, publicURL, storageURI, domain.NotificationRequest{
		SourceURL:      payload.ScriptURL,
		OutputCategory: "manga-output",
		TargetTitle:    manga.Title,
		ExecutionMode:  payload.Command + " / " + payload.Mode,
	})

	if err != nil {
		slog.Error("Failed to send slack notification", "error", err)
	}
}

// notifySlackForDesign design 生成完了を Slack に通知するヘルパーなのだ
func (p *MangaPipeline) notifySlackForDesign(ctx context.Context, payload domain.GenerateTaskPayload, url string, seed int64) {
	err := p.appCtx.SlackNotifier.Notify(ctx, url, "Google Cloud Storage", domain.NotificationRequest{
		SourceURL:      "N/A (Character Design)",
		OutputCategory: "design-sheet",
		TargetTitle:    fmt.Sprintf("Design: %s (Seed: %d)", payload.InputText, seed),
		ExecutionMode:  "design",
	})

	if err != nil {
		slog.Error("Failed to send slack notification", "error", err)
	}
}
