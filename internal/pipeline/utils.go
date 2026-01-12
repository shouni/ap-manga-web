package pipeline

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"ap-manga-web/internal/domain"

	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/publisher"
)

// getSafeTitle は引数から t を削除し、インスタンスの startTime を使うようにしたのだ
func (p *MangaPipeline) getSafeTitle(title string) string {
	// startTime が未設定（初期値）の場合は、その場で現在時刻を入れるバックアップなのだ
	t := p.startTime
	if t.IsZero() {
		t = time.Now()
	}

	// タイムゾーンを日本時間に変換
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	tJST := t.In(jst)

	h := md5.New()
	io.WriteString(h, title)
	// 同時実行時の衝突を防ぐため、ナノ秒もハッシュに混ぜるのがおすすめなのだ
	io.WriteString(h, fmt.Sprintf("%d", t.UnixNano()))
	hash := fmt.Sprintf("%x", h.Sum(nil))[:8]

	return fmt.Sprintf("%s_%s", tJST.Format("20060102_150405"), hash)
}

// buildMangaNotification も引数から t を削除したのだ
func (p *MangaPipeline) buildMangaNotification(
	payload domain.GenerateTaskPayload,
	manga mangadom.MangaResponse,
	result publisher.PublishResult,
) (*domain.NotificationRequest, string, string) {
	// 内部で一貫した safeTitle を生成できるのだ
	safeTitle := p.getSafeTitle(manga.Title)

	publicURL, _ := url.JoinPath(p.appCtx.Config.ServiceURL, "outputs", safeTitle)

	// PublishResult から実際のパスを取得。url.Parse を使って gs:// を守るのだ
	storageURI := "N/A"
	if u, err := url.Parse(result.MarkdownPath); err == nil {
		u.Path = path.Dir(u.Path)
		storageURI = u.String()
	}

	return &domain.NotificationRequest{
		SourceURL:      payload.ScriptURL,
		OutputCategory: "manga-output",
		TargetTitle:    manga.Title,
		ExecutionMode:  payload.Command + " / " + payload.Mode,
	}, publicURL, storageURI
}

// --- 他のユーティリティはそのまま維持するのだ ---

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
