package assets

import _ "embed"

var (
	//go:embed prompts/duet.md
	DuetPrompt string
	//go:embed prompts/dialogue.md
	DialoguePrompt string
)
