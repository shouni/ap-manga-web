package assets

import (
	"embed"

	"github.com/shouni/go-prompt-kit/resource"
)

const (
	promptDir    = "prompts"
	promptPrefix = "prompt_"
)

var (
	// promptFiles はプロンプトテンプレートです。
	//go:embed prompts/prompt_*.md
	promptFiles embed.FS

	// Characters は、漫画に登場するキャラクターの基本定義（名前、外見、Seed値など）を記述した JSON データです。
	//go:embed characters/characters.json
	Characters []byte

	// Templates は、すべてのHTMLテンプレートを保持します。
	//go:embed templates/*.html
	Templates embed.FS
)

// LoadPrompts は埋め込まれたプロンプトファイルを読み込みます。
func LoadPrompts() (map[string]string, error) {
	return resource.Load(promptFiles, promptDir, promptPrefix)
}
