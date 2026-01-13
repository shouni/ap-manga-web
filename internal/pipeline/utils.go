package pipeline

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"ap-manga-web/internal/domain"

	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/publisher"
)

const (
	publicURLConstructionError = "N/A (URL construction failed)"
	NotAvailable               = "N/A"
)

// resolveSafeTitle は、衝突を避けるための一意で安全なタイトル文字列を生成します。
// 生成された文字列は、公開URLのパスパラメータやGCSのディレクトリ名として使用されることを想定しています。
// フォーマット: YYYYMMDD_HHMMSS_<8桁のハッシュ>
func (e *mangaExecution) resolveSafeTitle(title string) string {
	if e.resolvedSafeTitle != "" {
		return e.resolvedSafeTitle
	}

	t := e.startTime
	if t.IsZero() {
		t = time.Now()
	}

	// Asia/Tokyo ロケーションでの時刻変換
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		slog.Warn("Failed to load Asia/Tokyo location, using FixedZone", "error", err)
		jst = time.FixedZone("Asia/Tokyo", 9*60*60)
	}
	tJST := t.In(jst)

	// ハッシュ生成（タイトル + ナノ秒）
	h := md5.New()
	h.Write([]byte(title))
	nanoBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nanoBytes, uint64(t.UnixNano()))
	h.Write(nanoBytes)

	hash := fmt.Sprintf("%x", h.Sum(nil))[:8]
	// この戻り値が ServeOutput の {title} パラメータ、および GCS のディレクトリ名になります。
	e.resolvedSafeTitle = fmt.Sprintf("%s_%s", tJST.Format("20060102_150405"), hash)

	return e.resolvedSafeTitle
}

// buildMangaNotification は漫画生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildMangaNotification(
	manga mangadom.MangaResponse,
	result publisher.PublishResult,
) (*domain.NotificationRequest, string, string) {
	safeTitle := e.resolveSafeTitle(manga.Title)

	// Web上の公開URLを構築
	publicURL, err := url.JoinPath(
		e.pipeline.appCtx.Config.ServiceURL,
		e.pipeline.appCtx.Config.BaseOutputDir,
		safeTitle,
	)
	if err != nil {
		slog.Error("Failed to construct public URL", "error", err, "serviceURL", e.pipeline.appCtx.Config.ServiceURL)
		publicURL = publicURLConstructionError
	}

	// GCS上の絶対パスを取得
	workDir := e.pipeline.appCtx.Config.GetWorkDir(safeTitle)
	storageURI := e.pipeline.appCtx.Config.GetGCSObjectURL(workDir)

	return &domain.NotificationRequest{
		SourceURL:      e.payload.ScriptURL,
		OutputCategory: "manga-output",
		TargetTitle:    manga.Title,
		ExecutionMode:  e.payload.Command + " / " + e.payload.Mode,
	}, publicURL, storageURI
}

// buildScriptNotification はスクリプト生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildScriptNotification(manga mangadom.MangaResponse, gcsPath string) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      e.payload.ScriptURL,
		OutputCategory: "script-json",
		TargetTitle:    manga.Title,
		ExecutionMode:  "script-only",
	}, NotAvailable, gcsPath
}

// buildDesignNotification はデザインシート生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildDesignNotification(outputStorageURI string, seed int64) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      "N/A (Design)",
		OutputCategory: "design-sheet",
		TargetTitle:    fmt.Sprintf("Design: %s (Seed: %d)", e.payload.InputText, seed),
		ExecutionMode:  "design",
	}, NotAvailable, outputStorageURI
}
