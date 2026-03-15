package adapters

import (
	"context"
	"fmt"

	mangaKitDom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-remote-io/pkg/remoteio"

	"ap-manga-web/internal/prompts"
)

// PromptDependencies はプロンプト関連の依存関係をまとめた構造体です。
type PromptDependencies struct {
	CharactersMap mangaKitDom.CharactersMap
	ScriptPrompt  mangaKitDom.ScriptPrompt
	ImagePrompt   mangaKitDom.ImagePrompt
}

// NewPromptDependencies は Prompt ビルダーを初期化します。
func NewPromptDependencies(ctx context.Context, reader remoteio.InputReader, characterConfigPath, styleSuffix string) (*PromptDependencies, error) {
	charMap, err := mangaKitDom.LoadCharacterMap(ctx, reader, characterConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate character map: %w", err)
	}

	textPrompt, err := prompts.NewTextPromptBuilder()
	if err != nil {
		return nil, fmt.Errorf("failed to create text prompt builder: %w", err)
	}

	imagePrompt := prompts.NewImagePromptBuilder(charMap, styleSuffix)

	return &PromptDependencies{
		CharactersMap: charMap,
		ScriptPrompt:  textPrompt,
		ImagePrompt:   imagePrompt,
	}, nil
}
