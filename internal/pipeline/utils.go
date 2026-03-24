package pipeline

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/shouni/go-manga-kit/asset"
	"github.com/shouni/go-manga-kit/ports"
)

// resolveWorkDir は、漫画のワークディレクトリパスを解決します。
// 共通化により保守性を向上させ、パス解決のルールを一元化するのだ。
func (e *mangaExecution) resolveWorkDir(manga *ports.MangaResponse) string {
	safeTitle := e.resolveSafeTitle(manga.Title)
	return e.cfg.GetWorkDir(safeTitle)
}

// resolveOutputURL は、出力先ディレクトリのURLを取得します。
func (e *mangaExecution) resolveOutputURL(manga *ports.MangaResponse) string {
	workDir := e.resolveWorkDir(manga)
	return e.cfg.GetGCSObjectURL(workDir)
}

// resolvePlotFileURL は、指定されたプロットファイルのフルパス（GCS URLなど）を解決します。
func (e *mangaExecution) resolvePlotFileURL(manga *ports.MangaResponse) string {
	workDir := e.resolveWorkDir(manga)
	filePath := path.Join(workDir, asset.DefaultMangaPlotJson)
	// パスを GCS オブジェクト URL (例: gs://bucket/path) に変換して返します。
	return e.cfg.GetGCSObjectURL(filePath)
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

// parseTargetPanels はカンマ区切りの文字列を解析し、有効なパネルインデックスのスライスを返します。
func parseTargetPanels(s string, total int) []int {
	if strings.TrimSpace(s) == "" {
		res := make([]int, total)
		for i := 0; i < total; i++ {
			res[i] = i
		}
		return res
	}

	var res []int
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		// 数値に変換できない、または範囲外のインデックスは意図的に無視します。
		if idx, err := strconv.Atoi(trimmed); err == nil && idx >= 0 && idx < total {
			res = append(res, idx)
		}
	}
	return res
}

// parseCSV はカンマ区切りの入力文字列をスライスに変換します。
func parseCSV(input string) []string {
	var res []string
	for _, s := range strings.Split(input, ",") {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}
