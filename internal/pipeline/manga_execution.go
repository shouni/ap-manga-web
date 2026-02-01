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
	var manga *mangadom.MangaResponse
	var scriptPath string

	// 失敗時の通知を defer 文で一括管理します。
	defer func() {
		if err != nil {
			titleHint := ""
			if manga != nil {
				titleHint = manga.Title
			}
			if titleHint == "" && e.payload.ScriptURL != "" {
				titleHint = fmt.Sprintf("Source: %s", e.payload.ScriptURL)
			}
			e.pipeline.notifyError(ctx, e.payload, err, titleHint)
		}
	}()

	slog.Info("Pipeline execution started", "command", e.payload.Command, "mode", e.payload.Mode)

	var notificationReq *domain.NotificationRequest
	var publicURL, storageURI string
	var publishResult publisher.PublishResult

	switch e.payload.Command {
	case "generate":
		// --- Phase 1: Script Phase ---
		// スクリプトを保存し、そのパスを受け取る。
		if manga, scriptPath, err = e.pipeline.runScriptStep(ctx, e); err != nil {
			return fmt.Errorf("script step failed: %w", err)
		}

		// --- Phase 2 & 3: Panel Generation & Publish ---
		// パネル生成と成果物のパブリッシュを連続して実行します。
		publishResult, err = e.pipeline.runPanelAndPublishSteps(ctx, manga, e)
		if err != nil {
			return err
		}

		// --- Phase 4: Page Generation Phase ---
		// 最終的なページ画像を構成する。
		if _, err = e.pipeline.runPageStep(ctx, manga, e); err != nil {
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
		if e.payload.InputText != "" {
			if err = json.Unmarshal([]byte(e.payload.InputText), &manga); err != nil {
				return fmt.Errorf("page mode input JSON unmarshal failed: %w", err)
			}
		}
		// mangaがnilの場合、処理を続行できないためエラーを返す。
		if manga == nil {
			return fmt.Errorf("page mode requires manga data in InputText")
		}
		if _, err = e.pipeline.runPageStep(ctx, manga, e); err != nil {
			return fmt.Errorf("page step failed: %w", err)
		}

		publishResult, err = e.pipeline.runPublishStep(ctx, manga, e)
		if err != nil {
			return err
		}

		notificationReq, publicURL, storageURI = e.buildMangaNotification(manga, publishResult)

	default:
		err = fmt.Errorf("unsupported command: %s", e.payload.Command)
		return err
	}

	// 成功時の共通通知処理を行います。
	if notificationReq != nil {
		if notifyErr := e.pipeline.slack.Notify(ctx, publicURL, storageURI, *notificationReq); notifyErr != nil {
			slog.ErrorContext(ctx, "Notification failed", "error", notifyErr)
			// 通知処理自体の失敗は、パイプライン全体の成否には影響させません。
		}
	}

	return nil
}
