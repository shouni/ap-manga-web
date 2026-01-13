package builder

import (
	"context"
	"fmt"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"github.com/shouni/go-manga-kit/pkg/workflow"
	"google.golang.org/genai"
)

// BuildScriptRunner は台本テキスト生成の Runner を構築します。
func BuildScriptRunner(ctx context.Context, appCtx *AppContext) (workflow.ScriptRunner, error) {
	runner, err := appCtx.WorkflowBuilder.BuildScriptRunner()
	if err != nil {
		return nil, fmt.Errorf("ScriptRunner の構築に失敗しました: %w", err)
	}
	return runner, nil
}

// BuildPanelImageRunner は個別パネル画像生成を担当する Runner を構築します。
func BuildPanelImageRunner(ctx context.Context, appCtx *AppContext) (workflow.PanelImageRunner, error) {
	runner := appCtx.WorkflowBuilder.BuildPanelImageRunner()
	return runner, nil
}

// BuildPageImageRunner は複数ページへの自動分割・統合画像生成に対応した Runner を構築するのだ。
func BuildPageImageRunner(ctx context.Context, appCtx *AppContext) (workflow.PageImageRunner, error) {
	runner := appCtx.WorkflowBuilder.BuildPageImageRunner()
	return runner, nil
}

// BuildPublishRunner はコンテンツ保存と変換を行う Runner を構築します。
func BuildPublishRunner(ctx context.Context, appCtx *AppContext) (workflow.PublishRunner, error) {
	runner, err := appCtx.WorkflowBuilder.BuildPublishRunner()
	if err != nil {
		return nil, fmt.Errorf("PublishRunner の構築に失敗しました: %w", err)
	}
	return runner, nil
}

// BuildDesignRunner はキャラクターデザインの生成を担う Runner を構築します。
func BuildDesignRunner(ctx context.Context, appCtx *AppContext) (workflow.DesignRunner, error) {
	runner := appCtx.WorkflowBuilder.BuildDesignRunner()
	return runner, nil
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
