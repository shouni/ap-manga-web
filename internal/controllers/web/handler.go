package web

import (
	"html/template"
	"net/http"

	"ap-manga-web/internal/config"
)

type Handler struct {
	cfg  config.Config
	tmpl *template.Template
}

func NewHandler(cfg config.Config) *Handler {
	// layout.html と 各コンテンツ用 html をパースする
	tmpl := template.Must(template.ParseFiles(cfg.TemplatePath, "templates/layout.html"))
	return &Handler{cfg: cfg, tmpl: tmpl}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Home",
	}
	// layout.html 内の content template を index.html で定義したもので上書きして実行
	h.tmpl.ExecuteTemplate(w, "layout.html", data)
}
