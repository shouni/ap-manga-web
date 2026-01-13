package pipeline

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"time"

	"ap-manga-web/internal/domain"

	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/publisher"
)

// resolveSafeTitle は実行開始時刻とタイトルから一意で安全なディレクトリ名を生成します。
func (e *mangaExecution) resolveSafeTitle(title string) string {
	if e.resolvedSafeTitle != "" {
		return e.resolvedSafeTitle
	}

	t := e.startTime
	if t.IsZero() {
		t = time.Now()
	}

	// Asia/Tokyo ロケーションでの時刻変換を行います。
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		slog.Warn("Failed to load Asia/Tokyo location, using FixedZone", "error", err)
		jst = time.FixedZone("Asia/Tokyo", 9*60*60)
	}
	tJST := t.In(jst)

	// タイトルとナノ秒バイナリを用いて衝突を防止するハッシュを生成します。
	h := md5.New()
	h.Write([]byte(title))
	nanoBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nanoBytes, uint64(t.UnixNano()))
	h.Write(nanoBytes)

	hash := fmt.Sprintf("%x", h.Sum(nil))[:8]
	e.resolvedSafeTitle = fmt.Sprintf("%s_%s", tJST.Format("20060102_150405"), hash)

	return e.resolvedSafeTitle
}

// buildMangaNotification は漫画生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildMangaNotification(
	manga mangadom.MangaResponse,
	result publisher.PublishResult,
) (*domain.NotificationRequest, string, string) {
	safeTitle := e.resolveSafeTitle(manga.Title)
	publicURL, _ := url.JoinPath(e.pipeline.appCtx.Config.ServiceURL, "outputs", safeTitle)

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
		SourceURL:      e.payload.ScriptURL,
		OutputCategory: "manga-output",
		TargetTitle:    manga.Title,
		ExecutionMode:  e.payload.Command + " / " + e.payload.Mode,
	}, publicURL, storageURI
}

// buildScriptNotification はスクリプト生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildScriptNotification(manga mangadom.MangaResponse, path string) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      e.payload.ScriptURL,
		OutputCategory: "script-json",
		TargetTitle:    manga.Title,
		ExecutionMode:  "script-only",
	}, "N/A", path
}

// buildDesignNotification はデザインシート生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildDesignNotification(url string, seed int64) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      "N/A (Design)",
		OutputCategory: "design-sheet",
		TargetTitle:    fmt.Sprintf("Design: %s (Seed: %d)", e.payload.InputText, seed),
		ExecutionMode:  "design",
	}, url, "N/A"
}
