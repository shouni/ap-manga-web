package prompts

import (
	"fmt"

	"github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-prompt-kit/prompts"

	"ap-manga-web/assets"
)

// Builder はプロンプトの構成を管理し、モード選択のロジックを内包します。
type Builder struct {
	promptbuilder *prompts.Builder
}

// NewPromptAdapter は Builder のインスタンスを構築します。
func NewPromptAdapter() (*Builder, error) {
	templates, err := assets.LoadPrompts()
	if err != nil {
		return nil, fmt.Errorf("プロンプトテンプレートの読み込みに失敗しました: %w", err)
	}
	return newBuilder(templates)
}

// newBuilder は Builder を初期化します。
func newBuilder(templates map[string]string) (*Builder, error) {
	promptbuilder, err := prompts.NewBuilder(templates)
	if err != nil {
		return nil, fmt.Errorf("プロンプトビルダーの初期化に失敗しました: %w", err)
	}

	return &Builder{
		promptbuilder: promptbuilder,
	}, nil
}

// Build は、要求されたモードに応じて適切なテンプレートを実行します。
// 注意: data の内容に関する事前バリデーションは行いません。呼び出し元で適切なデータが設定されていることを保証してください。
func (b *Builder) Build(mode string, data *domain.TemplateData) (string, error) {
	return b.promptbuilder.Build(mode, data)
}
