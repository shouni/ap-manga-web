package prompts

import (
	"fmt"

	"github.com/shouni/go-manga-kit/pkg/domain"
	promptkit "github.com/shouni/go-prompt-kit/prompts"
)

// Builder は go-prompt-kit を利用してプロンプトの構築を行います。
type Builder struct {
	promptBuilder *promptkit.Builder
}

// NewBuilder は Builder のインスタンスを構築します。
func NewBuilder(templates map[string]string) (*Builder, error) {
	pb, err := promptkit.NewBuilder(templates)
	if err != nil {
		return nil, fmt.Errorf("プロンプトビルダーの初期化に失敗しました: %w", err)
	}

	return &Builder{
		promptBuilder: pb,
	}, nil
}

// Build は、要求されたモードに応じて適切なテンプレートを実行します。
// 注意: data の内容に関する事前バリデーションは行いません。呼び出し元で適切なデータが設定されていることを保証してください。
func (b *Builder) Build(mode string, data *domain.TemplateData) (string, error) {
	if data == nil {
		return "", fmt.Errorf("データがnilです: テンプレートの実行にはデータが必要です")
	}
	return b.promptBuilder.Build(mode, data)
}
