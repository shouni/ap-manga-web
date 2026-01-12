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

	switch payload.Command {
	case "generate":
		// --- Phase 1: Script Phase ---
		if manga, _, err = p.runScriptStep(ctx, payload, executionTime); err != nil {
			return fmt.Errorf("script step failed: %w", err)
		}

		// --- Phase 2 & 3: Panel & Publish (共通メソッド化) ---
		publishResult, err := p.runPanelAndPublishSteps(ctx, manga, payload, executionTime)
		if err != nil {
			return err // 内部ですでにラップ済み
		}

		// --- Phase 4: Page Generation Phase ---
		if _, err = p.runPageStepWithAsset(ctx, publishResult.MarkdownPath, executionTime); err != nil {
			slog.ErrorContext(ctx, "Page generation failed", "error", err)
			return fmt.Errorf("page generation step failed: %w", err)
		}

		notificationReq, publicURL, storageURI = p.buildMangaNotification(payload, manga, publishResult, executionTime)

	case "design":
		var outputURL string
		var finalSeed int64
		outputURL, finalSeed, err = p.runDesignStep(ctx, payload, executionTime)
		if err != nil {
			return fmt.Errorf("design step failed: %w", err)
		}
		notificationReq, publicURL, storageURI = p.buildDesignNotification(payload, outputURL, finalSeed)

	case "script":
		var scriptPath string
		manga, scriptPath, err = p.runScriptStep(ctx, payload, executionTime)
		if err != nil {
			return fmt.Errorf("script step failed: %w", err)
		}
		notificationReq, publicURL, storageURI = p.buildScriptNotification(payload, manga, scriptPath)

	case "panel":
		// 入力JSONのパース
		if err = json.Unmarshal([]byte(payload.InputText), &manga); err != nil {
			slog.ErrorContext(ctx, "Failed to parse input JSON for panel mode", "error", err)
			return fmt.Errorf("panel mode input JSON unmarshal failed: %w", err)
		}

		// パネル生成と公開（共通メソッドの再利用）
		publishResult, err := p.runPanelAndPublishSteps(ctx, manga, payload, executionTime)
		if err != nil {
			return err
		}

		notificationReq, publicURL, storageURI = p.buildMangaNotification(payload, manga, publishResult, executionTime)

	case "page":
		if err = p.runPageStep(ctx, payload); err != nil {
			return fmt.Errorf("page step failed: %w", err)
		}

	default:
		return fmt.Errorf("unsupported command: %s", payload.Command)
	}

	// 共通の通知処理
	if notificationReq != nil {
		if notifyErr := p.appCtx.SlackNotifier.Notify(ctx, publicURL, storageURI, *notificationReq); notifyErr != nil {
			slog.ErrorContext(ctx, "Notification failed", "error", notifyErr)
		}
	}
	return nil
}

// --- 共通ロジックの抽出 ---

// runPanelAndPublishSteps は画像生成と公開処理をまとめて実行し、結果を返すのだ。
func (p *MangaPipeline) runPanelAndPublishSteps(ctx context.Context, manga mangadom.MangaResponse, payload domain.GenerateTaskPayload, executionTime time.Time) (publisher.PublishResult, error) {
	images, err := p.runPanelStep(ctx, manga, payload)
	if err != nil {
		return publisher.PublishResult{}, fmt.Errorf("panel generation step failed: %w", err)
	}

	publishResult, err := p.runPublishStep(ctx, manga, images, executionTime)
	if err != nil {
		return publisher.PublishResult{}, fmt.Errorf("publish step failed: %w", err)
	}
	return publishResult, nil
}
