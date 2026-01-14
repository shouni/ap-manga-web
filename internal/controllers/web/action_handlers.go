package web

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"ap-manga-web/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/shouni/go-manga-kit/pkg/asset"
)

// pageFileRegex はパッケージレベルで一度だけコンパイルする
var pageFileRegex = func() *regexp.Regexp {
	pageFile := asset.DefaultPageFileName
	baseName := strings.TrimSuffix(pageFile, filepath.Ext(pageFile))
	pattern := fmt.Sprintf(`%s_page_\d+\.png$`, regexp.QuoteMeta(baseName))
	return regexp.MustCompile(pattern)
}()

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

	// Plot content retrieval
	relPath, err := h.validateAndCleanPath(title, asset.DefaultMangaPlotName)
	if err != nil {
		slog.WarnContext(ctx, "Path validation failed for plot", "title", title, "error", err)
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	var plotPath string
	if h.cfg.GCSBucket != "" {
		// gs://bucket-name/relPath の形に結合
		plotPath = fmt.Sprintf("gs://%s/%s", h.cfg.GCSBucket, relPath)
	} else {
		plotPath = relPath
	}

	slog.InfoContext(ctx, "Resolved plot path", "path", plotPath)

	var markdownContent string
	if rc, err := h.reader.Open(ctx, plotPath); err == nil {
		defer rc.Close()
		if data, readErr := io.ReadAll(rc); readErr == nil {
			markdownContent = string(data)
		} else {
			slog.ErrorContext(ctx, "Failed to read plot content", "path", plotPath, "error", readErr)
		}
	} else {
		slog.WarnContext(ctx, "Plot file not found, skipping", "path", plotPath, "error", err)
	}

	prefix, err := h.validateAndCleanPath(title, asset.DefaultImageDir)
	if err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	gcsPrefix := fmt.Sprintf("gs://%s/%s", h.cfg.GCSBucket, prefix)
	var filePaths []string
	if err := h.reader.List(ctx, gcsPrefix, func(gcsPath string) error {
		// GCSからリストしたパスが、期待されるページ画像のファイル名パターンに一致するかを検証します。
		// この正規表現は、`asset.DefaultPageFileName` に基づいて動的に生成されます。
		if pageFileRegex.MatchString(gcsPath) {
			filePaths = append(filePaths, gcsPath)
		}
		return nil
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to list images from GCS", "prefix", gcsPrefix, "error", err)
		http.Error(w, "Failed to retrieve image list", http.StatusInternalServerError)
		return
	}

	// Sort and Generate Signed URLs
	sort.Strings(filePaths)

	var signedURLs []string
	for _, gcsPath := range filePaths {
		if url, err := h.signer.GenerateSignedURL(ctx, gcsPath, http.MethodGet, time.Hour); err == nil {
			signedURLs = append(signedURLs, url)
		} else {
			slog.ErrorContext(ctx, "Failed to generate signed URL", "path", gcsPath, "error", err)
		}
	}

	h.render(w, http.StatusOK, "manga_view.html", title, map[string]any{
		"Title":       title,
		"ImageURLs":   signedURLs,
		"MarkdownRaw": markdownContent,
	})
}
