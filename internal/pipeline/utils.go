package pipeline

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ap-manga-web/internal/domain"

	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
)

var invalidPathChars = regexp.MustCompile(`[\\/:\*\?"<>\|]`)

func (p *MangaPipeline) getSafeTitle(title string, t time.Time) string {
	h := md5.New()
	io.WriteString(h, title)
	hash := fmt.Sprintf("%x", h.Sum(nil))[:8]
	return fmt.Sprintf("%s_%s", t.Format("20060102_150405"), hash)
}

func (p *MangaPipeline) parseTargetPanels(ctx context.Context, s string, total int) []int {
	if strings.TrimSpace(s) == "" {
		res := make([]int, total)
		for i := 0; i < total; i++ {
			res[i] = i
		}
		return res
	}
	var res []int
	for _, part := range strings.Split(s, ",") {
		if idx, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && idx >= 0 && idx < total {
			res = append(res, idx)
		}
	}
	return res
}

func (p *MangaPipeline) parseCSV(input string) []string {
	var res []string
	for _, s := range strings.Split(input, ",") {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

func (p *MangaPipeline) buildMangaNotification(payload domain.GenerateTaskPayload, manga mangadom.MangaResponse, t time.Time) (*domain.NotificationRequest, string, string) {
	safeTitle := p.getSafeTitle(manga.Title, t)
	publicURL, _ := url.JoinPath(p.appCtx.Config.ServiceURL, "outputs", safeTitle)
	storageURI := fmt.Sprintf("gs://%s/output/%s", p.appCtx.Config.GCSBucket, safeTitle)
	return &domain.NotificationRequest{
		SourceURL: payload.ScriptURL, OutputCategory: "manga-output",
		TargetTitle: manga.Title, ExecutionMode: payload.Command + " / " + payload.Mode,
	}, publicURL, storageURI
}

func (p *MangaPipeline) buildScriptNotification(payload domain.GenerateTaskPayload, manga mangadom.MangaResponse, path string) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL: payload.ScriptURL, OutputCategory: "script-json",
		TargetTitle: manga.Title, ExecutionMode: "script-only",
	}, "N/A", path
}

func (p *MangaPipeline) buildDesignNotification(payload domain.GenerateTaskPayload, url string, seed int64) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL: "N/A (Design)", OutputCategory: "design-sheet",
		TargetTitle:   fmt.Sprintf("Design: %s (Seed: %d)", payload.InputText, seed),
		ExecutionMode: "design",
	}, url, "N/A"
}
