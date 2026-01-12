package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/domain"

	imagedom "github.com/shouni/gemini-image-kit/pkg/domain"
	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
)

func (p *MangaPipeline) runScriptStep(ctx context.Context, payload domain.GenerateTaskPayload, t time.Time) (mangadom.MangaResponse, string, error) {
	runner, err := builder.BuildScriptRunner(ctx, p.appCtx)
	if err != nil {
		return mangadom.MangaResponse{}, "", err
	}

	manga, err := runner.Run(ctx, payload.ScriptURL, payload.Mode)
	if err != nil {
		return mangadom.MangaResponse{}, "", err
	}

	outputPath := fmt.Sprintf("gs://%s/output/%s/script.json", p.appCtx.Config.GCSBucket, p.getSafeTitle(manga.Title, t))

	// MarshalIndent のエラーを適切にハンドリングするのだ
	data, err := json.MarshalIndent(manga, "", "  ")
	if err != nil {
		return mangadom.MangaResponse{}, "", fmt.Errorf("failed to marshal manga script to JSON: %w", err)
	}

	if err := p.appCtx.Writer.Write(ctx, outputPath, bytes.NewReader(data), "application/json"); err != nil {
		return manga, "", err
	}
	return manga, outputPath, nil
}

func (p *MangaPipeline) runPanelStep(ctx context.Context, manga mangadom.MangaResponse, payload domain.GenerateTaskPayload) ([]*imagedom.ImageResponse, error) {
	indices := p.parseTargetPanels(ctx, payload.TargetPanels, len(manga.Pages))
	runner, err := builder.BuildPanelImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, err
	}
	return runner.Run(ctx, manga, indices)
}

func (p *MangaPipeline) runPublishStep(ctx context.Context, manga mangadom.MangaResponse, images []*imagedom.ImageResponse, t time.Time) error {
	runner, err := builder.BuildPublishRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}
	outputDir := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, p.getSafeTitle(manga.Title, t))
	_, err = runner.Run(ctx, manga, images, outputDir)
	return err
}

func (p *MangaPipeline) runPageStepWithAsset(ctx context.Context, assetPath string, t time.Time) ([]string, error) {
	runner, err := builder.BuildPageImageRunner(ctx, p.appCtx)
	if err != nil {
		return nil, err
	}
	resps, err := runner.Run(ctx, assetPath)
	if err != nil {
		return nil, err
	}

	dir := path.Dir(assetPath)
	var savedPaths []string
	for i, resp := range resps {
		pagePath := fmt.Sprintf("%s/final_page_%d.png", dir, i+1)
		if err := p.appCtx.Writer.Write(ctx, pagePath, bytes.NewReader(resp.Data), resp.MimeType); err != nil {
			return savedPaths, err
		}
		savedPaths = append(savedPaths, pagePath)
	}
	return savedPaths, nil
}

func (p *MangaPipeline) runPageStep(ctx context.Context, payload domain.GenerateTaskPayload) error {
	runner, err := builder.BuildPageImageRunner(ctx, p.appCtx)
	if err != nil {
		return err
	}
	_, err = runner.Run(ctx, payload.ScriptURL)
	return err
}

func (p *MangaPipeline) runDesignStep(ctx context.Context, payload domain.GenerateTaskPayload, t time.Time) (string, int64, error) {
	runner, err := builder.BuildDesignRunner(ctx, p.appCtx)
	if err != nil {
		return "", 0, err
	}
	charIDs := p.parseCSV(payload.InputText)
	if len(charIDs) == 0 {
		return "", 0, fmt.Errorf("character ID required")
	}
	dir := p.getSafeTitle("design_"+strings.Join(charIDs, "_"), t)
	outputDir := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, dir)
	return runner.Run(ctx, charIDs, payload.Seed, outputDir)
}
