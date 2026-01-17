package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"

	"ap-manga-web/internal/config"

	"github.com/go-chi/chi/v5"
	"github.com/shouni/go-manga-kit/pkg/asset"
	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
)

// mangaViewData はテンプレート「manga_view.html」に渡すためのデータ構造体
type mangaViewData struct {
	Title         string
	OriginalTitle string // ユーザー向けの本来のタイトル
	PlotContent   string
	PageURLs      []string
	PanelURLs     []string
}

// ServeOutput は指定されたタイトルの漫画成果物（プロットおよび画像）を取得し、ビューアー画面を表示する。
func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := chi.URLParam(r, "title")
	// JSONプロットから本来のタイトルを読み取るヘルパーを呼び出す
	originalTitle := h.loadOriginalTitle(r, title)

	handleImageLoadError := func(err error, imageType string) bool {
		if err == nil {
			return false
		}
		if errors.Is(err, ErrInvalidPath) {
			slog.WarnContext(ctx, "Invalid path request for images", "title", title, "type", imageType, "error", err)
			http.Error(w, "Invalid Title Path", http.StatusBadRequest)
		} else {
			slog.ErrorContext(ctx, "Failed to prepare image URLs", "title", title, "type", imageType, "error", err)
			http.Error(w, "Failed to retrieve manga images", http.StatusInternalServerError)
		}
		return true
	}

	// 1. プロットの取得
	plotContent, err := h.loadPlotContent(r, title)
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

	// 2. 署名付き画像URLリストの取得（ページとパネル、それぞれを独立して取得する）
	signedPageURLs, err := h.loadSignedImageURLs(r, title, asset.PageFileRegex)
	if handleImageLoadError(err, "page") {
		return
	}

	signedPanelURLs, err := h.loadSignedImageURLs(r, title, asset.PanelFileRegex)
	if handleImageLoadError(err, "panel") {
		return
	}

	// 署名付きURLの有効期限に同期させる！
	cacheAgeSec := int64(config.SignedURLExpiration.Seconds())
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", cacheAgeSec))

	// 3. テンプレートのレンダリング
	h.render(w, http.StatusOK, "manga_view.html", title, mangaViewData{
		Title:         title,
		OriginalTitle: originalTitle,
		PlotContent:   plotContent,
		PageURLs:      signedPageURLs,
		PanelURLs:     signedPanelURLs,
	})
}

// loadPlotContent は指定されたタイトルのプロットファイルを読み込み、その内容を文字列として返す。
func (h *Handler) loadPlotContent(r *http.Request, title string) (string, error) {
	relPath, err := h.validateAndCleanPath(title, asset.DefaultMangaPlotName)
	if err != nil {
		return "", err // ErrInvalidPath がラップされて返却されるのだ
	}

	plotPath := h.cfg.GetGCSObjectURL(relPath)
	rc, err := h.reader.Open(r.Context(), plotPath)
	if err != nil {
		// ファイルが見つからない場合は空文字として扱い、ビューアー側で「データなし」を表示させる
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

// loadSignedImageURLs は指定されたタイトルの画像をリストし、一時的な署名付きURLのスライスを生成する
func (h *Handler) loadSignedImageURLs(r *http.Request, title string, regex *regexp.Regexp) ([]string, error) {
	ctx := r.Context()
	prefix, err := h.validateAndCleanPath(title, asset.DefaultImageDir)
	if err != nil {
		return nil, err
	}

	gcsPrefix := h.cfg.GetGCSObjectURL(prefix)
	var filePaths []string

	// 指定された正規表現（Page用かPanel用か）にマッチするファイルのみを抽出
	err = h.reader.List(ctx, gcsPrefix, func(gcsPath string) error {
		if regex.MatchString(filepath.Base(gcsPath)) {
			filePaths = append(filePaths, gcsPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list images from store: %w", err)
	}

	// ページ順・パネル順を正しく表示するため、ファイル名でソートする
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

	if len(filePaths) > 0 && len(signedURLs) == 0 {
		return nil, errors.New("could not generate any signed URLs for the available image assets")
	}

	return signedURLs, nil
}

// loadOriginalTitle は JSON プロットを読み込み、本来のタイトルを抽出します。
// 失敗した場合は、フォールバックとして safeTitle を返します。
func (h *Handler) loadOriginalTitle(r *http.Request, safeTitle string) string {
	relPath, err := h.validateAndCleanPath(safeTitle, asset.DefaultMangaPlotJson)
	if err != nil {
		return safeTitle
	}

	plotPath := h.cfg.GetGCSObjectURL(relPath)
	rc, err := h.reader.Open(r.Context(), plotPath)
	if err != nil {
		return safeTitle
	}
	defer rc.Close()

	var manga mangadom.MangaResponse
	if err := json.NewDecoder(rc).Decode(&manga); err != nil {
		return safeTitle
	}

	if manga.Title == "" {
		return safeTitle
	}
	return manga.Title
}
