package prompts

import (
	_ "embed"
)

const (
	ModeDuet     = "duet"
	ModeDialogue = "dialogue"
)

var (
	//go:embed duet.md
	DuetPrompt string
	//go:embed dialogue.md
	DialoguePrompt string
)

// allTemplates はモードとテンプレート文字列を紐づけるマップです。
var allTemplates = map[string]string{
	ModeDuet:     DuetPrompt,
	ModeDialogue: DialoguePrompt,
}
