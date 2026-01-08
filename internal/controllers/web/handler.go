package web

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
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
		log.Printf("[ERROR] Failed to execute template: %v", err)
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
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	limit, _ := strconv.Atoi(r.FormValue("panel_limit"))
	if limit <= 0 {
		limit = 10
	}

	// 2. タスクペイロードの構築
	payload := domain.GenerateTaskPayload{
		ScriptURL:  r.FormValue("script_url"),
		Mode:       r.FormValue("mode"),
		PanelLimit: limit,
	}

	// 3. 本来はここで Cloud Tasks に投げるのだ
	log.Printf("[INFO] タスクをキューに投入する準備中なのだ: %+v", payload)

	// 仮のレスポンス
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, "漫画生成の依頼を受け付けたのだ！\nURL: %s\nMode: %s\n完了まで数分待つのだ。", payload.ScriptURL, payload.Mode)
}
