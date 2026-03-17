package prompts

import (
	"fmt"
	"strings"
	"text/template"

	"ap-manga-web/assets"

	"github.com/shouni/go-manga-kit/pkg/domain"
)

const (
	ModeDuet     = "duet"
	ModeDialogue = "dialogue"
)

// Builder はレビュープロンプトの構成を管理し、モード選択のロジックを内包します。
type Builder struct {
	templates map[string]*template.Template
}

// NewPromptAdapter は Builder のインスタンスを構築します。
func NewPromptAdapter() (*Builder, error) {
	templates := map[string]string{
		ModeDuet:     assets.DuetPrompt,
		ModeDialogue: assets.DialoguePrompt,
	}

	return NewBuilder(templates)
}

// NewBuilder は Builder を初期化します。
func NewBuilder(templates map[string]string) (*Builder, error) {
	if len(templates) == 0 {
		return nil, fmt.Errorf("テンプレートマップが空またはnilです")
	}
	parsedTemplates := make(map[string]*template.Template)
	for mode, content := range templates {
		if content == "" {
			return nil, fmt.Errorf("プロンプトテンプレート '%s' の読み込みに失敗しました: 内容が空です", mode)
		}

		tmpl, err := template.New(mode).Parse(content)
		if err != nil {
			return nil, fmt.Errorf("プロンプト '%s' の解析に失敗: %w", mode, err)
		}
		parsedTemplates[mode] = tmpl
	}

	return &Builder{
		templates: parsedTemplates,
	}, nil
}

// Build は、要求されたモードに応じて適切なテンプレートを実行します。
// 注意: data の内容に関する事前バリデーションは行いません。呼び出し元で適切なデータが設定されていることを保証してください。
func (b *Builder) Build(mode string, data *domain.TemplateData) (string, error) {
	if data == nil {
		return "", fmt.Errorf("データがnilです: テンプレートの実行にはデータが必要です")
	}

	tmpl, err := b.getTemplate(mode)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("プロンプトテンプレートの実行に失敗しました: %w", err)
	}

	return sb.String(), nil
}

// getTemplate は指定されたモードに対応するテンプレートを取得します。
func (b *Builder) getTemplate(mode string) (*template.Template, error) {
	tmpl, ok := b.templates[mode]
	if !ok {
		return nil, fmt.Errorf("不明なモードです: '%s'", mode)
	}
	return tmpl, nil
}
