package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/shouni/go-manga-kit/asset"
	"github.com/shouni/go-manga-kit/ports"
)

var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

// --- Path Resolvers ---

// saveJSON は共通の保存処理として切り出す
func (e *mangaExecution) saveJSON(ctx context.Context, path string, data any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("failed to encode to JSON: %w", err)
	}
	return e.writer.Write(ctx, path, &buf, "application/json")
}

// resolveWorkDir は、漫画のワークディレクトリパスを解決します。
func (e *mangaExecution) resolveWorkDir(manga *ports.MangaResponse) string {
	title := ""
	if manga != nil {
		title = manga.Title
	}
	safeTitle := e.resolveSafeTitle(title)
	return e.cfg.GetWorkDir(safeTitle)
}

// resolveOutputURL は、出力先ディレクトリのフルURLを取得します。
func (e *mangaExecution) resolveOutputURL(manga *ports.MangaResponse) string {
	return e.cfg.GetGCSObjectURL(e.resolveWorkDir(manga))
}

// resolvePlotFileURL は、プロットファイル（JSON）のフルパスを解決します。
func (e *mangaExecution) resolvePlotFileURL(manga *ports.MangaResponse) string {
	filePath := path.Join(e.resolveWorkDir(manga), asset.DefaultMangaPlotJson)
	return e.cfg.GetGCSObjectURL(filePath)
}

// --- Title & ID Generators ---

// resolveSafeTitle は、一意で安全な実行用ディレクトリ名を生成します。
// フォーマット: YYYYMMDD_HHMMSS_<8桁のハッシュ>
func (e *mangaExecution) resolveSafeTitle(title string) string {
	if e.resolvedSafeTitle != "" {
		return e.resolvedSafeTitle
	}

	t := e.startTime
	if t.IsZero() {
		t = time.Now()
	}
	tJST := t.In(jst)

	// ハッシュ生成: セキュリティスキャン(G401)回避のため SHA256 を使用
	seed := fmt.Sprintf("%s-%d", title, t.UnixNano())
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(seed)))[:8]

	e.resolvedSafeTitle = fmt.Sprintf("%s_%s", tJST.Format("20060102_150405"), hash)
	return e.resolvedSafeTitle
}

// --- String Parsers ---

// parseTargetPanels はカンマ区切りの文字列を解析し、範囲内のインデックスを返します。
// 入力が空、または空白のみの場合は、全インデックス (0...total-1) を返します。
func parseTargetPanels(s string, total int) []int {
	trimmedInput := strings.TrimSpace(s)
	if trimmedInput == "" {
		res := make([]int, total)
		for i := 0; i < total; i++ {
			res[i] = i
		}
		return res
	}

	parts := strings.Split(trimmedInput, ",")
	res := make([]int, 0, len(parts))
	for _, part := range parts {
		if idx, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			if idx >= 0 && idx < total {
				res = append(res, idx)
			}
		}
	}
	return res
}

// parseCSV はカンマ区切りの文字列をスライスに変換します。
func parseCSV(input string) []string {
	trimmedInput := strings.TrimSpace(input)
	if trimmedInput == "" {
		return nil
	}

	parts := strings.Split(trimmedInput, ",")
	res := make([]string, 0, len(parts))
	for _, s := range parts {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}
