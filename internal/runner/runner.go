package runner

import (
	"fmt"
	"log/slog" // log を log/slog に変更
	"time"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

// Runner は漫画生成の各ステップをオーケストレートする構造体なのだ
type Runner struct {
	cfg config.Config
	// 今後ここに adapters.GeminiClient や adapters.GCSAdapter などを追加していくのだ
}

// NewRunner は Runner の新しいインスタンスを生成するのだ
func NewRunner(cfg config.Config) *Runner {
	return &Runner{
		cfg: cfg,
	}
}

// Execute は具体的な漫画生成プロセスを順次実行するのだ
func (r *Runner) Execute(payload domain.GenerateTaskPayload) error {
	// 処理開始の構造化ログ
	slog.Info("[Runner] Starting manga generation pipeline",
		"script_url", payload.ScriptURL,
		"mode", payload.Mode,
	)

	// --- STEP 1: コンテンツの抽出と台本(Script)の生成 ---
	slog.Info("[Runner] STEP 1: Generating script via Gemini API...", "url", payload.ScriptURL)
	// 本来はここで Gemini API を叩いて JSON を作るのだ
	time.Sleep(2 * time.Second) // 処理のシミュレーション

	// --- STEP 2: キャラクターDNAの注入とパネル作画 ---
	slog.Info("[Runner] STEP 2: Drawing manga panels (Nano Banana)",
		"panel_limit", payload.PanelLimit,
	)
	// 本来はここで Nano Banana (Image Kit) を呼び出すのだ
	time.Sleep(3 * time.Second) // 処理のシミュレーション

	// --- STEP 3: 最終画像の錬成と GCS への保存 ---
	timestamp := time.Now().Unix()
	outputURL := fmt.Sprintf("https://storage.googleapis.com/%s/manga/%d/index.html", r.cfg.GCSBucket, timestamp)

	slog.Info("[Runner] STEP 3: Saving results to GCS",
		"bucket", r.cfg.GCSBucket,
		"output_url", outputURL,
	)
	// 本来はここで画像を統合して HTML と一緒に GCS へ書き出すのだ

	slog.Info("[Runner] Generation pipeline completed successfully",
		"script_url", payload.ScriptURL,
		"final_url", outputURL,
	)

	return nil
}
