package runner

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"ap-manga-web/internal/config"

	imgdom "github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/generator"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

// DesignRunner はデザインシート生成のインターフェースです。
type DesignRunner interface {
	Run(ctx context.Context, cfg *config.Config, charIDs []string, seed int64, outputGCS string) (string, int64, error)
}

// MangaDesignRunner はキャラクターデザインシート生成の実行実体なのだ。
type MangaDesignRunner struct {
	mangaGen generator.MangaGenerator
	writer   remoteio.OutputWriter
}

// NewMangaDesignRunner は依存関係を注入して初期化するのだ。
func NewMangaDesignRunner(mangaGen generator.MangaGenerator, writer remoteio.OutputWriter) *MangaDesignRunner {
	return &MangaDesignRunner{
		mangaGen: mangaGen,
		writer:   writer,
	}
}

// Run は、キャラクターIDを指定してデザインシートを生成し、GCSやローカルに保存するのだ。
func (dr *MangaDesignRunner) Run(ctx context.Context, cfg *config.Config, charIDs []string, seed int64, outputGCS string) (string, int64, error) {
	// 1. 複数キャラの情報を集約 (CharactersMap などから取得)
	refs, descriptions, err := collectCharacterAssets(dr.mangaGen.Characters, charIDs)
	if err != nil {
		return "", 0, fmt.Errorf("キャラクター資産の収集に失敗しました: %w", err)
	}

	slog.Info("Executing design work generation",
		slog.Any("chars", charIDs),
		slog.Int("ref_count", len(refs)),
	)

	// 2. プロンプト構築
	designPrompt := buildDesignPrompt(descriptions)

	// 3. 生成リクエスト (Seedが0の場合はランダム生成)
	pageReq := imgdom.ImagePageRequest{
		Prompt:        designPrompt,
		ReferenceURLs: refs,
		AspectRatio:   "16:9",
		Seed:          ptrInt64(seed),
	}

	// 4. 生成実行 (Gemini Nano Banana 呼び出し)
	resp, err := dr.mangaGen.ImgGen.GenerateMangaPage(ctx, pageReq)
	if err != nil {
		slog.Error("Design generation failed", "error", err)
		return "", 0, fmt.Errorf("画像の生成に失敗しました: %w", err)
	}

	// 5. 画像の保存 (GCS: OutputGCS を優先的に使用)
	outputPath, err := dr.saveResponseImage(ctx, *resp, charIDs, outputGCS)
	if err != nil {
		slog.Error("Failed to save image", "error", err)
		return "", 0, fmt.Errorf("画像の保存に失敗しました: %w", err)
	}

	return outputPath, resp.UsedSeed, nil
}

// --- 以下、ヘルパー関数群 (CLI版のロジックを継承) ---

func (dr *MangaDesignRunner) saveResponseImage(ctx context.Context, resp imgdom.ImageResponse, charIDs []string, imageDir string) (string, error) {
	extension := getPreferredExtension(resp.MimeType)
	charTags := strings.Join(charIDs, "_")
	isCloud := remoteio.IsGCSURI(imageDir) || remoteio.IsS3URI(imageDir)

	filename := fmt.Sprintf("design_%s%s", charTags, extension)
	var finalPath string
	var err error

	if isCloud {
		finalPath, err = url.JoinPath(imageDir, filename)
	} else {
		finalPath = filepath.Join(imageDir, filename)
		if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
			return "", err
		}
	}

	if err != nil {
		return "", err
	}

	if err := dr.writer.Write(ctx, finalPath, bytes.NewReader(resp.Data), resp.MimeType); err != nil {
		return "", err
	}

	return finalPath, nil
}

func buildDesignPrompt(descriptions []string) string {
	base := fmt.Sprintf("Masterpiece character design sheet of %s, side-by-side, multiple views (front, side, back), standing full body",
		strings.Join(descriptions, " and "))
	// 保存されたデフォルトの画風サフィックスを結合
	return fmt.Sprintf("%s, %s, white background, sharp focus, 4k resolution", base, config.DefaultImagePromptSuffix)
}

func collectCharacterAssets(chars map[string]domain.Character, ids []string) ([]string, []string, error) {
	var refs []string
	var descs []string

	for _, id := range ids {
		char, ok := chars[id]
		if !ok {
			continue
		}
		if char.ReferenceURL != "" {
			refs = append(refs, char.ReferenceURL)
		}
		desc := char.Name
		if len(char.VisualCues) > 0 {
			desc = fmt.Sprintf("%s (%s)", char.Name, strings.Join(char.VisualCues, ", "))
		}
		descs = append(descs, desc)
	}

	if len(refs) == 0 {
		return nil, nil, fmt.Errorf("参照URLを持つキャラクターが見つかりません")
	}
	return refs, descs, nil
}

func ptrInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func getPreferredExtension(mimeType string) string {
	preferred := map[string]string{"image/png": ".png", "image/jpeg": ".jpg"}
	if ext, ok := preferred[mimeType]; ok {
		return ext
	}
	return ".png"
}
