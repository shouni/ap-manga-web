package web

import (
	"fmt"
	"path"
	"strings"
)

func (h *Handler) validateAndCleanPath(title, file string) (string, error) {
	if title == "" || !validTitle.MatchString(title) {
		return "", fmt.Errorf("invalid title: %s", title)
	}

	baseDir := h.cfg.GetWorkDir(title)
	cleaned := path.Clean(path.Join(baseDir, file))

	if !strings.HasPrefix(cleaned, baseDir) {
		return "", fmt.Errorf("potential traversal: %s", cleaned)
	}
	return cleaned, nil
}
