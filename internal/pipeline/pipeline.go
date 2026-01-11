package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"path"
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

	var notificationReq *domain.NotificationRequest
	var publicURL string
	var storageURI string
	var err error

	var manga mngdom.MangaResponse
	var images []*imagedom.ImageResponse
	var scriptPath string

	switch payload.Command {
	case "generate":
		// Phase 1: Script (台本生成と保存)
		if manga, scriptPath, err = p.runScriptStep(ctx, payload); err != nil {
			return err
		}
		// Phase 2: Image (パネル画像生成 - TargetPanelsを考慮)
		if images, err = p.runImageStep(ctx, manga, payload); err != nil {
			return err
		}
		// Phase 3: Publish (成果物の統合・公開)
		if err = p.runPublishStep(ctx, manga, images); err == nil {
			notificationReq, publicURL, storageURI = p.buildMangaNotification(payload, manga)
		}

	case "design":
		var outputURL string
		var finalSeed int64
		outputURL, finalSeed, err = p.runDesignStep(ctx, payload)
		if err == nil {
			notificationReq, publicURL, storageURI = p.buildDesignNotification(payload, outputURL, finalSeed)
		}

	case "script":
		manga, scriptPath, err = p.runScriptStep(ctx, payload)
		if err == nil {
			notificationReq, publicURL, storageURI = p.buildScriptNotification(payload, manga, scriptPath)
		}

	case "image":
		// 入力がJSON(台本)であることを想定
		if err = json.Unmarshal([]byte(payload.InputText), &manga); err != nil {
			slog.WarnContext(ctx, "Failed to parse input JSON for image mode", "error", err)
			return nil
		}
		if images, err = p.runImageStep(ctx, manga, payload); err != nil {
			return err
		}
		if err = p.runPublishStep(ctx, manga, images); err == nil {
			notificationReq, publicURL, storageURI = p.buildMangaNotification(payload, manga)
		}

	case "story":
		err = p.runStoryStep(ctx, payload)

	default:
		return fmt.Errorf("unsupported command: %s", payload.Command)
	}

	if err != nil {
		return err
	}

	// Slackへの通知実行
	if notificationReq != nil {
		if notifyErr := p.appCtx.SlackNotifier.Notify(ctx, publicURL, storageURI, *notificationReq); notifyErr != nil {
			slog.ErrorContext(ctx, "Notification failed", "error", notifyErr)
		}
	}

	return nil
}

// --- 内部ステップ群 (workflow.Runner インターフェース準拠) ---

func (p *MangaPipeline) runScriptStep(ctx context.Context, payload domain.GenerateTaskPayload) (mngdom.MangaResponse, string, error) {
	slog.Info("Step: Script generation", "url", payload.ScriptURL)

	runner, err := builder.BuildScriptRunner(ctx, p.appCtx)
	if err != nil {
		return mngdom.MangaResponse{}, "", err
	}

	manga, err := runner.Run(ctx, payload.ScriptURL, payload.Mode)
	if err != nil {
		return mngdom.MangaResponse{}, "", err
	}

	// 成果物(JSON)の保存 - bytes.NewReaderでio.Readerに変換するのだ
	safeTitle := p.getSafeTitle(manga.Title)
	outputPath := path.Join("output", safeTitle, "script.json")
	data, _ := json.MarshalIndent(manga, "", "  ")
	if err := p.appCtx.Writer.Write(ctx, outputPath, bytes.NewReader(data), "application/json"); err != nil {
		return manga, "", fmt.Errorf("script saving failed: %w", err)
	}

	slog.Info("Script JSON saved", "path", outputPath)
	return manga, outputPath, nil
}

func (p *MangaPipeline) runImageStep(ctx context.Context, manga mngdom.MangaResponse, payload domain.GenerateTaskPayload) ([]*imagedom.ImageResponse, error) {
	var targetIndices []int

	// TargetPanels文字列をパースして[]intに変換するのだ
	if payload.TargetPanels != "" {
		parts := strings.Split(payload.TargetPanels, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			idx, err := strconv.Atoi(trimmed)
			if err == nil && idx >= 0 && idx < len(manga.Pages) {
				targetIndices = append(targetIndices, idx)
			}
		}
	}

	// 指定がない場合は、上限なく全パネルを対象とするのだ
	if len(targetIndices) == 0 {
		for i := 0; i < len(manga.Pages); i++ {
			targetIndices = append(targetIndices, i)
		}
	}

	slog.Info("Step: Image generation",
		"target_count", len(targetIndices),
		"indices", targetIndices,
	)

	runner, err := builder.BuildPanelImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, err
	}
	return runner.Run(ctx, manga, targetIndices)
}

func (p *MangaPipeline) runDesignStep(ctx context.Context, payload domain.GenerateTaskPayload) (string, int64, error) {
	slog.Info("Step: Design sheet generation")

	runner, err := builder.BuildDesignRunner(ctx, p.appCtx)
	if err != nil {
		return "", 0, err
	}

	charIDs := p.parseCSV(payload.InputText)
	if len(charIDs) == 0 {
		return "", 0, fmt.Errorf("at least one character ID is required")
	}

	return runner.Run(ctx, charIDs, payload.Seed, p.appCtx.Config.GCSBucket)
}

func (p *MangaPipeline) runStoryStep(ctx context.Context, payload domain.GenerateTaskPayload) error {
	slog.Info("Step: Page image generation (Story)", "asset_path", payload.ScriptURL)

	runner, err := builder.BuildPageImageRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}
	_, err = runner.Run(ctx, payload.ScriptURL)
	return err
}

func (p *MangaPipeline) runPublishStep(ctx context.Context, manga mngdom.MangaResponse, images []*imagedom.ImageResponse) error {
	slog.Info("Step: Publishing")

	runner, err := builder.BuildPublishRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}

	outputDir := path.Join("output", p.getSafeTitle(manga.Title))
	// publisher.PublishResultを受け取ってエラーチェックをするのだ
	_, err = runner.Run(ctx, manga, images, outputDir)
	return err
}

// --- ヘルパー関数 ---

func (p *MangaPipeline) getSafeTitle(title string) string {
	safe := invalidPathChars.ReplaceAllString(title, "_")
	if safe == "" {
		return fmt.Sprintf("untitled_%d", time.Now().Unix())
	}
	return safe
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

func (p *MangaPipeline) buildMangaNotification(payload domain.GenerateTaskPayload, manga mngdom.MangaResponse) (*domain.NotificationRequest, string, string) {
	safeTitle := p.getSafeTitle(manga.Title)
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
	storageURI := fmt.Sprintf("gs://%s/%s", p.appCtx.Config.GCSBucket, scriptPath)
	return &domain.NotificationRequest{
		SourceURL:      payload.ScriptURL,
		OutputCategory: "script-json",
		TargetTitle:    manga.Title,
		ExecutionMode:  "script-only",
	}, "N/A", storageURI
}

func (p *MangaPipeline) buildDesignNotification(payload domain.GenerateTaskPayload, url string, seed int64) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      "N/A (Character Design)",
		OutputCategory: "design-sheet",
		TargetTitle:    fmt.Sprintf("Design: %s (Seed: %d)", payload.InputText, seed),
		ExecutionMode:  "design",
	}, url, "N/A"
}
