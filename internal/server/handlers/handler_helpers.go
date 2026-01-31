package handlers

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"
)

var validTitle = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// render は HTML テンプレートをレンダリングし、レスポンスを書き込みます。
func (h *Handler) render(w http.ResponseWriter, status int, pageName string, title string, data any) {
	tmpl, ok := h.templateCache[pageName]
	if !ok {
		slog.Error("キャッシュ内にテンプレートが見つかりません", "page", pageName)
		http.Error(w, "システムエラーが発生しました（テンプレート未定義）", http.StatusInternalServerError)
		return
	}

	renderData := struct {
		Title string
		Data  any
	}{
		Title: title + titleSuffix,
		Data:  data,
	}

	var buf bytes.Buffer
	// レイアウトファイルをベースに実行します
	if err := tmpl.ExecuteTemplate(&buf, "layout.html", renderData); err != nil {
		slog.Error("テンプレートのレンダリングに失敗しました", "page", pageName, "error", err)
		http.Error(w, "画面の表示中にエラーが発生しました", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := buf.WriteTo(w); err != nil {
		slog.Error("レスポンスの書き込みに失敗しました", "error", err)
	}
}

// validateAndCleanPath タイトルを検証し、指定されたワークスペース内に安全でクリーンなファイル パスを構築します
func (h *Handler) validateAndCleanPath(title, file string) (string, error) {
	if title == "" || !validTitle.MatchString(title) {
		return "", fmt.Errorf("invalid title: %s", title)
	}

	baseDir := h.appCtx.Config.GetWorkDir(title)
	cleaned := path.Clean(path.Join(baseDir, file))

	if !strings.HasPrefix(cleaned, baseDir) {
		return "", fmt.Errorf("potential traversal: %s", cleaned)
	}
	return cleaned, nil
}
