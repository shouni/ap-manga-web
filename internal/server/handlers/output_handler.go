package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"

	"ap-manga-web/internal/config"

	"github.com/go-chi/chi/v5"
	"github.com/shouni/go-manga-kit/pkg/asset"
	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
)

// mangaViewData はテンプレート「manga_view.html」に渡すためのデータ構造体
type mangaViewData struct {
	Title         string
	OriginalTitle string
	Manga         mangadom.MangaResponse // JSONからデコードしURL置換済みのデータ
	PlotContent   string                 // URL置換済みのMarkdown
	PageURLs      []string               // ページ全体画像の署名付きURL
}

// ServeOutput は指定されたタイトルの漫画成果物を取得し、ビューアー画面を表示する。
func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := chi.URLParam(r, "title")

	handleImageLoadError := func(err error, imageType string) bool {
		if err == nil {
			return false
		}
		if errors.Is(err, ErrInvalidPath) {
			slog.WarnContext(ctx, "画像のリクエストパスが不正です", "title", title, "type", imageType, "error", err)
			http.Error(w, "不正なタイトルパスです", http.StatusBadRequest)
		} else {
			slog.ErrorContext(ctx, "画像URLの生成に失敗しました", "title", title, "type", imageType, "error", err)
			http.Error(w, "漫画画像の取得に失敗しました", http.StatusInternalServerError)
		}
		return true
	}

	// 1. JSONプロットの取得（これが正となるデータ源なのだ）
	manga, err := h.loadMangaJSON(r, title)
	if err != nil {
		slog.ErrorContext(ctx, "プロットJSONの読み込みに失敗しました", "title", title, "error", err)
		http.Error(w, "データの読み込みに失敗しました", http.StatusInternalServerError)
		return
	}

	// 2. 署名付き画像URLリストの取得（Page用とPanel用）
	signedPageURLs, err := h.loadSignedImageURLs(r, title, asset.PageFileRegex)
	if handleImageLoadError(err, "page") {
		return
	}

	signedPanelURLs, err := h.loadSignedImageURLs(r, title, asset.PanelFileRegex)
	if handleImageLoadError(err, "panel") {
		return
	}

	// 3. マッピング処理：構造体内の相対パスを署名付きURLに置き換える
	panelMap := make(map[string]string)
	for _, u := range signedPanelURLs {
		cleanPath := strings.Split(u, "?")[0]
		fileName := path.Base(cleanPath)

		if existing, ok := panelMap[fileName]; ok {
			slog.WarnContext(ctx, "ファイル名の衝突を検知しました。画像が正しく表示されない可能性があります",
				"filename", fileName, "existing", existing, "new", u)
		}
		panelMap[fileName] = u
	}

	for i := range manga.Panels {
		p := &manga.Panels[i]
		// 元の ReferenceURL (gs://... や相対パス) からファイル名を特定
		fileName := path.Base(p.ReferenceURL)
		if signed, ok := panelMap[fileName]; ok {
			p.ReferenceURL = signed
		}
	}

	// 4. 置換後の構造体から Markdown を構築する
	plotMarkdown := h.workflow.PublishRunner.BuildMarkdown(&manga)
	// 署名付きURLの有効期限に同期させる
	cacheAgeSec := int64(config.SignedURLExpiration.Seconds())
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", cacheAgeSec))

	// 5. テンプレートのレンダリング
	h.render(w, http.StatusOK, "manga_view.html", title, mangaViewData{
		Title:         title,
		OriginalTitle: manga.Title,
		Manga:         manga,
		PlotContent:   plotMarkdown,
		PageURLs:      signedPageURLs,
	})
}

// loadMangaJSON は GCS から manga_plot.json を読み込み、ドメインモデルにデコードします。
func (h *Handler) loadMangaJSON(r *http.Request, title string) (mangadom.MangaResponse, error) {
	var manga mangadom.MangaResponse
	relPath, err := h.validateAndCleanPath(title, asset.DefaultMangaPlotJson)
	if err != nil {
		return manga, err
	}

	plotPath := h.cfg.GetGCSObjectURL(relPath)
	rc, err := h.rio.Reader.Open(r.Context(), plotPath)
	if err != nil {
		return manga, fmt.Errorf("JSONファイルが見つかりません: %w", err)
	}
	defer rc.Close()

	if err := json.NewDecoder(rc).Decode(&manga); err != nil {
		return manga, fmt.Errorf("JSONの解析に失敗しました: %w", err)
	}
	return manga, nil
}

// loadSignedImageURLs は指定されたタイトルの画像をリストし、一時的な署名付きURLを生成します。
func (h *Handler) loadSignedImageURLs(r *http.Request, title string, regex *regexp.Regexp) ([]string, error) {
	ctx := r.Context()
	prefix, err := h.validateAndCleanPath(title, asset.DefaultImageDir)
	if err != nil {
		return nil, err
	}

	gcsPrefix := h.cfg.GetGCSObjectURL(prefix)
	var filePaths []string

	// path.Base を使い、OSに依存せずスラッシュ区切りでファイル名を判定
	err = h.rio.Reader.List(ctx, gcsPrefix, func(gcsPath string) error {
		if regex.MatchString(path.Base(gcsPath)) {
			filePaths = append(filePaths, gcsPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ストレージのリスト取得に失敗: %w", err)
	}

	sort.Strings(filePaths)

	var signedURLs []string
	for _, gcsPath := range filePaths {
		u, err := h.rio.Signer.GenerateSignedURL(ctx, gcsPath, http.MethodGet, config.SignedURLExpiration)
		if err != nil {
			slog.ErrorContext(ctx, "署名付きURL生成失敗", "path", gcsPath, "error", err)
			continue
		}
		signedURLs = append(signedURLs, u)
	}

	return signedURLs, nil
}
