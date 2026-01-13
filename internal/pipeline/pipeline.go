package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/domain"

	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/publisher"
)

type MangaPipeline struct {
	appCtx            *builder.AppContext
	startTime         time.Time
	resolvedSafeTitle string
}

func NewMangaPipeline(appCtx *builder.AppContext) *MangaPipeline {
	return &MangaPipeline{appCtx: appCtx}
}

// Execute は名前付き戻り値 (err error) を使うことで defer 内でエラーを判定できるようにしたのだ
func (p *MangaPipeline) Execute(ctx context.Context, payload domain.GenerateTaskPayload) (err error) {
	p.startTime = time.Now()
	var manga mangadom.MangaResponse

	// 失敗時の通知を defer で一括管理する
	defer func() {
		if err != nil {
			// manga.Titleが設定されていれば、それをエラー通知に利用する
			p.notifyError(ctx, payload, err, manga.Title)
		}
	}()

	slog.Info("Pipeline execution started", "command", payload.Command, "mode", payload.Mode)

	var notificationReq *domain.NotificationRequest
	var publicURL, storageURI string

	switch payload.Command {
	case "generate":
		// --- Phase 1: Script Phase ---
		if manga, _, err = p.runScriptStep(ctx, payload); err != nil {
			return fmt.Errorf("script step failed: %w", err)
		}

		// --- Phase 2 & 3: Panel & Publish ---
		var publishResult publisher.PublishResult
		publishResult, err = p.runPanelAndPublishSteps(ctx, manga, payload)
		if err != nil {
			return err
		}

		// --- Phase 4: Page Generation Phase ---
		if _, err = p.runPageStepWithAsset(ctx, publishResult.MarkdownPath); err != nil {
			slog.ErrorContext(ctx, "Page generation failed", "error", err)
			return fmt.Errorf("page generation step failed: %w", err)
		}

		notificationReq, publicURL, storageURI = p.buildMangaNotification(payload, manga, publishResult)

	case "design":
		var outputURL string
		var finalSeed int64
		outputURL, finalSeed, err = p.runDesignStep(ctx, payload)
		if err != nil {
			return fmt.Errorf("design step failed: %w", err)
		}
		notificationReq, publicURL, storageURI = p.buildDesignNotification(payload, outputURL, finalSeed)

	case "script":
		var scriptPath string
		manga, scriptPath, err = p.runScriptStep(ctx, payload)
		if err != nil {
			return fmt.Errorf("script step failed: %w", err)
		}
		notificationReq, publicURL, storageURI = p.buildScriptNotification(payload, manga, scriptPath)

	case "panel":
		if err = json.Unmarshal([]byte(payload.InputText), &manga); err != nil {
			slog.ErrorContext(ctx, "Failed to parse input JSON for panel mode", "error", err)
			return fmt.Errorf("panel mode input JSON unmarshal failed: %w", err)
		}

		var publishResult publisher.PublishResult
		publishResult, err = p.runPanelAndPublishSteps(ctx, manga, payload)
		if err != nil {
			return err
		}

		notificationReq, publicURL, storageURI = p.buildMangaNotification(payload, manga, publishResult)

	case "page":
		if err = p.runPageStep(ctx, payload); err != nil {
			return fmt.Errorf("page step failed: %w", err)
		}

	default:
		err = fmt.Errorf("unsupported command: %s", payload.Command)
		return err
	}

	// 成功時の共通通知処理
	if notificationReq != nil {
		if notifyErr := p.appCtx.SlackNotifier.Notify(ctx, publicURL, storageURI, *notificationReq); notifyErr != nil {
			slog.ErrorContext(ctx, "Notification failed", "error", notifyErr)
			// 通知自体の失敗はパイプラインのメインエラーにはしないでおくのだ
		}
	}

	return nil
}

// --- 共通ロジックの抽出 ---

func (p *MangaPipeline) runPanelAndPublishSteps(ctx context.Context, manga mangadom.MangaResponse, payload domain.GenerateTaskPayload) (publisher.PublishResult, error) {
	images, err := p.runPanelStep(ctx, manga, payload)
	if err != nil {
		return publisher.PublishResult{}, fmt.Errorf("panel generation step failed: %w", err)
	}

	publishResult, err := p.runPublishStep(ctx, manga, images)
	if err != nil {
		return publisher.PublishResult{}, fmt.Errorf("publish step failed: %w", err)
	}
	return publishResult, nil
}
