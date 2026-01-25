package handlers

import (
	"errors"
	"net/http"
)

var ErrInvalidPath = errors.New("invalid path provided")

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "index.html", "Generate", nil)
}
func (h *Handler) Design(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "design.html", "Character Design", nil)
}
func (h *Handler) Script(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "script.html", "Script Generation", nil)
}
func (h *Handler) Panel(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "panel.html", "Panel Generation", nil)
}
func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "page.html", "Page Layout", nil)
}
