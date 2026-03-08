package prompts

import (
	"strings"

	"github.com/shouni/go-manga-kit/pkg/domain"
)

const (
	// CinematicTags クオリティ向上のための共通タグ
	CinematicTags = "cinematic composition, high resolution, sharp focus, 2k"

	// RenderingStyle は共通の画風を定義します。
	RenderingStyle = `### GLOBAL VISUAL STYLE ###
- RENDERING: Sharp clean lineart, vibrant colors, no blurring, high contrast, cinematic manga lighting.`
)

// ImagePromptBuilder は、キャラクター情報を考慮してAIプロンプトを構築します。
type ImagePromptBuilder struct {
	characterMap  domain.CharactersMap
	defaultSuffix string // 例: "anime style, high quality"
}

// NewImagePromptBuilder は新しい PromptBuilder を生成します。
func NewImagePromptBuilder(characterMap domain.CharactersMap, suffix string) *ImagePromptBuilder {
	return &ImagePromptBuilder{
		characterMap:  characterMap,
		defaultSuffix: suffix,
	}
}

// sanitizeInline は文字列をプロンプトに埋め込む前の最低限の正規化を行います。
func sanitizeInline(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.TrimSpace(s)
}

// formatDialogue はダイアログの正規化のみを行います。
func formatDialogue(s string) string {
	s = sanitizeInline(s)
	// プロンプト内で "TEXT_TO_RENDER: "..." のようにテキストをダブルクォートで囲むため、
	// 内部にダブルクォートが含まれていると構文エラーを引き起こし、AIの解釈を妨げる可能性があります。
	// これを防ぐため、内部のダブルクォートはシングルクォートに置換します。
	s = strings.ReplaceAll(s, "\"", "'")
	return s
}
