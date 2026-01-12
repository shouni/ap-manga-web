package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/domain"

	imagedom "github.com/shouni/gemini-image-kit/pkg/domain"
	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
)

type MangaPipeline struct {
	appCtx *builder.AppContext
}

func NewMangaPipeline(appCtx *builder.AppContext) *MangaPipeline {
	return &MangaPipeline{appCtx: appCtx}
}

func (p *MangaPipeline) Execute(ctx context.Context, payload domain.GenerateTaskPayload) error {
	slog.Info("Pipeline execution started", "command", payload.Command, "mode", payload.Mode)
	executionTime := time.Now()

	var notificationReq *domain.NotificationRequest
	var publicURL, storageURI string
	var err error
	var manga mangadom.MangaResponse
	var images []*imagedom.ImageResponse

	switch payload.Command {
	case "generate":
		if manga, _, err = p.runScriptStep(ctx, payload, executionTime); err != nil {
			return err
		}
		if images, err = p.runPanelStep(ctx, manga, payload); err != nil {
			return err
		}
		if err = p.runPublishStep(ctx, manga, images, executionTime); err != nil {
			return err
		}

		safeTitle := p.getSafeTitle(manga.Title, executionTime)
		assetPath := fmt.Sprintf("gs://%s/output/%s/manga.md", p.appCtx.Config.GCSBucket, safeTitle)
		if _, err = p.runPageStepWithAsset(ctx, assetPath, executionTime); err != nil {
			slog.ErrorContext(ctx, "Page generation failed", "error", err)
		}
		notificationReq, publicURL, storageURI = p.buildMangaNotification(payload, manga, executionTime)

	case "design":
		var outputURL string
		var finalSeed int64
		outputURL, finalSeed, err = p.runDesignStep(ctx, payload, executionTime)
		if err == nil {
			notificationReq, publicURL, storageURI = p.buildDesignNotification(payload, outputURL, finalSeed)
		}

	case "script":
		var scriptPath string
		manga, scriptPath, err = p.runScriptStep(ctx, payload, executionTime)
		if err == nil {
			notificationReq, publicURL, storageURI = p.buildScriptNotification(payload, manga, scriptPath)
		}

	case "panel":
		if err = json.Unmarshal([]byte(payload.InputText), &manga); err != nil {
			return nil
		}
		if images, err = p.runPanelStep(ctx, manga, payload); err != nil {
			return err
		}
		if err = p.runPublishStep(ctx, manga, images, executionTime); err == nil {
			notificationReq, publicURL, storageURI = p.buildMangaNotification(payload, manga, executionTime)
		}

	case "page":
		err = p.runPageStep(ctx, payload)

	default:
		return fmt.Errorf("unsupported command: %s", payload.Command)
	}

	if err != nil {
		return err
	}
	if notificationReq != nil {
		if notifyErr := p.appCtx.SlackNotifier.Notify(ctx, publicURL, storageURI, *notificationReq); notifyErr != nil {
			slog.ErrorContext(ctx, "Notification failed", "error", notifyErr)
		}
	}
	return nil
}
