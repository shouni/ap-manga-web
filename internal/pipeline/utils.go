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

	"github.com/shouni/go-manga-kit/pkg/asset"
	mangadom "github.com/shouni/go-manga-kit/pkg/domain"
	"github.com/shouni/go-manga-kit/pkg/publisher"
)

// resolveWorkDir は、漫画のワークディレクトリパスを解決します。
// 共通化により保守性を向上させ、パス解決のルールを一元化するのだ。
func (e *mangaExecution) resolveWorkDir(manga *mangadom.MangaResponse) string {
	safeTitle := e.resolveSafeTitle(manga.Title)
	return e.pipeline.appCtx.Config.GetWorkDir(safeTitle)
}

// resolvePlotFileURL は、指定されたプロットファイルのフルパス（GCS URLなど）を解決します。
func (e *mangaExecution) resolvePlotFileURL(manga *mangadom.MangaResponse) string {
	workDir := e.resolveWorkDir(manga)
	filePath := path.Join(workDir, asset.DefaultMangaPlotJson)
	// パスを GCS オブジェクト URL (例: gs://bucket/path) に変換して返します。
	return e.pipeline.appCtx.Config.GetGCSObjectURL(filePath)
}

// resolveOutputURL は、出力先ディレクトリのURLを取得します。
func (e *mangaExecution) resolveOutputURL(manga *mangadom.MangaResponse) string {
	workDir := e.resolveWorkDir(manga)
	return e.pipeline.appCtx.Config.GetGCSObjectURL(workDir)
}

// resolveSafeTitle は、衝突を避けるための一意で安全なタイトル文字列を生成します。
// フォーマット: YYYYMMDD_HHMMSS_<8桁のハッシュ>
func (e *mangaExecution) resolveSafeTitle(title string) string {
	if e.resolvedSafeTitle != "" {
		return e.resolvedSafeTitle
	}

	t := e.startTime
	if t.IsZero() {
		t = time.Now()
	}

	// Asia/Tokyo ロケーションでの時刻変換（環境に依存せず一定のフォーマットを保つ）
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		slog.Warn("Failed to load Asia/Tokyo location, using FixedZone", "error", err)
		jst = time.FixedZone("Asia/Tokyo", 9*60*60)
	}
	tJST := t.In(jst)

	// ハッシュ生成（タイトル + ナノ秒で一意性を担保）
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
	manga *mangadom.MangaResponse,
	result publisher.PublishResult,
) (*domain.NotificationRequest, string, string) {
	safeTitle := e.resolveSafeTitle(manga.Title)
	publicURL, err := url.JoinPath(
		e.pipeline.appCtx.Config.ServiceURL,
		e.pipeline.appCtx.Config.BaseOutputDir,
		safeTitle,
	)
	if err != nil {
		slog.Error("Failed to construct public URL", "error", err)
		publicURL = domain.PublicURLConstructionError
	}

	workDir := e.resolveWorkDir(manga)
	storageURI := e.pipeline.appCtx.Config.GetGCSObjectURL(workDir)

	return &domain.NotificationRequest{
		SourceURL:      e.payload.ScriptURL,
		OutputCategory: "manga-output",
		TargetTitle:    manga.Title,
		ExecutionMode:  e.payload.Command + " / " + e.payload.Mode,
	}, publicURL, storageURI
}

// buildScriptNotification はスクリプト生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildScriptNotification(manga *mangadom.MangaResponse, gcsPath string) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      e.payload.ScriptURL,
		OutputCategory: "script-json",
		TargetTitle:    manga.Title,
		ExecutionMode:  "script-only",
	}, domain.NotAvailable, gcsPath
}

// buildDesignNotification はデザインシート生成の結果に基づいてSlack通知用リクエストを構築します。
func (e *mangaExecution) buildDesignNotification(outputStorageURI string, seed int64) (*domain.NotificationRequest, string, string) {
	return &domain.NotificationRequest{
		SourceURL:      "N/A (Design)",
		OutputCategory: "design-sheet",
		TargetTitle:    fmt.Sprintf("Design: %s (Seed: %d)", e.payload.InputText, seed),
		ExecutionMode:  "design",
	}, domain.NotAvailable, outputStorageURI
}
