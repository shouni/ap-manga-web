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

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

const defaultPanelLimit = 10

type IndexPageData struct {
	Title string
}

type AcceptedPageData struct {
	Title     string
	Command   string
	ScriptURL string
}

// Handler は事前にパースされたテンプレートキャッシュを使用して HTTP リクエストを管理するのだ。
type Handler struct {
	cfg           config.Config
	templateCache map[string]*template.Template
	taskAdapter   adapters.TaskAdapter
}

// NewHandler は起動時に各ページテンプレートとレイアウトを組み合わせてパースするのだ。
func NewHandler(cfg config.Config, taskAdapter adapters.TaskAdapter) (*Handler, error) {
	cache := make(map[string]*template.Template)
	layoutPath := filepath.Join(cfg.TemplateDir, "layout.html")

	// layout.html の存在を確認
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
			continue
		}

		// layout.html と各ページを独立したセットとしてパース
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

// render はキャッシュされたテンプレートセットを実行するヘルパーなのだ。
func (h *Handler) render(w http.ResponseWriter, status int, pageName string, data any) {
	tmpl, ok := h.templateCache[pageName]
	if !ok {
		slog.Error("Template not found in cache", "page", pageName)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	// layout.html をエントリーポイントとして実行
	if err := tmpl.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		slog.Error("Failed to render template", "page", pageName, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// Index はメインの入力画面（一括生成）を表示するのだ。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "index.html", IndexPageData{
		Title: "Generate - AP Manga Web",
	})
}

// Design はキャラ設計画面を表示するのだ
func (h *Handler) Design(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "design.html", IndexPageData{
		Title: "Character Design - AP Manga Web",
	})
}

// Script は台本抽出画面を表示するのだ
func (h *Handler) Script(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "script.html", IndexPageData{
		Title: "Script Generation - AP Manga Web",
	})
}

// Image は画像錬成画面を表示するのだ
func (h *Handler) Image(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "image.html", IndexPageData{
		Title: "Image Generation - AP Manga Web",
	})
}

// Story はプロット構成画面を表示するのだ
func (h *Handler) Story(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "story.html", IndexPageData{
		Title: "Story Boarding - AP Manga Web",
	})
}

// HandleSubmit は、あらゆるワークフロー（Generate/Design/Story等）のフォームを受け付けるのだ。
func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		slog.Warn("Failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 1. フォーム値の抽出と数値変換
	limitStr := r.FormValue("panel_limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		// 空文字列の場合はデフォルト値を使用するが、それ以外の不正な値の場合は警告ログを出力する
		if limitStr != "" {
			slog.WarnContext(ctx, "Invalid panel_limit value, using default",
				"input", limitStr,
				"default", defaultPanelLimit,
			)
		}
		limit = defaultPanelLimit
	}

	if limit <= 0 {
		limit = defaultPanelLimit
	}

	// 2. domain.GenerateTaskPayload へのマッピング
	// Command フィールドによってバックエンドの Pipeline 分岐が決定されるのだ。
	payload := domain.GenerateTaskPayload{
		Command:    r.FormValue("command"),
		ScriptURL:  r.FormValue("script_url"),
		InputText:  r.FormValue("input_text"),
		Mode:       r.FormValue("mode"),
		PanelLimit: limit,
	}

	// 最小限のバリデーション
	if payload.Command == "" {
		slog.Warn("Missing command in form submission")
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}

	slog.Info("Form submission received",
		"command", payload.Command,
		"url", payload.ScriptURL,
		"limit", payload.PanelLimit,
	)

	// 3. Cloud Tasks へのエンキュー実行
	// 注入された taskAdapter を使用して非同期ワーカーへジョブを投げるのだ。
	if err := h.taskAdapter.EnqueueGenerateTask(ctx, payload); err != nil {
		slog.Error("Failed to enqueue task",
			"error", err,
			"command", payload.Command,
		)
		http.Error(w, "Failed to schedule task", http.StatusInternalServerError)
		return
	}

	// 4. 受付完了画面の表示
	h.render(w, http.StatusAccepted, "accepted.html", AcceptedPageData{
		Title:     "Accepted - AP Manga Web",
		Command:   payload.Command,
		ScriptURL: payload.ScriptURL,
	})
}
