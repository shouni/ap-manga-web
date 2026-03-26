package adapters

import (
	"context"
	"fmt"

	"github.com/shouni/go-gemini-client/gemini"
	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-manga-kit/ports"
	"github.com/shouni/go-manga-kit/workflow"

	"ap-manga-web/assets"
	"ap-manga-web/internal/app"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
	"ap-manga-web/internal/prompts"
)

// WorkflowsAdapter は、Workflows インターフェイスをラップするアダプタ構造体です。
type WorkflowsAdapter struct {
	workflows *ports.Workflows
}

// NewWorkflowsAdapter は Workflowsを初期化します。
func NewWorkflowsAdapter(cfg *config.Config, httpClient httpkit.HTTPClient, rio *app.RemoteIO, aiClient gemini.GenerativeModel) (domain.Workflows, error) {
	promptDeps, err := buildPromptDeps(cfg.StyleSuffix)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize prompt dependencies: %w", err)
	}

	args := workflow.ManagerArgs{
		Config: ports.Config{
			GeminiModel:        cfg.GeminiModel,
			ImageStandardModel: cfg.ImageStandardModel,
			ImageQualityModel:  cfg.ImageQualityModel,
			StyleSuffix:        cfg.StyleSuffix,
			MaxConcurrency:     cfg.MaxConcurrency,
			RateInterval:       cfg.RateInterval,
			MaxPanelsPerPage:   cfg.MaxPanelsPerPage,
		},
		HTTPClient:      httpClient,
		Reader:          rio.Reader,
		Writer:          rio.Writer,
		AIClient:        aiClient,
		AIClientQuality: aiClient,
		PromptDeps:      promptDeps,
	}
	workflows, err := workflow.New(args)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflows: %w", err)
	}

	return &WorkflowsAdapter{
		workflows: workflows,
	}, nil
}

// Design は指定されたキャラクターIDのキャラクターを生成します。
func (w *WorkflowsAdapter) Design(ctx context.Context, charIDs []string, seed int64, outputDir string) (string, int64, error) {
	return w.workflows.Design.Run(ctx, charIDs, seed, outputDir)
}

// Script は指定されたURLから台本を作成します。
func (w *WorkflowsAdapter) Script(ctx context.Context, sourceURL, mode string) (*ports.MangaResponse, error) {
	return w.workflows.Script.Run(ctx, sourceURL, mode)
}

// Panel は指定された漫画のページを保存します。
func (w *WorkflowsAdapter) Panel(ctx context.Context, manga *ports.MangaResponse, outputPath string) (*ports.MangaResponse, error) {
	return w.workflows.PanelImage.RunAndSave(ctx, manga, outputPath)
}

// Page は指定された漫画のページを保存します。
func (w *WorkflowsAdapter) Page(ctx context.Context, manga *ports.MangaResponse, outputPath string) ([]string, error) {
	return w.workflows.PageImage.RunAndSave(ctx, manga, outputPath)
}

// Publish は指定された漫画を公開します。
func (w *WorkflowsAdapter) Publish(ctx context.Context, manga *ports.MangaResponse, outputDir string) (*ports.PublishResult, error) {
	return w.workflows.Publish.Run(ctx, manga, outputDir)
}

// buildPromptDeps は Prompt ビルダーを初期化します。
func buildPromptDeps(styleSuffix string) (*workflow.PromptDeps, error) {
	templates, err := assets.LoadPrompts()
	if err != nil {
		return nil, fmt.Errorf("プロンプトテンプレートの読み込みに失敗しました: %w", err)
	}

	textPrompt, err := prompts.NewBuilder(templates)
	if err != nil {
		return nil, fmt.Errorf("failed to create text prompt builder: %w", err)
	}

	charMap, err := assets.LoadCharacters()
	if err != nil {
		return nil, fmt.Errorf("failed to generate character map: %w", err)
	}
	imagePrompt := prompts.NewImageBuilder(charMap, styleSuffix)

	return &workflow.PromptDeps{
		CharactersMap: charMap,
		ScriptPrompt:  textPrompt,
		ImagePrompt:   imagePrompt,
	}, nil
}
