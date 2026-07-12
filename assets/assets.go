// Package assets は、プロンプトテンプレートとキャラクター設定を埋め込みリソースとして提供します。
package assets

import (
	"embed"

	characterassets "github.com/shouni/go-character-kit/assets"
	"github.com/shouni/go-character-kit/character"
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

	// Templates は、すべてのHTMLテンプレートを保持します。
	//go:embed templates/*.html
	Templates embed.FS
)

// LoadPrompts は埋め込まれたプロンプトファイルを読み込みます。
func LoadPrompts() (map[string]string, error) {
	return resource.Load(promptFiles, promptDir, promptPrefix)
}

// LoadCharacters は go-character-kit の共有キャラクター定義を読み込みます。
// ap-mv・ap-comp と同じ正規データソースを使うことで、キャラクターの外見・参照画像URLの
// 定義がアプリ間で食い違わないようにしています（このパッケージが独自に characters.json
// を埋め込んで管理していた過去の実装は、定義が徐々に乖離する原因になっていました）。
func LoadCharacters() (*character.Characters, error) {
	return characterassets.LoadCharacters()
}
