package handlers

import (
	"errors"
	"net/http"
)

// ErrInvalidPath は、リクエストされたパスがサニタイズ検証に失敗したことを示します。
var ErrInvalidPath = errors.New("invalid path provided")

// Index は、漫画生成フォームのトップページを表示します。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "index.html", "Generate", nil)
}

// Design は、キャラクターデザイン生成フォームを表示します。
func (h *Handler) Design(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "design.html", "Character Design", nil)
}

// Script は、漫画スクリプト生成フォームを表示します。
func (h *Handler) Script(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "script.html", "Script Generation", nil)
}

// Panel は、パネル画像生成フォームを表示します。
func (h *Handler) Panel(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "panel.html", "Panel Generation", nil)
}

// Page は、ページレイアウト生成フォームを表示します。
func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "page.html", "Page Layout", nil)
}
