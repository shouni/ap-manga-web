package web

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

const defaultPanelLimit = 10

type IndexPageData struct {
	Title string
}

type AcceptedPageData struct {
	Title     string
	ScriptURL string
}

// Handler manages HTTP requests using a cache of pre-parsed template sets.
type Handler struct {
	cfg           config.Config
	templateCache map[string]*template.Template
}

// NewHandler parses each page template combined with the base layout at startup.
func NewHandler(cfg config.Config) (*Handler, error) {
	cache := make(map[string]*template.Template)
	layoutPath := filepath.Join(cfg.TemplateDir, "layout.html")

	// layout.html の存在を最初に確認する
	if _, err := os.Stat(layoutPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("layout template not found: %s", layoutPath)
	}

	// layout.html を除く全ての .html ファイルをページとして取得
	pagePaths, err := filepath.Glob(filepath.Join(cfg.TemplateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to glob for page templates: %w", err)
	}

	for _, pagePath := range pagePaths {
		pageName := filepath.Base(pagePath)
		if pageName == "layout.html" {
			continue // レイアウトファイル自体はスキップ
		}

		// layout.html と各ページを組み合わせて、独立したセットとしてパースする
		tmpl, err := template.ParseFiles(layoutPath, pagePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", pageName, err)
		}
		cache[pageName] = tmpl
	}

	return &Handler{
		cfg:           cfg,
		templateCache: cache,
	}, nil
}

// render executes a pre-cached template set based on the provided page name.
func (h *Handler) render(w http.ResponseWriter, status int, pageName string, data any) {
	tmpl, ok := h.templateCache[pageName]
	if !ok {
		slog.Error("Template not found in cache", "page", pageName)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	// layout.html をエントリーポイントとして実行する
	if err := tmpl.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		slog.Error("Failed to render template", "page", pageName, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "index.html", IndexPageData{
		Title: "Generate - Manga Runner",
	})
}

func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Warn("Failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	limitStr := r.FormValue("panel_limit")
	limit, err := strconv.Atoi(limitStr)
	// limitStrが空、または不正な値の場合にデフォルト値を使用する
	if err != nil {
		// 空文字列の場合はエラーではなく、単にデフォルト値を使用する
		if limitStr != "" {
			slog.Warn("Invalid panel_limit value, using default",
				"input", limitStr,
				"default", defaultPanelLimit,
			)
		}
		limit = defaultPanelLimit
	}

	if limit <= 0 {
		limit = defaultPanelLimit
	}

	payload := domain.GenerateTaskPayload{
		ScriptURL:  r.FormValue("script_url"),
		Mode:       r.FormValue("mode"),
		PanelLimit: limit,
	}

	slog.Info("Enqueuing task",
		"script_url", payload.ScriptURL,
		"mode", payload.Mode,
		"panel_limit", payload.PanelLimit,
	)

	h.render(w, http.StatusAccepted, "accepted.html", AcceptedPageData{
		Title:     "Accepted - Manga Runner",
		ScriptURL: payload.ScriptURL,
	})
}
