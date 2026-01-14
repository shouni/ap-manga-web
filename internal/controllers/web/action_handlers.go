package web

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"ap-manga-web/internal/domain"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Warn("Failed to parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	targetPanels := r.FormValue("target_panels")
	if !validTargetPanels.MatchString(targetPanels) {
		slog.WarnContext(r.Context(), "Invalid characters in target_panels", "input", targetPanels)
		http.Error(w, "Bad Request: invalid panel format.", http.StatusBadRequest)
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
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}

	if err := h.taskAdapter.EnqueueGenerateTask(r.Context(), payload); err != nil {
		slog.Error("Failed to enqueue task", "error", err)
		http.Error(w, "Failed to schedule task", http.StatusInternalServerError)
		return
	}

	h.render(w, http.StatusAccepted, "accepted.html", "Accepted", payload)
}

func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := chi.URLParam(r, "title")

	plotPath, err := h.validateAndCleanPath(title, "manga_plot.md")
	if err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	var markdownContent string
	if rc, err := h.reader.Open(ctx, plotPath); err == nil {
		data, _ := io.ReadAll(rc)
		rc.Close()
		markdownContent = string(data)
	}

	prefix, _ := h.validateAndCleanPath(title, "images/")
	gcsPrefix := fmt.Sprintf("gs://%s/%s", h.cfg.GCSBucket, prefix)
	var filePaths []string
	_ = h.reader.List(ctx, gcsPrefix, func(gcsPath string) error {
		if strings.HasSuffix(gcsPath, ".png") {
			filePaths = append(filePaths, gcsPath)
		}
		return nil
	})

	sort.Strings(filePaths)

	var signedURLs []string
	for _, gcsPath := range filePaths {
		if url, err := h.signer.GenerateSignedURL(ctx, gcsPath, http.MethodGet, time.Hour); err == nil {
			signedURLs = append(signedURLs, url)
		}
	}

	h.render(w, http.StatusOK, "manga_view.html", title, map[string]any{
		"Title":       title,
		"ImageURLs":   signedURLs,
		"MarkdownRaw": markdownContent,
	})
}
