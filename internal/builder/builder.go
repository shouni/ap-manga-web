package builder

import (
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/prompts"
	"ap-manga-web/internal/runner"
	"context"
	"fmt"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-manga-kit/pkg/generator"
	"github.com/shouni/go-manga-kit/pkg/publisher"
	"github.com/shouni/go-text-format/pkg/builder"
	"github.com/shouni/go-web-exact/v2/pkg/extract"
	"google.golang.org/genai"
)

// BuildScriptRunner は台本テキスト生成の Runner を構築します。
func BuildScriptRunner(ctx context.Context, appCtx *AppContext) (runner.ScriptRunner, error) {
	// appCtx.httpClient (非公開) を使用してエクストラクタを初期化
	extractor, err := extract.NewExtractor(appCtx.httpClient)
	if err != nil {
		return nil, fmt.Errorf("エクストラクタの初期化に失敗しました: %w", err)
	}

	// プロンプトビルダーの作成
	promptBuilder, err := prompts.NewTextPromptBuilder()
	if err != nil {
		return nil, fmt.Errorf("Prompt Builder の構築に失敗しました: %w", err)
	}

	// 最新の引数構成に合わせて注入 (cfg, ext, pb, ai, r)
	return runner.NewMangaScriptRunner(
		*appCtx.Config,
		extractor,
		promptBuilder,
		appCtx.aiClient,
		appCtx.Reader,
	), nil
}

// BuildImageRunner は個別パネル画像生成を担当する MangaImageRunner を構築します。
func BuildImageRunner(ctx context.Context, appCtx *AppContext) (runner.ImageRunner, error) {
	// limit は Config の DefaultPanelLimit 定数を使用
	return runner.NewMangaImageRunner(
		appCtx.MangaGenerator,
		config.DefaultImagePromptSuffix,
	), nil
}

// BuildMangaPageRunner は複数ページへの自動分割に対応した Runner を構築するのだ。
func BuildMangaPageRunner(ctx context.Context, appCtx *AppContext) (runner.PageRunner, error) {
	return runner.NewMangaPageRunner(
		appCtx.MangaGenerator,
		config.DefaultImagePromptSuffix,
	), nil
}

// BuildPublisherRunner はコンテンツ保存と変換を行う Runner を構築します。
func BuildPublisherRunner(ctx context.Context, appCtx *AppContext) (runner.PublisherRunner, error) {
	fmtConfig := builder.BuilderConfig{
		EnableHardWraps: true,
		Mode:            "webtoon",
	}
	md2htmlBuilder, err := builder.NewBuilder(fmtConfig)
	if err != nil {
		return nil, fmt.Errorf("MarkdownToHtmlビルダーの初期化に失敗しました: %w", err)
	}

	md2htmlRunner, err := md2htmlBuilder.BuildRunner()
	if err != nil {
		return nil, fmt.Errorf("MarkdownToHtmlRunnerの初期化に失敗しました: %w", err)
	}

	pub := publisher.NewMangaPublisher(appCtx.Writer, md2htmlRunner)

	// Web版のフラットな Config をそのまま渡す
	return runner.NewDefaultPublisherRunner(*appCtx.Config, pub), nil
}

// BuildDesignRunner はキャラクターデザインの生成を担う Runner を構築します。
func BuildDesignRunner(ctx context.Context, appCtx *AppContext) (runner.DesignRunner, error) {
	return runner.NewMangaDesignRunner(
		appCtx.MangaGenerator,
		appCtx.Writer,
	), nil
}

// initializeAIClient は gemini クライアントを初期化します。
func initializeAIClient(ctx context.Context, apiKey string) (gemini.GenerativeModel, error) {
	const defaultGeminiTemperature = float32(0.2)
	clientConfig := gemini.Config{
		APIKey:      apiKey,
		Temperature: genai.Ptr(defaultGeminiTemperature),
	}
	aiClient, err := gemini.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("AIクライアントの初期化に失敗しました: %w", err)
	}
	return aiClient, nil
}

// initializeMangaGenerator は MangaGeneratorを初期化します。
func initializeMangaGenerator(httpClient httpkit.ClientInterface, aiClient gemini.GenerativeModel, model, characterConfig string) (generator.MangaGenerator, error) {
	mangaGen, err := generator.NewMangaGenerator(httpClient, aiClient, model, characterConfig)
	if err != nil {
		return generator.MangaGenerator{}, fmt.Errorf("MangaGeneratorの初期化に失敗しました: %w", err)
	}

	return mangaGen, nil
}
