package assets

import _ "embed"

var (
	// DuetPrompt は、二人のキャラクターによる掛け合い（デュエット）形式の
	// プロンプトテンプレートを保持します。
	//go:embed prompts/duet.md
	DuetPrompt string

	// DialoguePrompt は、キャラクター同士の対話シーンを生成するための
	// 基本的なプロンプトテンプレートを保持します。
	//go:embed prompts/dialogue.md
	DialoguePrompt string

	// Characters は、漫画に登場するキャラクターの基本定義（名前、外見、Seed値など）を
	// 記述した JSON データです。
	//go:embed characters/characters.json
	Characters []byte
)
