package adapters

import (
	"context"
	"fmt"

	"github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-remote-io/pkg/remoteio"

	"ap-manga-web/internal/prompts"
)

// PromptDependencies はプロンプト関連の依存関係をまとめた構造体です。
type PromptDependencies struct {
	CharactersMap domain.CharactersMap
	ScriptPrompt  domain.ScriptPrompt
	ImagePrompt   domain.ImagePrompt
}

// InitializePromptDependencies は Prompt ビルダーを初期化します。
func InitializePromptDependencies(ctx context.Context, reader remoteio.InputReader, path string, styleSuffix string) (*PromptDependencies, error) {
	charMap, err := domain.LoadCharacterMap(ctx, reader, path)
	if err != nil {
		return nil, fmt.Errorf("CharacterMap の生成に失敗しました: %w", err)
	}

	textPrompt, err := prompts.NewTextPromptBuilder()
	if err != nil {
		return nil, fmt.Errorf("TextPromptBuilder の新規作成に失敗しました: %w", err)
	}

	imagePrompt := prompts.NewImagePromptBuilder(charMap, styleSuffix)

	return &PromptDependencies{
		CharactersMap: charMap,
		ScriptPrompt:  textPrompt,
		ImagePrompt:   imagePrompt,
	}, nil
}
