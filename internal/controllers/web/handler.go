package web

import (
	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

// target_panels のバリデーション
var validTargetPanels = regexp.MustCompile(`^[0-9, ]*$`)

type IndexPageData struct {
	Title string
}

type AcceptedPageData struct {
	Title     string
	Command   string
	ScriptURL string
}

// Handler はテンプレート管理とリクエスト処理を制御します。
type Handler struct {
	cfg           config.Config
	templateCache map[string]*template.Template
	taskAdapter   adapters.TaskAdapter
	reader        remoteio.InputReader
	signer        remoteio.URLSigner
}

// NewHandler はテンプレートをキャッシュし、ハンドラーを初期化します。
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

// --- 画面表示メソッド ---

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "index.html", IndexPageData{Title: "Generate - AP Manga Web"})
}
func (h *Handler) Design(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "design.html", IndexPageData{Title: "Character Design - AP Manga Web"})
}
func (h *Handler) Script(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "script.html", IndexPageData{Title: "Script Generation - AP Manga Web"})
}
func (h *Handler) Panel(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "panel.html", IndexPageData{Title: "Panel Generation - AP Manga Web"})
}
func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "page.html", IndexPageData{Title: "Page Layout - AP Manga Web"})
}

// HandleSubmit は、HTMLフォームからの送信を処理し、非同期タスクをエンキューします。
func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		slog.Warn("Failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	targetPanels := r.FormValue("target_panels")
	if !validTargetPanels.MatchString(targetPanels) {
		slog.WarnContext(ctx, "Invalid characters in target_panels", "input", targetPanels)
		http.Error(w, "Bad Request: target_panels contains invalid characters.", http.StatusBadRequest)
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
		slog.Warn("Missing command in form submission")
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}

	slog.Info("Form submission received",
		"command", payload.Command,
		"url", payload.ScriptURL,
		"panels", payload.TargetPanels,
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

// ServeOutput は GCS に保存された成果物をクライアントに配信します。
func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := chi.URLParam(r, "title")
	file := chi.URLParam(r, "*")

	// 1. パスのバリデーション（安全確認）
	if title == "" || strings.Contains(title, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	if file == "" || file == "/" {
		file = "manga_plot.html"
	}

	// 2. GCS上のオブジェクトパスを構築
	// config.GetWorkDir を通じてベースパス(output/ 等)を強制する
	objectPath := path.Join(h.cfg.GetWorkDir(title), file)
	gcsURI := h.cfg.GetGCSObjectURL(objectPath)

	// 3. 署名付きURLの生成（15分間有効など）
	signedURL, err := h.signer.GenerateSignedURL(ctx, gcsURI, "GET", 15*time.Minute)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to sign URL", "path", gcsURI, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 4. クライアントを署名付きURLへリダイレクト
	// 302 (Found) または 307 (Temporary Redirect) を使用します
	http.Redirect(w, r, signedURL, http.StatusFound)
}
