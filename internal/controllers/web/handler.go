package web

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog" // log を log/slog に変更
	"net/http"
	"path/filepath"
	"strconv"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

type IndexPageData struct {
	Title string
}

type Handler struct {
	cfg  config.Config
	tmpl *template.Template
}

func NewHandler(cfg config.Config) (*Handler, error) {
	pattern := filepath.Join(cfg.TemplateDir, "*.html")
	tmpl, err := template.ParseGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates from %s: %w", pattern, err)
	}

	if tmpl.Lookup("layout.html") == nil {
		return nil, fmt.Errorf("main template 'layout.html' not found in %s", pattern)
	}

	return &Handler{
		cfg:  cfg,
		tmpl: tmpl,
	}, nil
}

// Index はメイン画面を表示するのだ
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	data := IndexPageData{
		Title: "Generate - Manga Runner",
	}

	var buf bytes.Buffer
	err := h.tmpl.ExecuteTemplate(&buf, "layout.html", data)
	if err != nil {
		// slog.Error を使用して構造化ログを出力
		slog.Error("Failed to execute template",
			"template", "layout.html",
			"error", err,
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
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
		limit = 10
		if limitStr != "" {
			// 不正な数値入力があった場合は警告ログを残す
			slog.Warn("Invalid panel_limit value, using default",
				"input", limitStr,
				"default", limit,
			)
		}
	}

	// 2. タスクペイロードの構築
	payload := domain.GenerateTaskPayload{
		ScriptURL:  r.FormValue("script_url"),
		Mode:       r.FormValue("mode"),
		PanelLimit: limit,
	}

	// 3. Cloud Tasks への投入準備（構造化データとして出力）
	slog.Info("Preparing to enqueue task",
		"script_url", payload.ScriptURL,
		"mode", payload.Mode,
		"panel_limit", payload.PanelLimit,
	)

	// 仮のレスポンス
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, "漫画生成の依頼を受け付けたのだ！\nURL: %s\nMode: %s\n完了まで数分待つのだ。", payload.ScriptURL, payload.Mode)
}
