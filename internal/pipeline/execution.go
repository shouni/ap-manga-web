package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/shouni/go-manga-kit/ports"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

// mangaExecution は一回のリクエスト実行に関する状態を保持します。
type mangaExecution struct {
	// 実行状態
	payload           domain.GenerateTaskPayload
	startTime         time.Time
	resolvedSafeTitle string

	// 依存関係
	cfg       *config.Config
	workflows domain.Workflows
	notifier  domain.Notifier
}

// run はメインのエントリーポイントとして各コマンドにディスパッチします。
func (e *mangaExecution) run(ctx context.Context) (err error) {
	var manga *ports.MangaResponse

	// 失敗時の通知を defer で一括管理
	defer func() {
		if err != nil {
			e.handleFailure(ctx, manga, err)
		}
	}()

	slog.Info("Pipeline execution started",
		"command", e.payload.Command,
		"mode", e.payload.Mode,
	)

	var req *domain.NotificationRequest
	var publicURL, storageURI string

	// コマンドに応じたハンドラ呼び出し
	switch e.payload.Command {
	case "generate":
		req, publicURL, storageURI, manga, err = e.handleGenerate(ctx)
	case "design":
		req, publicURL, storageURI, err = e.handleDesign(ctx)
	case "script":
		req, publicURL, storageURI, manga, err = e.handleScript(ctx)
	case "panel":
		req, publicURL, storageURI, manga, err = e.handlePanel(ctx)
	case "page":
		req, publicURL, storageURI, manga, err = e.handlePage(ctx)
	default:
		return fmt.Errorf("unsupported command: %s", e.payload.Command)
	}

	if err != nil {
		return err // defer により handleFailure が呼ばれる
	}

	// 成功時の共通通知
	e.notifySuccess(ctx, req, publicURL, storageURI)
	return nil
}

// --- Command Handlers ---

// handleGenerate は スクリプト解析 -> パネル生成 -> ページ構成 のフルパイプラインを実行します。
func (e *mangaExecution) handleGenerate(ctx context.Context) (*domain.NotificationRequest, string, string, *ports.MangaResponse, error) {
	manga, _, err := e.runScriptStep(ctx)
	if err != nil {
		return nil, "", "", nil, fmt.Errorf("script step failed: %w", err)
	}

	if _, err := e.runPanelAndPublishSteps(ctx, manga); err != nil {
		return nil, "", "", manga, err
	}

	if _, err := e.runPageStep(ctx, manga); err != nil {
		return nil, "", "", manga, fmt.Errorf("page generation step failed: %w", err)
	}

	req, url, uri := e.buildMangaNotification(manga)
	return req, url, uri, manga, nil
}

// handleDesign は キャラクターのデザインシート生成を実行します。
func (e *mangaExecution) handleDesign(ctx context.Context) (*domain.NotificationRequest, string, string, error) {
	outputURL, finalSeed, err := e.runDesignStep(ctx)
	if err != nil {
		return nil, "", "", fmt.Errorf("design step failed: %w", err)
	}
	req, url, uri := e.buildDesignNotification(outputURL, finalSeed)
	return req, url, uri, nil
}

// handleScript は スクリプトの解析と保存のみを実行します。
func (e *mangaExecution) handleScript(ctx context.Context) (*domain.NotificationRequest, string, string, *ports.MangaResponse, error) {
	manga, scriptPath, err := e.runScriptStep(ctx)
	if err != nil {
		return nil, "", "", nil, fmt.Errorf("script step failed: %w", err)
	}
	req, url, uri := e.buildScriptNotification(manga, scriptPath)
	return req, url, uri, manga, nil
}

// handlePanel は 既存のJSONデータからパネル画像を生成します。
func (e *mangaExecution) handlePanel(ctx context.Context) (*domain.NotificationRequest, string, string, *ports.MangaResponse, error) {
	var manga *ports.MangaResponse
	if err := json.Unmarshal([]byte(e.payload.InputText), &manga); err != nil {
		return nil, "", "", nil, fmt.Errorf("panel mode input JSON unmarshal failed: %w", err)
	}

	if _, err := e.runPanelAndPublishSteps(ctx, manga); err != nil {
		return nil, "", "", manga, err
	}

	req, url, uri := e.buildMangaNotification(manga)
	return req, url, uri, manga, nil
}

// handlePage は 既存のパネルデータから最終ページ画像を構成します。
func (e *mangaExecution) handlePage(ctx context.Context) (*domain.NotificationRequest, string, string, *ports.MangaResponse, error) {
	var manga *ports.MangaResponse
	if e.payload.InputText != "" {
		if err := json.Unmarshal([]byte(e.payload.InputText), &manga); err != nil {
			return nil, "", "", nil, fmt.Errorf("page mode input JSON unmarshal failed: %w", err)
		}
	}
	if manga == nil {
		return nil, "", "", nil, fmt.Errorf("page mode requires manga data in InputText")
	}

	if _, err := e.runPageStep(ctx, manga); err != nil {
		return nil, "", "", manga, fmt.Errorf("page step failed: %w", err)
	}

	if _, err := e.runPublishStep(ctx, manga); err != nil {
		return nil, "", "", manga, err
	}

	req, url, uri := e.buildMangaNotification(manga)
	return req, url, uri, manga, nil
}

// --- Helper Methods ---

// handleFailure はエラー発生時の通知ロジックをカプセル化します。
func (e *mangaExecution) handleFailure(ctx context.Context, manga *ports.MangaResponse, err error) {
	titleHint := ""
	if manga != nil {
		titleHint = manga.Title
	}
	if titleHint == "" && e.payload.ScriptURL != "" {
		titleHint = fmt.Sprintf("Source: %s", e.payload.ScriptURL)
	}
	e.notifyError(ctx, e.payload, err, titleHint)
}

// notifySuccess は成功時の通知を実行します。
func (e *mangaExecution) notifySuccess(ctx context.Context, req *domain.NotificationRequest, url, uri string) {
	if req == nil {
		return
	}
	if notifyErr := e.notifier.Notify(ctx, url, uri, *req); notifyErr != nil {
		slog.ErrorContext(ctx, "Notification failed", "error", notifyErr)
	}
}
