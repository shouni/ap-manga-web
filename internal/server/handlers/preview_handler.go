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
	"github.com/shouni/go-manga-kit/pkg/domain"
)

// mangaViewData はテンプレート「manga_view.html」に渡すためのデータ構造体
type mangaViewData struct {
	Title         string
	OriginalTitle string
	Manga         domain.MangaResponse // JSONからデコードしURL置換済みのデータ
	PageURLs      []string             // ページ全体画像の署名付きURL
}

// ServePreview は指定されたタイトルの漫画成果物を取得し、プレビュー画面を表示します。
func (h *Handler) ServePreview(w http.ResponseWriter, r *http.Request) {
	title := chi.URLParam(r, "title")

	// 1. JSONプロットの取得
	manga, err := h.loadMangaJSON(r, title)
	if err != nil {
		h.handleError(w, r, "プロットJSONの読み込みに失敗しました", title, err, http.StatusInternalServerError)
		return
	}

	// 2. 署名付き画像URLリストの取得（Page用）
	signedPageURLs, err := h.loadSignedImageURLs(r, title, asset.PageFileRegex)
	if err != nil {
		h.handleError(w, r, "ページ画像の取得に失敗しました", title, err, http.StatusInternalServerError)
		return
	}

	// 3. 署名付き画像URLリストの取得（Panel用）
	signedPanelURLs, err := h.loadSignedImageURLs(r, title, asset.PanelFileRegex)
	if err != nil {
		h.handleError(w, r, "パネル画像の取得に失敗しました", title, err, http.StatusInternalServerError)
		return
	}

	// 4. マッピング処理：パネル内の相対パスを署名付きURLに置換
	h.resolvePanelURLs(&manga, signedPanelURLs)

	// 5. キャッシュ制御（署名付きURLの有効期限に同期）
	cacheAgeSec := int64(config.SignedURLExpiration.Seconds())
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", cacheAgeSec))

	// 6. テンプレートのレンダリング
	h.render(w, http.StatusOK, "manga_view.html", title, mangaViewData{
		Title:         title,
		OriginalTitle: manga.Title,
		Manga:         manga,
		PageURLs:      signedPageURLs,
	})
}

// resolvePanelURLs は取得した署名付きURLを使って、MangaResponse内のReferenceURLを更新します。
func (h *Handler) resolvePanelURLs(manga *domain.MangaResponse, signedURLs []string) {
	panelMap := make(map[string]string)
	for _, u := range signedURLs {
		// クエリパラメータを除去してファイル名を取得
		cleanPath := strings.Split(u, "?")[0]
		fileName := path.Base(cleanPath)
		panelMap[fileName] = u
	}

	for i := range manga.Panels {
		p := &manga.Panels[i]
		fileName := path.Base(p.ReferenceURL)
		if signed, ok := panelMap[fileName]; ok {
			p.ReferenceURL = signed
		}
	}
}

// handleError は一貫したロギングとエラーレスポンスを提供します。
func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, msg, title string, err error, code int) {
	if errors.Is(err, ErrInvalidPath) {
		slog.WarnContext(r.Context(), "不正なパスリクエスト", "title", title, "error", err)
		http.Error(w, "リクエストされたパスが不正です", http.StatusBadRequest)
		return
	}
	slog.ErrorContext(r.Context(), msg, "title", title, "error", err)
	http.Error(w, msg, code)
}

// loadMangaJSON は GCS から manga_plot.json を読み込み、ドメインモデルにデコードします。
func (h *Handler) loadMangaJSON(r *http.Request, title string) (domain.MangaResponse, error) {
	var manga domain.MangaResponse
	relPath, err := h.validateAndCleanPath(title, asset.DefaultMangaPlotJson)
	if err != nil {
		return manga, err
	}

	plotPath := h.cfg.GetGCSObjectURL(relPath)
	rc, err := h.remoteIO.Reader.Open(r.Context(), plotPath)
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

	err = h.remoteIO.Reader.List(ctx, gcsPrefix, func(gcsPath string) error {
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
		u, err := h.remoteIO.Signer.GenerateSignedURL(ctx, gcsPath, http.MethodGet, config.SignedURLExpiration)
		if err != nil {
			slog.ErrorContext(ctx, "署名付きURL生成失敗", "path", gcsPath, "error", err)
			continue
		}
		signedURLs = append(signedURLs, u)
	}

	return signedURLs, nil
}
