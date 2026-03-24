package pipeline

import (
	"ap-manga-web/internal/domain"
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/shouni/go-manga-kit/ports"
)

const (
	defaultErrorTitle   = "漫画錬成エラー"
	errorReportCategory = "error-report"
)

// notifyError はエラー発生時に SlackAdapter を通じて通知を行います。
func (e *mangaExecution) notifyError(ctx context.Context, payload domain.GenerateTaskPayload, opErr error, titleHint string) {
	reqTitle := defaultErrorTitle
	if titleHint != "" {
		reqTitle = titleHint
	}

	req := domain.NotificationRequest{
		SourceURL:      payload.ScriptURL,
		OutputCategory: errorReportCategory,
		TargetTitle:    reqTitle,
		ExecutionMode:  payload.Command,
	}

	if err := e.notifier.NotifyError(ctx, opErr, req); err != nil {
		slog.ErrorContext(ctx, "Failed to send error notification", "error", err)
	}
}

// buildMangaNotification は漫画生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildMangaNotification(
	manga *ports.MangaResponse,
) (*domain.NotificationRequest, string, string) {
	safeTitle := e.resolveSafeTitle(manga.Title)
	publicURL, err := url.JoinPath(
		e.cfg.ServiceURL,
		e.cfg.BaseOutputDir,
		safeTitle,
	)
	if err != nil {
		slog.Error("Failed to construct public URL", "error", err)
		publicURL = domain.PublicURLConstructionError
	}

	workDir := e.resolveWorkDir(manga)
	storageURI := e.cfg.GetGCSObjectURL(workDir)

	return &domain.NotificationRequest{
		SourceURL:      e.payload.ScriptURL,
		OutputCategory: "manga-output",
		TargetTitle:    manga.Title,
		ExecutionMode:  e.payload.Command + " / " + e.payload.Mode,
	}, publicURL, storageURI
}

// buildScriptNotification はスクリプト生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildScriptNotification(manga *ports.MangaResponse, gcsPath string) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      e.payload.ScriptURL,
		OutputCategory: "script-json",
		TargetTitle:    manga.Title,
		ExecutionMode:  "script-only",
	}, domain.NotAvailable, gcsPath
}

// buildDesignNotification はデザインシート生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildDesignNotification(outputStorageURI string, seed int64) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      "N/A (Design)",
		OutputCategory: "design-sheet",
		TargetTitle:    fmt.Sprintf("Design: %s (Seed: %d)", e.payload.InputText, seed),
		ExecutionMode:  "design",
	}, domain.NotAvailable, outputStorageURI
}
