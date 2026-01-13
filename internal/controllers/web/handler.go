package web

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

var (
	validTargetPanels = regexp.MustCompile(`^[0-9, ]*$`)
	validTitle        = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// ページタイトルのプレフィックスを一元管理
const titleSuffix = " - AP Manga Web"

type Handler struct {
	cfg           config.Config
	templateCache map[string]*template.Template
	taskAdapter   adapters.TaskAdapter
	reader        remoteio.InputReader
	signer        remoteio.URLSigner
}

// NewHandler initializes a new Handler struct by loading templates, setting up configuration, and preparing dependencies.
// It returns a pointer to the Handler instance or an error if initialization fails.
func NewHandler(cfg config.Config, taskAdapter adapters.TaskAdapter, reader remoteio.InputReader, signer remoteio.URLSigner) (*Handler, error) {
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
		reader:        reader,
		signer:        signer,
	}, nil
}

// render はテンプレートのレンダリングを一手に引き受けます。
func (h *Handler) render(w http.ResponseWriter, status int, pageName string, title string, data any) {
	tmpl, ok := h.templateCache[pageName]
	if !ok {
		slog.Error("Template not found in cache", "page", pageName)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 共通のデータ構造を構築
	renderData := struct {
		Title string
		Data  any
	}{
		Title: title + titleSuffix,
		Data:  data,
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout.html", renderData); err != nil {
		slog.Error("Failed to render template", "page", pageName, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// --- 画面表示メソッド ---

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "index.html", "Generate", nil)
}
func (h *Handler) Design(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "design.html", "Character Design", nil)
}
func (h *Handler) Script(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "script.html", "Script Generation", nil)
}
func (h *Handler) Panel(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "panel.html", "Panel Generation", nil)
}
func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "page.html", "Page Layout", nil)
}

// --- アクションメソッド ---

// HandleSubmit processes form submissions for task generation requests and enqueues tasks via the task adapter.
// It validates inputs, parses the form, and builds a task payload. On success, renders an acceptance response.
func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Warn("Failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	targetPanels := r.FormValue("target_panels")
	if !validTargetPanels.MatchString(targetPanels) {
		slog.WarnContext(r.Context(), "Invalid characters in target_panels", "input", targetPanels)
		http.Error(w, "Bad Request: invalid panel format.", http.StatusBadRequest)
		return
	}

	payload := domain.GenerateTaskPayload{
		Command:      r.FormValue("command"),
		ScriptURL:    r.FormValue("script_url"),
		InputText:    r.FormValue("input_text"),
		Mode:         r.FormValue("mode"),
		TargetPanels: targetPanels,
	}

	if payload.Command == "" {
		slog.WarnContext(r.Context(), "Form submission rejected: command is missing")
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}

	if err := h.taskAdapter.EnqueueGenerateTask(r.Context(), payload); err != nil {
		slog.Error("Failed to enqueue task", "error", err, "command", payload.Command)
		http.Error(w, "Failed to schedule task", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "Successfully enqueued generation task",
		"command", payload.Command,
		"script_url", payload.ScriptURL,
		"target_panels", payload.TargetPanels,
	)

	h.render(w, http.StatusAccepted, "accepted.html", "Accepted", payload)
}

// ServeOutput handles incoming requests to retrieve generated content by redirecting to a signed GCS URL.
// It validates the request parameters, ensures path safety, and generates a temporary signed URL for secure access.
// If validation or URL generation fails, the method sends an appropriate HTTP error response.
func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := chi.URLParam(r, "title")
	file := chi.URLParam(r, "*")

	if file == "" {
		file = domain.DefaultOutputFile
	}

	// パスの安全性を検証
	safePath, err := h.validateAndCleanPath(title, file)
	if err != nil {
		slog.WarnContext(ctx, "Security alert: path validation failed", "error", err, "remote_addr", r.RemoteAddr)
		http.Error(w, "Invalid path parameters", http.StatusForbidden)
		return
	}

	gcsURI := h.cfg.GetGCSObjectURL(safePath)
	signedURL, err := h.signer.GenerateSignedURL(ctx, gcsURI, http.MethodGet, h.cfg.SignedURLExpiration)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to generate signed URL", "uri", gcsURI, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, signedURL, http.StatusTemporaryRedirect)
}

// validateAndCleanPath はパスの安全性を検証し、クリーニングされたパスを返します。
func (h *Handler) validateAndCleanPath(title, file string) (string, error) {
	if title == "" || !validTitle.MatchString(title) {
		return "", fmt.Errorf("invalid title: %s", title)
	}

	baseDir := h.cfg.GetWorkDir(title)
	cleaned := path.Clean(path.Join(baseDir, file))

	if !strings.HasPrefix(cleaned, baseDir) {
		return "", fmt.Errorf("potential traversal: %s", cleaned)
	}
	return cleaned, nil
}
