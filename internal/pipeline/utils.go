package pipeline

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"ap-manga-web/internal/domain"

	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/publisher"
)

// resolveSafeTitle は実行開始時刻とタイトルから一意で安全なディレクトリ名を生成するのだ
func (p *MangaPipeline) resolveSafeTitle(title string) string {
	if p.resolvedSafeTitle != "" {
		return p.resolvedSafeTitle
	}

	t := p.startTime
	if t.IsZero() {
		t = time.Now()
	}

	// 1. time.LoadLocation による堅牢なJST変換
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		// Dockerにtzdataがない場合などのフォールバックなのだ
		slog.Warn("Failed to load Asia/Tokyo location, using FixedZone", "error", err)
		jst = time.FixedZone("Asia/Tokyo", 9*60*60)
	}
	tJST := t.In(jst)

	// 2. 効率的で安全なハッシュ生成
	h := md5.New()
	h.Write([]byte(title))

	// ナノ秒をバイナリ形式で直接書き込んで衝突を防止するのだ
	nanoBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nanoBytes, uint64(t.UnixNano()))
	h.Write(nanoBytes)

	hash := fmt.Sprintf("%x", h.Sum(nil))[:8]
	p.resolvedSafeTitle = fmt.Sprintf("%s_%s", tJST.Format("20060102_150405"), hash)

	// 例: 20260112_174500_a1b2c3d4
	return fmt.Sprintf("%s_%s", tJST.Format("20060102_150405"), hash)
}

// buildMangaNotification は生成結果を基にSlack通知用のリクエストを構築するのだ
func (p *MangaPipeline) buildMangaNotification(
	payload domain.GenerateTaskPayload,
	manga mangadom.MangaResponse,
	result publisher.PublishResult,
) (*domain.NotificationRequest, string, string) {
	safeTitle := p.resolveSafeTitle(manga.Title)
	publicURL, _ := url.JoinPath(p.appCtx.Config.ServiceURL, "outputs", safeTitle)

	// 3. url.Parse のエラー可視化
	storageURI := "N/A"
	if result.MarkdownPath != "" {
		u, err := url.Parse(result.MarkdownPath)
		if err != nil {
			slog.Error("Failed to parse MarkdownPath for storageURI", "path", result.MarkdownPath, "error", err)
		} else {
			u.Path = path.Dir(u.Path)
			storageURI = u.String()
		}
	}

	return &domain.NotificationRequest{
		SourceURL:      payload.ScriptURL,
		OutputCategory: "manga-output",
		TargetTitle:    manga.Title,
		ExecutionMode:  payload.Command + " / " + payload.Mode,
	}, publicURL, storageURI
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

func (p *MangaPipeline) buildScriptNotification(payload domain.GenerateTaskPayload, manga mangadom.MangaResponse, path string) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      payload.ScriptURL,
		OutputCategory: "script-json",
		TargetTitle:    manga.Title,
		ExecutionMode:  "script-only",
	}, "N/A", path
}

func (p *MangaPipeline) buildDesignNotification(payload domain.GenerateTaskPayload, url string, seed int64) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      "N/A (Design)",
		OutputCategory: "design-sheet",
		TargetTitle:    fmt.Sprintf("Design: %s (Seed: %d)", payload.InputText, seed),
		ExecutionMode:  "design",
	}, url, "N/A"
}
