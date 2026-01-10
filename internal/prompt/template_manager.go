package prompt

import (
	_ "embed"
	"fmt"
	"maps"
	"slices"
	"strings"
)

const (
	ModeDuet     = "duet"
	ModeDialogue = "dialogue"
)

//go:embed duet.md
var DuetPrompt string

//go:embed dialogue.md
var DialoguePrompt string

// modeTemplates はモードとテンプレート文字列を紐づけるマップなのだ。
var modeTemplates = map[string]string{
	ModeDuet:     DuetPrompt,
	ModeDialogue: DialoguePrompt,
}

// GetPromptByMode は、指定されたモードに対応するプロンプト文字列を返すのだ。
func GetPromptByMode(mode string) (string, error) {
	content, ok := modeTemplates[mode]
	if !ok {
		supported := slices.Collect(maps.Keys(modeTemplates))
		slices.Sort(supported)

		return "", fmt.Errorf("サポートされていないモード: '%s'。サポートされているモードは [%s] です",
			mode, strings.Join(supported, ", "))
	}

	if content == "" {
		return "", fmt.Errorf("モード '%s' に対応するプロンプトテンプレートが空なのだ。embed設定を確認してほしいのだ", mode)
	}

	return content, nil
}
