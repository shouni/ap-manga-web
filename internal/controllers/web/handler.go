package web

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

// 💡 defaultPanelLimit は不要になったので削除したのだ

type IndexPageData struct {
	Title string
}

type AcceptedPageData struct {
	Title     string
	Command   string
	ScriptURL string
}

type Handler struct {
	cfg           config.Config
	templateCache map[string]*template.Template
	taskAdapter   adapters.TaskAdapter
}

func NewHandler(cfg config.Config, taskAdapter adapters.TaskAdapter) (*Handler, error) {
	cache := make(map[string]*template.Template)
	layoutPath := filepath.Join(cfg.TemplateDir, "layout.html")

	if _, err := os.Stat(layoutPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("layout template not found: %s", layoutPath)
	}

	pagePaths, err := filepath.Glob(filepath.Join(cfg.TemplateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to glob for page templates: %w", err)
	}

	for _, pagePath := range pagePaths {
		pageName := filepath.Base(pagePath)
		if pageName == "layout.html" {
			continue
		}

		tmpl, err := template.ParseFiles(layoutPath, pagePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", pageName, err)
		}
		cache[pageName] = tmpl
	}

	return &Handler{
		cfg:           cfg,
		templateCache: cache,
		taskAdapter:   taskAdapter,
	}, nil
}

func (h *Handler) render(w http.ResponseWriter, status int, pageName string, data any) {
	tmpl, ok := h.templateCache[pageName]
	if !ok {
		slog.Error("Template not found in cache", "page", pageName)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		slog.Error("Failed to render template", "page", pageName, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// 各画面の表示関数（変更なし）
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "index.html", IndexPageData{Title: "Generate - AP Manga Web"})
}
func (h *Handler) Design(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "design.html", IndexPageData{Title: "Character Design - AP Manga Web"})
}
func (h *Handler) Script(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "script.html", IndexPageData{Title: "Script Generation - AP Manga Web"})
}
func (h *Handler) Image(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "image.html", IndexPageData{Title: "Image Generation - AP Manga Web"})
}
func (h *Handler) Story(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "story.html", IndexPageData{Title: "Story Boarding - AP Manga Web"})
}

// HandleSubmit は、HTMLフォームからの送信を処理するのだ。
func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		slog.Warn("Failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 💡 domain.GenerateTaskPayload へのマッピングを最新化したのだ
	payload := domain.GenerateTaskPayload{
		Command:   r.FormValue("command"),
		ScriptURL: r.FormValue("script_url"),
		InputText: r.FormValue("input_text"),
		Mode:      r.FormValue("mode"),
		// 💡 panel_limit のパースを廃止し、target_panels を取得するように変更！
		TargetPanels: r.FormValue("target_panels"),
	}

	if payload.Command == "" {
		slog.Warn("Missing command in form submission")
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}

	slog.Info("Form submission received",
		"command", payload.Command,
		"url", payload.ScriptURL,
		"panels", payload.TargetPanels, // ログも変更なのだ
	)

	if err := h.taskAdapter.EnqueueGenerateTask(ctx, payload); err != nil {
		slog.Error("Failed to enqueue task",
			"error", err,
			"command", payload.Command,
		)
		http.Error(w, "Failed to schedule task", http.StatusInternalServerError)
		return
	}

	h.render(w, http.StatusAccepted, "accepted.html", AcceptedPageData{
		Title:     "Accepted - AP Manga Web",
		Command:   payload.Command,
		ScriptURL: payload.ScriptURL,
	})
}
