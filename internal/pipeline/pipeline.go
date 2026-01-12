package pipeline

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	slog.Info("Pipeline execution started",
		"command", payload.Command,
		"mode", payload.Mode,
	)

	// 実行開始時の時刻を固定
	executionTime := time.Now()

	var notificationReq *domain.NotificationRequest
	var publicURL string
	var storageURI string
	var err error

	var manga mngdom.MangaResponse
	var images []*imagedom.ImageResponse
	var scriptPath string

	switch payload.Command {
	case "generate":
		if manga, scriptPath, err = p.runScriptStep(ctx, payload, executionTime); err != nil {
			return err
		}
		if images, err = p.runPanelStep(ctx, manga, payload); err != nil {
			return err
		}
		if err = p.runPublishStep(ctx, manga, images, executionTime); err == nil {
			notificationReq, publicURL, storageURI = p.buildMangaNotification(payload, manga, executionTime)
		}

	case "design":
		var outputURL string
		var finalSeed int64
		outputURL, finalSeed, err = p.runDesignStep(ctx, payload, executionTime)
		if err == nil {
			notificationReq, publicURL, storageURI = p.buildDesignNotification(payload, outputURL, finalSeed)
		}

	case "script":
		manga, scriptPath, err = p.runScriptStep(ctx, payload, executionTime)
		if err == nil {
			notificationReq, publicURL, storageURI = p.buildScriptNotification(payload, manga, scriptPath)
		}

	case "panel":
		if err = json.Unmarshal([]byte(payload.InputText), &manga); err != nil {
			slog.WarnContext(ctx, "Failed to parse input JSON for panel mode", "error", err)
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

// --- 内部ステップ群 ---

func (p *MangaPipeline) runScriptStep(ctx context.Context, payload domain.GenerateTaskPayload, t time.Time) (mngdom.MangaResponse, string, error) {
	slog.Info("Step: Script generation", "url", payload.ScriptURL)

	runner, err := builder.BuildScriptRunner(ctx, p.appCtx)
	if err != nil {
		return mngdom.MangaResponse{}, "", err
	}

	manga, err := runner.Run(ctx, payload.ScriptURL, payload.Mode)
	if err != nil {
		return mngdom.MangaResponse{}, "", err
	}

	safeTitle := p.getSafeTitle(manga.Title, t)
	outputPath := fmt.Sprintf("gs://%s/output/%s/script.json", p.appCtx.Config.GCSBucket, safeTitle)

	data, err := json.MarshalIndent(manga, "", "  ")
	if err != nil {
		return manga, "", fmt.Errorf("script JSON serialization failed: %w", err)
	}
	if err := p.appCtx.Writer.Write(ctx, outputPath, bytes.NewReader(data), "application/json"); err != nil {
		return manga, "", fmt.Errorf("script saving failed: %w", err)
	}

	slog.Info("Script JSON saved", "path", outputPath)
	return manga, outputPath, nil
}

func (p *MangaPipeline) runPanelStep(ctx context.Context, manga mngdom.MangaResponse, payload domain.GenerateTaskPayload) ([]*imagedom.ImageResponse, error) {
	targetIndices := p.parseTargetPanels(ctx, payload.TargetPanels, len(manga.Pages))

	slog.Info("Step: Panel image generation",
		"target_count", len(targetIndices),
		"indices", targetIndices,
	)

	runner, err := builder.BuildPanelImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, err
	}
	return runner.Run(ctx, manga, targetIndices)
}

func (p *MangaPipeline) runDesignStep(ctx context.Context, payload domain.GenerateTaskPayload, t time.Time) (string, int64, error) {
	slog.Info("Step: Design sheet generation")

	runner, err := builder.BuildDesignRunner(ctx, p.appCtx)
	if err != nil {
		return "", 0, err
	}

	charIDs := p.parseCSV(payload.InputText)
	if len(charIDs) == 0 {
		return "", 0, fmt.Errorf("at least one character ID is required")
	}

	titleForDir := fmt.Sprintf("design_%s", strings.Join(charIDs, "_"))
	safeDirName := p.getSafeTitle(titleForDir, t)
	outputDir := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, safeDirName)

	return runner.Run(ctx, charIDs, payload.Seed, outputDir)
}

func (p *MangaPipeline) runPageStep(ctx context.Context, payload domain.GenerateTaskPayload) error {
	slog.Info("Step: Page image generation", "asset_path", payload.ScriptURL)

	runner, err := builder.BuildPageImageRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}
	_, err = runner.Run(ctx, payload.ScriptURL)
	return err
}

func (p *MangaPipeline) runPublishStep(ctx context.Context, manga mngdom.MangaResponse, images []*imagedom.ImageResponse, t time.Time) error {
	slog.Info("Step: Publishing")

	runner, err := builder.BuildPublishRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}

	safeTitle := p.getSafeTitle(manga.Title, t)
	outputDir := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, safeTitle)
	_, err = runner.Run(ctx, manga, images, outputDir)
	return err
}

// --- ヘルパー関数 ---

func (p *MangaPipeline) parseTargetPanels(ctx context.Context, panelsStr string, totalPanels int) []int {
	trimmedStr := strings.TrimSpace(panelsStr)
	if trimmedStr == "" {
		indices := make([]int, totalPanels)
		for i := 0; i < totalPanels; i++ {
			indices[i] = i
		}
		return indices
	}

	var targetIndices []int
	parts := strings.Split(trimmedStr, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		idx, err := strconv.Atoi(trimmed)
		if err != nil {
			slog.WarnContext(ctx, "Invalid panel index found in target_panels, skipping", "input", trimmed)
			continue
		}
		if idx >= 0 && idx < totalPanels {
			targetIndices = append(targetIndices, idx)
		}
	}
	return targetIndices
}

func (p *MangaPipeline) getSafeTitle(title string, t time.Time) string {
	h := md5.New()
	io.WriteString(h, title)
	hash := fmt.Sprintf("%x", h.Sum(nil))[:8]
	nowStr := t.Format("20060102_150405")
	return fmt.Sprintf("%s_%s", nowStr, hash)
}

func (p *MangaPipeline) parseCSV(input string) []string {
	raw := strings.Split(input, ",")
	var result []string
	for _, s := range raw {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// --- 通知ビルダ ---

func (p *MangaPipeline) buildMangaNotification(payload domain.GenerateTaskPayload, manga mngdom.MangaResponse, t time.Time) (*domain.NotificationRequest, string, string) {
	safeTitle := p.getSafeTitle(manga.Title, t)
	publicURL, _ := url.JoinPath(p.appCtx.Config.ServiceURL, "outputs", safeTitle)
	storageURI := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, safeTitle)

	return &domain.NotificationRequest{
		SourceURL:      payload.ScriptURL,
		OutputCategory: "manga-output",
		TargetTitle:    manga.Title,
		ExecutionMode:  payload.Command + " / " + payload.Mode,
	}, publicURL, storageURI
}

func (p *MangaPipeline) buildScriptNotification(payload domain.GenerateTaskPayload, manga mngdom.MangaResponse, scriptPath string) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      payload.ScriptURL,
		OutputCategory: "script-json",
		TargetTitle:    manga.Title,
		ExecutionMode:  "script-only",
	}, "N/A", scriptPath
}

func (p *MangaPipeline) buildDesignNotification(payload domain.GenerateTaskPayload, url string, seed int64) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      "N/A (Character Design)",
		OutputCategory: "design-sheet",
		TargetTitle:    fmt.Sprintf("Design: %s (Seed: %d)", payload.InputText, seed),
		ExecutionMode:  "design",
	}, url, "N/A"
}
