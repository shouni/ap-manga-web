package web

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"ap-manga-web/internal/config"
)

// IndexPageData は Home (Generate) 画面用のビューモデルです
type IndexPageData struct {
	Title string
}

type Handler struct {
	cfg  config.Config
	tmpl *template.Template
}

// NewHandler はテンプレートディレクトリ内の全てのHTMLをパースし、整合性をチェックします
func NewHandler(cfg config.Config) (*Handler, error) {
	pattern := filepath.Join(cfg.TemplateDir, "*.html")
	tmpl, err := template.ParseGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates from %s: %w", pattern, err)
	}

	// 指摘事項: 起動時に主要なテンプレート（layout.html）が存在するかチェック
	if tmpl.Lookup("layout.html") == nil {
		return nil, fmt.Errorf("essential template 'layout.html' not found in %s (parsed files: %v)", cfg.TemplateDir, tmpl.DefinedTemplates())
	}

	return &Handler{
		cfg:  cfg,
		tmpl: tmpl,
	}, nil
}

// Index はメインの漫画生成フォームを表示します
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	data := IndexPageData{
		Title: "Generate - Manga Runner",
	}

	// 指摘事項: bytes.Buffer を使用して、パース完了までレスポンス書き込みを待機（アトミックな送信）
	var buf bytes.Buffer
	err := h.tmpl.ExecuteTemplate(&buf, "layout.html", data)
	if err != nil {
		log.Printf("[ERROR] Failed to execute template 'layout.html': %v", err)
		// まだ書き込み前なので、安全に http.Error を送出できる
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 成功した場合のみ、Content-Type を設定して一気に書き出す
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := buf.WriteTo(w); err != nil {
		log.Printf("[ERROR] Failed to write response: %v", err)
	}
}
