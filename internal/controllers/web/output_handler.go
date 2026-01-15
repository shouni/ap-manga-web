package web

import (
	"ap-manga-web/internal/config"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/shouni/go-manga-kit/pkg/asset"
)

type mangaViewData struct {
	Title       string
	ImageURLs   []string
	MarkdownRaw string
}

// ServeOutput は指定されたタイトルの漫画成果物（プロットおよび画像）を取得し、ビューアー画面を表示します。
func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := chi.URLParam(r, "title")

	// 1. プロット（Markdown）の取得
	// パス検証エラー (ErrInvalidPath) は 400、それ以外の読み込みエラーは 500 を返却します。
	markdownContent, err := h.loadPlotContent(r, title)
	if err != nil {
		if errors.Is(err, ErrInvalidPath) {
			slog.WarnContext(ctx, "Invalid path request for plot", "title", title, "error", err)
			http.Error(w, "Invalid Title Path", http.StatusBadRequest)
		} else {
			slog.ErrorContext(ctx, "Failed to load plot content", "title", title, "error", err)
			http.Error(w, "Failed to load plot content", http.StatusInternalServerError)
		}
		return
	}

	// 2. 署名付き画像URLリストの取得
	signedURLs, err := h.loadSignedImageURLs(r, title)
	if err != nil {
		if errors.Is(err, ErrInvalidPath) {
			slog.WarnContext(ctx, "Invalid path request for images", "title", title, "error", err)
			http.Error(w, "Invalid Title Path", http.StatusBadRequest)
		} else {
			slog.ErrorContext(ctx, "Failed to prepare image URLs", "title", title, "error", err)
			http.Error(w, "Failed to retrieve manga images", http.StatusInternalServerError)
		}
		return
	}

	cacheAgeSec := int64(config.SignedURLExpiration.Seconds())
	if cacheAgeSec < 0 {
		cacheAgeSec = 0
	}
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", cacheAgeSec))

	// 3. テンプレートのレンダリング
	h.render(w, http.StatusOK, "manga_view.html", title, mangaViewData{
		Title:       title,
		ImageURLs:   signedURLs,
		MarkdownRaw: markdownContent,
	})
}

// loadPlotContent は指定されたタイトルのプロットファイルを読み込み、その内容を文字列として返します。
// ファイルが存在しない場合は空文字列を返しますが、検証エラーや致命的な読み込みエラーは error として返します。
func (h *Handler) loadPlotContent(r *http.Request, title string) (string, error) {
	relPath, err := h.validateAndCleanPath(title, asset.DefaultMangaPlotName)
	if err != nil {
		return "", err // ErrInvalidPath がラップされて返却される
	}

	plotPath := h.cfg.GetGCSObjectURL(relPath)
	rc, err := h.reader.Open(r.Context(), plotPath)
	if err != nil {
		// ファイルが見つからない場合は仕様として許容し、正常系（空文字）として扱います。
		slog.WarnContext(r.Context(), "Plot file not found, skipping rendering", "path", plotPath)
		return "", nil
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("failed to read plot file: %w", err)
	}
	return string(data), nil
}

// loadSignedImageURLs は指定されたタイトルの画像をリストし、一時的な閲覧権限を持つ署名付きURLのスライスを生成します。
func (h *Handler) loadSignedImageURLs(r *http.Request, title string) ([]string, error) {
	ctx := r.Context()
	prefix, err := h.validateAndCleanPath(title, asset.DefaultImageDir)
	if err != nil {
		return nil, err
	}

	gcsPrefix := h.cfg.GetGCSObjectURL(prefix)
	var filePaths []string

	err = h.reader.List(ctx, gcsPrefix, func(gcsPath string) error {
		if asset.PageFileRegex.MatchString(filepath.Base(gcsPath)) {
			filePaths = append(filePaths, gcsPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list images from store: %w", err)
	}

	sort.Strings(filePaths)

	var signedURLs []string
	for _, gcsPath := range filePaths {
		u, err := h.signer.GenerateSignedURL(ctx, gcsPath, http.MethodGet, config.SignedURLExpiration)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to generate signed URL", "path", gcsPath, "error", err)
			continue
		}
		signedURLs = append(signedURLs, u)
	}

	// 画像が存在するにもかかわらず、署名付きURLが1つも生成できなかった場合は
	// 権限設定ミスやサービス中断の可能性があるため、エラーとして扱います。
	if len(filePaths) > 0 && len(signedURLs) == 0 {
		return nil, errors.New("could not generate any signed URLs for the available image assets")
	}

	return signedURLs, nil
}
