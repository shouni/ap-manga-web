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

// HandleSubmit processes form submissions for task generation requests.
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

// ServeOutput retrieves manga content (plot and images) and renders the viewer page.
func (h *Handler) ServeOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := chi.URLParam(r, "title")

	// 1. Plot content retrieval with improved resource management
	plotPath, err := h.validateAndCleanPath(title, "manga_plot.md")
	if err != nil {
		slog.WarnContext(ctx, "Path validation failed for plot", "title", title, "error", err)
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	var markdownContent string
	rc, err := h.reader.Open(ctx, plotPath)
	if err != nil {
		// File missing is handled as a warning, not a fatal error for the page.
		slog.WarnContext(ctx, "Plot file not found, skipping", "path", plotPath, "error", err)
	} else {
		defer rc.Close()
		data, readErr := io.ReadAll(rc)
		if readErr != nil {
			slog.ErrorContext(ctx, "Failed to read plot content", "path", plotPath, "error", readErr)
		} else {
			markdownContent = string(data)
		}
	}

	// 2. Image listing with error handling
	prefix, err := h.validateAndCleanPath(title, "images/")
	if err != nil {
		slog.ErrorContext(ctx, "Path validation failed for images prefix", "error", err)
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	gcsPrefix := fmt.Sprintf("gs://%s/%s", h.cfg.GCSBucket, prefix)
	var filePaths []string
	if err := h.reader.List(ctx, gcsPrefix, func(gcsPath string) error {
		if strings.HasSuffix(gcsPath, ".png") {
			filePaths = append(filePaths, gcsPath)
		}
		return nil
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to list images from GCS", "prefix", gcsPrefix, "error", err)
		http.Error(w, "Failed to retrieve image list", http.StatusInternalServerError)
		return
	}

	sort.Strings(filePaths)

	// 3. Generate Signed URLs with error logging
	var signedURLs []string
	expiration := 1 * time.Hour

	for _, gcsPath := range filePaths {
		url, err := h.signer.GenerateSignedURL(ctx, gcsPath, http.MethodGet, expiration)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to generate signed URL for image", "path", gcsPath, "error", err)
			continue
		}
		signedURLs = append(signedURLs, url)
	}

	h.render(w, http.StatusOK, "manga_view.html", title, map[string]any{
		"Title":       title,
		"ImageURLs":   signedURLs,
		"MarkdownRaw": markdownContent,
	})
}
