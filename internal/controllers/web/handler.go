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

type AcceptedPageData struct {
	Title     string
	ScriptURL string
}

// Handler manages HTTP requests using a cache of pre-parsed template sets.
type Handler struct {
	cfg config.Config
	// ページ名（"index.html"など）をキーにして、合成済みのテンプレートを保持するのだ
	templateCache map[string]*template.Template
}

// NewHandler parses each page template combined with the base layout at startup.
func NewHandler(cfg config.Config) (*Handler, error) {
	cache := make(map[string]*template.Template)
	layoutPath := filepath.Join(cfg.TemplateDir, "layout.html")

	// 1. まず layout.html が存在するか確認するのだ
	if _, err := filepath.Glob(layoutPath); err != nil {
		return nil, fmt.Errorf("layout.html not found: %w", err)
	}

	// 2. ページごとのテンプレートファイルを特定するのだ
	pages := []string{"index.html", "accepted.html"}

	for _, page := range pages {
		pagePath := filepath.Join(cfg.TemplateDir, page)

		// layout.html と各ページを組み合わせて、独立したセットとしてパースするのだ
		// これで {{define "content"}} が衝突しなくなるのだよ！
		tmpl, err := template.ParseFiles(layoutPath, pagePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", page, err)
		}
		cache[page] = tmpl
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
	// layout.html をエントリーポイントとして実行するのだ
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
	limit, _ := strconv.Atoi(limitStr)
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
