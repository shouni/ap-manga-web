package adapters

import (
	"context"
	"fmt"

	"github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-remote-io/pkg/remoteio"

	"ap-manga-web/internal/prompts"
)

// NewPrompts は Prompt ビルダーを初期化します。
func NewPrompts(ctx context.Context, reader remoteio.InputReader, path string, styleSuffix string) (domain.CharactersMap, domain.ScriptPrompt, domain.ImagePrompt, error) {
	charMap, err := domain.LoadCharacterMap(ctx, reader, path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("CharacterMa の生成に失敗しました: %w", err)
	}

	textPrompt, err := prompts.NewTextPromptBuilder()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("TextPromptBuilder の新規作成に失敗しました: %w", err)
	}

	imagePrompt := prompts.NewImagePromptBuilder(charMap, styleSuffix)

	return charMap, textPrompt, imagePrompt, nil
}
