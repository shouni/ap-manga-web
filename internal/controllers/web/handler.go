package web

import (
	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

// target_panels のバリデーション（数字、カンマ、スペースのみ許可）
var validTargetPanels = regexp.MustCompile(`^[0-9, ]*$`)

type IndexPageData struct {
	Title string
}

type AcceptedPageData struct {
	Title     string
	Command   string
	ScriptURL string
}

// Handler はテンプレート管理とリクエスト処理を行うのだ。
type Handler struct {
	cfg           config.Config
	templateCache map[string]*template.Template
	taskAdapter   adapters.TaskAdapter
	reader        remoteio.InputReader
}

// NewHandler はテンプレートをキャッシュし、ハンドラーを初期化するのだ。
func NewHandler(cfg config.Config, taskAdapter adapters.TaskAdapter, reader remoteio.InputReader) (*Handler, error) {
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

// HandleSubmit は、HTMLフォームからの送信を処理し、タスクをエンキューするのだ。
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

// ServeOutput は GCS に保存された成果物を配信するのだ。
func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// URLパラメータから取得
	title := chi.URLParam(r, "title")
	file := chi.URLParam(r, "*")

	// ファイル指定がない、またはスラッシュのみの場合はデフォルトのHTMLを表示するのだ
	if file == "" || file == "/" {
		file = "manga_plot.html"
	}

	// パス トラバーサル対策
	// GCS上の構成に合わせて "output" をベースにするのだ
	const baseDir = "output"
	safeSubPath := path.Join(baseDir, title, file)

	// 正規化後のパスが依然として "output/" 配下であることを厳格に確認するのだ
	if !strings.HasPrefix(safeSubPath, baseDir+"/") {
		slog.WarnContext(ctx, "Security alert: attempted path traversal", "input_title", title, "input_file", file)
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// GCS上の絶対パスを構築するのだ
	gcsPath := fmt.Sprintf("gs://%s/%s", h.cfg.GCSBucket, safeSubPath)

	// 1. InputReader.Open を使ってストリームを開くのだ
	rc, err := h.reader.Open(ctx, gcsPath)
	if err != nil {
		// 404のデバッグ用にパスをログに出力しておくのだ
		slog.ErrorContext(ctx, "GCS open error for output", "path", gcsPath, "error", err)
		http.Error(w, "Output not found", http.StatusNotFound)
		return
	}
	defer rc.Close()

	// 2. 拡張子から Content-Type を判定するのだ
	ext := path.Ext(file)
	contentType := mime.TypeByExtension(ext)

	if ext == ".md" {
		contentType = "text/markdown; charset=utf-8"
	} else if ext == ".html" {
		contentType = "text/html; charset=utf-8"
	} else if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 3. ヘッダーをセットしてブラウザに流し込むのだ
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, rc); err != nil {
		slog.ErrorContext(ctx, "Failed to stream output to response", "path", gcsPath, "error", err)
	}
}
