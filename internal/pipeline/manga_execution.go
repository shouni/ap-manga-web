package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"ap-manga-web/internal/domain"

	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/publisher"
)

// mangaExecution は一回のリクエスト実行に関する状態（開始時刻や生成されたタイトルなど）を保持します。
type mangaExecution struct {
	pipeline          *MangaPipeline
	payload           domain.GenerateTaskPayload
	startTime         time.Time
	resolvedSafeTitle string
}

// run は各生成フェーズを順番に実行し、結果を通知します。
func (e *mangaExecution) run(ctx context.Context) (err error) {
	var manga mangadom.MangaResponse

	// 失敗時の通知を defer 文で一括管理します。
	defer func() {
		if err != nil {
			// manga.Title が取得できている場合は、エラー通知のコンテキストとして利用します。
			e.pipeline.notifyError(ctx, e.payload, err, manga.Title)
		}
	}()

	slog.Info("Pipeline execution started", "command", e.payload.Command, "mode", e.payload.Mode)

	var notificationReq *domain.NotificationRequest
	var publicURL, storageURI string
	var publishResult publisher.PublishResult // switch 前に宣言を共通化し、冗長性を排除します。

	switch e.payload.Command {
	case "generate":
		// --- Phase 1: Script Phase ---
		// 実行コンテキスト (e) を介して、一貫したディレクトリ名でスクリップトを保存します。
		if manga, _, err = e.pipeline.runScriptStep(ctx, e); err != nil {
			return fmt.Errorf("script step failed: %w", err)
		}

		// --- Phase 2 & 3: Panel & Publish ---
		publishResult, err = e.pipeline.runPanelAndPublishSteps(ctx, manga, e)
		if err != nil {
			return err
		}

		// --- Phase 4: Page Generation Phase ---
		if _, err = e.pipeline.runPageStepWithAsset(ctx, publishResult.MarkdownPath); err != nil {
			slog.ErrorContext(ctx, "Page generation failed", "error", err)
			return fmt.Errorf("page generation step failed: %w", err)
		}

		notificationReq, publicURL, storageURI = e.buildMangaNotification(manga, publishResult)

	case "design":
		var outputURL string
		var finalSeed int64
		outputURL, finalSeed, err = e.pipeline.runDesignStep(ctx, e)
		if err != nil {
			return fmt.Errorf("design step failed: %w", err)
		}
		notificationReq, publicURL, storageURI = e.buildDesignNotification(outputURL, finalSeed)

	case "script":
		var scriptPath string
		manga, scriptPath, err = e.pipeline.runScriptStep(ctx, e)
		if err != nil {
			return fmt.Errorf("script step failed: %w", err)
		}
		notificationReq, publicURL, storageURI = e.buildScriptNotification(manga, scriptPath)

	case "panel":
		if err = json.Unmarshal([]byte(e.payload.InputText), &manga); err != nil {
			slog.ErrorContext(ctx, "Failed to parse input JSON for panel mode", "error", err)
			return fmt.Errorf("panel mode input JSON unmarshal failed: %w", err)
		}

		publishResult, err = e.pipeline.runPanelAndPublishSteps(ctx, manga, e)
		if err != nil {
			return err
		}

		notificationReq, publicURL, storageURI = e.buildMangaNotification(manga, publishResult)

	case "page":
		if err = e.pipeline.runPageStep(ctx, e.payload); err != nil {
			return fmt.Errorf("page step failed: %w", err)
		}

	default:
		err = fmt.Errorf("unsupported command: %s", e.payload.Command)
		return err
	}

	// 成功時の共通通知処理を行います。
	if notificationReq != nil {
		if notifyErr := e.pipeline.appCtx.SlackNotifier.Notify(ctx, publicURL, storageURI, *notificationReq); notifyErr != nil {
			slog.ErrorContext(ctx, "Notification failed", "error", notifyErr)
			// 通知処理自体の失敗は、パイプライン全体の成否には影響させません。
		}
	}

	return nil
}
