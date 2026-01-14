package web

import (
	"ap-manga-web/internal/domain"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"

	"github.com/go-chi/chi/v5"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

var (
	validTargetPanels = regexp.MustCompile(`^[0-9, ]*$`)
	validTitle        = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

const titleSuffix = " - AP Manga Web"

type Handler struct {
	cfg           config.Config
	templateCache map[string]*template.Template
	taskAdapter   adapters.TaskAdapter
	reader        remoteio.InputReader
	signer        remoteio.URLSigner
}

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

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	for _, pagePath := range pagePaths {
		pageName := filepath.Base(pagePath)
		if pageName == "layout.html" {
			continue
		}

		tmpl := template.New(pageName).Funcs(funcMap)
		tmpl, err = tmpl.ParseFiles(layoutPath, pagePath)
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

func (h *Handler) render(w http.ResponseWriter, status int, pageName string, title string, data any) {
	tmpl, ok := h.templateCache[pageName]
	if !ok {
		slog.Error("Template not found in cache", "page", pageName)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

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

func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := chi.URLParam(r, "title")

	// 1. Plot content retrieval with proper error handling
	plotPath, err := h.validateAndCleanPath(title, "manga_plot.md")
	if err != nil {
		slog.WarnContext(ctx, "Path validation failed for plot", "title", title, "error", err)
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	var markdownContent string
	rc, err := h.reader.Open(ctx, plotPath)
	if err != nil {
		slog.WarnContext(ctx, "Failed to open plot file", "path", plotPath, "error", err)
		// Continue without markdown content if file is missing
	} else {
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			slog.ErrorContext(ctx, "Failed to read plot content", "path", plotPath, "error", err)
		} else {
			markdownContent = string(data)
		}
	}

	// 2. Image listing
	prefix, err := h.validateAndCleanPath(title, "images/")
	if err != nil {
		slog.ErrorContext(ctx, "Path validation failed for images prefix", "error", err)
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	gcsPrefix := fmt.Sprintf("gs://%s/%s", h.cfg.GCSBucket, prefix)
	var filePaths []string
	err = h.reader.List(ctx, gcsPrefix, func(gcsPath string) error {
		if strings.HasSuffix(gcsPath, ".png") {
			filePaths = append(filePaths, gcsPath)
		}
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to list images", "prefix", gcsPrefix, "error", err)
	}

	sort.Strings(filePaths)

	// 3. Generate Signed URLs instead of Base64 encoding
	// This reduces server CPU/Memory usage and improves scalability.
	var signedURLs []string
	expiration := 1 * time.Hour // Default expiration

	for _, gcsPath := range filePaths {
		signedURL, err := h.signer.GenerateSignedURL(ctx, gcsPath, http.MethodGet, expiration)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to generate signed URL", "path", gcsPath, "error", err)
			continue // Skip failed images
		}
		signedURLs = append(signedURLs, signedURL)
	}

	h.render(w, http.StatusOK, "manga_view.html", title, map[string]any{
		"Title":       title,
		"ImageURLs":   signedURLs,
		"MarkdownRaw": markdownContent,
	})
}

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
