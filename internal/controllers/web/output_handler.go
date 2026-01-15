package web

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shouni/go-manga-kit/pkg/asset"
)

// pageFileRegex は初期化時に一度だけコンパイル
var pageFileRegex = func() *regexp.Regexp {
	baseName := strings.TrimSuffix(asset.DefaultPageFileName, filepath.Ext(asset.DefaultPageFileName))
	pattern := fmt.Sprintf(`^%s_\d+\.png$`, regexp.QuoteMeta(baseName))
	return regexp.MustCompile(pattern)
}()

type mangaViewData struct {
	Title       string
	ImageURLs   []string
	MarkdownRaw string
}

// ServeOutput は漫画ビューアーのメインハンドラ
func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	title := chi.URLParam(r, "title")

	// 1. プロット（Markdown）の取得
	markdownContent := h.loadPlotContent(r, title)

	// 2. 画像リストの取得と署名付きURLの生成
	signedURLs, err := h.loadSignedImageURLs(r, title)
	if err != nil {
		// loadSignedImageURLs 内でログ出力済みを想定
		http.Error(w, "Failed to retrieve contents", http.StatusInternalServerError)
		return
	}

	// 3. レンダリング
	h.render(w, http.StatusOK, "manga_view.html", title, mangaViewData{
		Title:       title,
		ImageURLs:   signedURLs,
		MarkdownRaw: markdownContent,
	})
}

// --- ヘルパーメソッド ---

// loadPlotContent はプロットファイルを読み込むのだ
func (h *Handler) loadPlotContent(r *http.Request, title string) string {
	relPath, err := h.validateAndCleanPath(title, asset.DefaultMangaPlotName)
	if err != nil {
		return ""
	}

	plotPath := h.cfg.GetGCSObjectURL(relPath)
	rc, err := h.reader.Open(r.Context(), plotPath)
	if err != nil {
		slog.WarnContext(r.Context(), "Plot file not found", "path", plotPath)
		return ""
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		slog.ErrorContext(r.Context(), "Read error", "path", plotPath, "error", err)
		return ""
	}
	return string(data)
}

// loadSignedImageURLs はGCSから画像をリストして署名付きURLのリストを返すのだ
func (h *Handler) loadSignedImageURLs(r *http.Request, title string) ([]string, error) {
	ctx := r.Context()
	prefix, err := h.validateAndCleanPath(title, asset.DefaultImageDir)
	if err != nil {
		return nil, err
	}

	gcsPrefix := h.cfg.GetGCSObjectURL(prefix)
	var filePaths []string

	err = h.reader.List(ctx, gcsPrefix, func(gcsPath string) error {
		if pageFileRegex.MatchString(filepath.Base(gcsPath)) {
			filePaths = append(filePaths, gcsPath)
		}
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "List error", "prefix", gcsPrefix, "error", err)
		return nil, err
	}

	sort.Strings(filePaths)

	var urls []string
	for _, p := range filePaths {
		u, err := h.signer.GenerateSignedURL(ctx, p, http.MethodGet, time.Hour)
		if err != nil {
			slog.ErrorContext(ctx, "Sign error", "path", p, "error", err)
			continue
		}
		urls = append(urls, u)
	}
	return urls, nil
}
