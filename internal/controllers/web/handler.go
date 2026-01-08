package web

import (
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

// NewHandler はテンプレートディレクトリ内の全てのHTMLをパースして初期化します
func NewHandler(cfg config.Config) (*Handler, error) {
	// 指定されたディレクトリ（例: templates/*.html）をまとめてパース
	pattern := filepath.Join(cfg.TemplateDir, "*.html")
	tmpl, err := template.ParseGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates from %s: %w", pattern, err)
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

	// テンプレートの実行とエラーハンドリング
	err := h.tmpl.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		log.Printf("[ERROR] Failed to execute template 'layout.html': %v", err)
		// ヘッダーが既に書き込まれていない場合のみエラーを返す
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// 各画面のハンドラーも同様の構造で追加可能
// func (h *Handler) Design(w http.ResponseWriter, r *http.Request) { ... }
