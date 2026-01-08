package runner

import (
	"fmt"
	"log"
	"time"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

// Runner は漫画生成の各ステップをオーケストレートする構造体なのだ
type Runner struct {
	cfg config.Config
	// ここに GeminiClient や GCSWriter などの依存関係を後で追加するのだ
}

// NewRunner は Runner の新しいインスタンスを生成するのだ
func NewRunner(cfg config.Config) *Runner {
	return &Runner{
		cfg: cfg,
	}
}

// Execute は具体的な漫画生成プロセスを順次実行するのだ
// worker.Handler から呼び出されるメインロジックなのだ
func (r *Runner) Execute(payload domain.GenerateTaskPayload) error {
	log.Printf("[Runner] 漫画生成を開始するのだ: URL=%s, Mode=%s", payload.ScriptURL, payload.Mode)

	// --- STEP 1: コンテンツの抽出と台本(Script)の生成 ---
	// 本来はここで Gemini API を叩いて JSON を作るのだ
	log.Printf("[Runner] STEP 1: 台本を構成中なのだ...")
	time.Sleep(2 * time.Second) // 処理のシミュレーション

	// --- STEP 2: キャラクターDNAの注入とパネル作画 ---
	// 本来はここで Nano Banana (Image Kit) を呼び出すのだ
	log.Printf("[Runner] STEP 2: パネルを描画中なのだ (上限: %d 枚)...", payload.PanelLimit)
	time.Sleep(3 * time.Second) // 処理のシミュレーション

	// --- STEP 3: 最終画像の錬成と GCS への保存 ---
	// 本来はここで画像を統合して HTML と一緒に GCS へ書き出すのだ
	outputURL := fmt.Sprintf("https://storage.googleapis.com/%s/manga/%d/index.html", r.cfg.GCSBucket, time.Now().Unix())
	log.Printf("[Runner] STEP 3: 保存完了なのだ！ 公開URL: %s", outputURL)

	// 仮実装なので、今は成功を返すのだ
	return nil
}
