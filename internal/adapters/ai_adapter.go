package adapters

import (
	"context"
	"fmt"
	"time"

	"ap-manga-web/internal/config"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

const (
	// defaultLocationID はデフォルトのロケーションIDです。
	defaultLocationID = "global"

	// defaultGeminiTemperature はモデル生成時の多様性を制御します。
	// 0.1 は低い値に設定することで、漫画の構成や指示への忠実度を安定させます。
	defaultGeminiTemperature = float32(0.1)

	// defaultInitialDelay リトライのデフォルトの遅延期間を指定します。
	defaultInitialDelay = 60 * time.Second
)

// NewAIAdapter は aiClientを初期化します。
func NewAIAdapter(ctx context.Context, cfg *config.Config) (gemini.GenerativeModel, error) {
	clientConfig := gemini.Config{
		Temperature:  genai.Ptr(defaultGeminiTemperature),
		InitialDelay: defaultInitialDelay,
	}

	if cfg.GeminiAPIKey != "" {
		clientConfig.APIKey = cfg.GeminiAPIKey
	} else if cfg.ProjectID != "" {
		clientConfig.ProjectID = cfg.ProjectID
		clientConfig.LocationID = defaultLocationID
	} else {
		return nil, fmt.Errorf("GEMINI_API_KEY or GCP_PROJECT_ID is not set")
	}

	aiClient, err := gemini.NewClient(ctx, clientConfig)

	if err != nil {
		return nil, fmt.Errorf("AIクライアントの初期化に失敗しました: %w", err)
	}
	return aiClient, nil
}
