package builder

import (
	"context"
	"fmt"

	"github.com/shouni/go-manga-kit/pkg/workflow"
)

// BuildScriptRunner は台本テキスト生成の Runner を構築します。
func BuildScriptRunner(ctx context.Context, appCtx *AppContext) (workflow.ScriptRunner, error) {
	runner, err := appCtx.Workflow.BuildScriptRunner()
	if err != nil {
		return nil, fmt.Errorf("ScriptRunner の構築に失敗しました: %w", err)
	}
	return runner, nil
}

// BuildPanelImageRunner は個別パネル画像生成を担当する Runner を構築します。
func BuildPanelImageRunner(ctx context.Context, appCtx *AppContext) (workflow.PanelImageRunner, error) {
	runner, err := appCtx.Workflow.BuildPanelImageRunner()
	if err != nil {
		return nil, fmt.Errorf("PanelImageRunner の構築に失敗しました: %w", err)
	}
	return runner, nil
}

// BuildPageImageRunner は複数ページへの自動分割・統合画像生成に対応した Runner を構築するのだ。
func BuildPageImageRunner(ctx context.Context, appCtx *AppContext) (workflow.PageImageRunner, error) {
	runner, err := appCtx.Workflow.BuildPageImageRunner()
	if err != nil {
		return nil, fmt.Errorf("PageImageRunner の構築に失敗しました: %w", err)
	}
	return runner, nil
}

// BuildPublishRunner はコンテンツ保存と変換を行う Runner を構築します。
func BuildPublishRunner(ctx context.Context, appCtx *AppContext) (workflow.PublishRunner, error) {
	runner, err := appCtx.Workflow.BuildPublishRunner()
	if err != nil {
		return nil, fmt.Errorf("PublishRunner の構築に失敗しました: %w", err)
	}
	return runner, nil
}

// BuildDesignRunner はキャラクターデザインの生成を担う Runner を構築します。
func BuildDesignRunner(ctx context.Context, appCtx *AppContext) (workflow.DesignRunner, error) {
	runner, err := appCtx.Workflow.BuildDesignRunner()
	if err != nil {
		return nil, fmt.Errorf("DesignRunner の構築に失敗しました: %w", err)
	}
	return runner, nil
}
