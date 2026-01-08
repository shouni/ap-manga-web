package web

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

const defaultPanelLimit = 10

type IndexPageData struct {
	Title string
}

// AcceptedPageData は受付完了画面に渡すためのデータ構造なのだ
type AcceptedPageData struct {
	Title     string // ページタイトル
	ScriptURL string // ユーザーが入力した解析対象のURL
}

type Handler struct {
	cfg  config.Config
	tmpl *template.Template
}

// NewHandler は基本となる layout.html のみをパースして保持するのだ
func NewHandler(cfg config.Config) (*Handler, error) {
	layoutPath := filepath.Join(cfg.TemplateDir, "layout.html")
	// まだ実行されていない「クリーンな」テンプレートとして layout を保持するのだ
	tmpl, err := template.ParseFiles(layoutPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse layout template: %w", err)
	}

	return &Handler{
		cfg:  cfg,
		tmpl: tmpl,
	}, nil
}

// render は指定されたページファイルを layout と結合して描画する共通ヘルパーなのだ
func (h *Handler) render(w http.ResponseWriter, status int, pageFile string, data any) {
	// layout を保持しているテンプレートをクローンするのだ（未実行なので Clone 可能！）
	t, err := h.tmpl.Clone()
	if err != nil {
		slog.Error("Failed to clone template", "error", err)
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	// 各ページ固有のテンプレートファイルをパースして {{define "content"}} を注入するのだ
	pagePath := filepath.Join(h.cfg.TemplateDir, pageFile)
	if _, err := t.ParseFiles(pagePath); err != nil {
		slog.Error("Failed to parse page template", "path", pagePath, "error", err)
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	// layout.html 内の {{template "content" .}} に pageFile の内容が差し込まれるのだ
	if err := t.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		slog.Error("Failed to render template", "page", pageFile, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// Index はメイン画面を表示するのだ
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "index.html", IndexPageData{
		Title: "Generate - Manga Runner",
	})
}

// HandleSubmit は UI からの生成リクエストを処理するのだ
func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	// 1. フォームの解析
	if err := r.ParseForm(); err != nil {
		slog.Warn("Failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	limitStr := r.FormValue("panel_limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = defaultPanelLimit
		if limitStr != "" {
			slog.Warn("Invalid panel_limit value, using default",
				"input", limitStr,
				"default", limit,
			)
		}
	}

	// タスクペイロードの作成
	payload := domain.GenerateTaskPayload{
		ScriptURL:  r.FormValue("script_url"),
		Mode:       r.FormValue("mode"),
		PanelLimit: limit,
	}

	// 2. Cloud Tasks への投入準備（ログ出力）
	slog.Info("Enqueuing task", "script_url", payload.ScriptURL)

	// 3. 完了画面の描画（accepted.html を動的に結合）
	h.render(w, http.StatusAccepted, "accepted.html", AcceptedPageData{
		Title:     "受付完了なのだ",
		ScriptURL: payload.ScriptURL,
	})
}
