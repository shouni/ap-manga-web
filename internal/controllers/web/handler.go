package web

import (
	"bytes"
	"errors"
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

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"

	"cloud.google.com/go/storage"
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

// Handler はテンプレート管理とリクエスト処理を制御します。
type Handler struct {
	cfg           config.Config
	templateCache map[string]*template.Template
	taskAdapter   adapters.TaskAdapter
	reader        remoteio.InputReader
}

// NewHandler はテンプレートをキャッシュし、ハンドラーを初期化します。
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
// ルーティング設定 r.Get("/output/{title}/*", h.ServeOutput) を想定しています。
func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. URLパラメータからタイトルと相対パスを取得
	title := chi.URLParam(r, "title")
	remainingPath := chi.URLParam(r, "*")

	// タイトルのバリデーション（空チェックとディレクトリ区切り文字の拒絶）
	if title == "" || strings.ContainsAny(title, "/\\") {
		slog.WarnContext(ctx, "Security alert: invalid title parameter", "input_title", title)
		http.Error(w, "Invalid title parameter", http.StatusBadRequest)
		return
	}

	// 2. ファイル指定がない、またはスラッシュのみの場合は、デフォルトのHTMLを使用
	file := remainingPath
	if file == "" || file == "/" {
		file = "manga_plot.html"
	}

	// 3. パスの正規化と厳格な検証
	// title と file を結合し、path.Clean により ".." 等を解決します。
	safeSubPath := path.Clean(path.Join(title, file))

	// 正規化後のパスが、意図した title ディレクトリ配下であることを厳格に検証します。
	// file に "../another_title/file" 等を指定して title 外に脱出する攻撃を防ぎます。
	if !strings.HasPrefix(safeSubPath, title+"/") && safeSubPath != title {
		slog.WarnContext(ctx, "Security alert: attempted path traversal",
			"input_title", title,
			"input_file", file,
			"result_path", safeSubPath)
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// 4. GCS上の絶対パスを構築
	// GCS上の構造: gs://[BUCKET]/output/[TITLE]/[FILE]
	gcsKey := path.Join("output", safeSubPath)
	gcsPath := fmt.Sprintf("gs://%s/%s", h.cfg.GCSBucket, gcsKey)

	// 5. GCSからオブジェクトのストリームを取得
	rc, err := h.reader.Open(ctx, gcsPath)
	if err != nil {
		slog.ErrorContext(ctx, "GCS open error for output", "path", gcsPath, "error", err)

		if errors.Is(err, storage.ErrObjectNotExist) {
			http.Error(w, "Output not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	defer rc.Close()

	// 6. Content-Type の決定 [Minor 対策: 安定性の確保]
	ext := path.Ext(file)
	var contentType string

	// 特定の拡張子について Content-Type と charset を優先的に指定します。
	switch ext {
	case ".md":
		contentType = "text/markdown; charset=utf-8"
	case ".html":
		contentType = "text/html; charset=utf-8"
	default:
		// それ以外は環境の mime.types に基づいて推測します。
		contentType = mime.TypeByExtension(ext)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 7. ヘッダー設定とデータ転送
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, rc); err != nil {
		slog.ErrorContext(ctx, "Failed to stream output to response", "path", gcsPath, "error", err)
	}
}
