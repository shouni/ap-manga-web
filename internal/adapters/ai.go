package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"google.golang.org/genai"

	"ap-manga-web/internal/config"
)

const (
	// defaultGeminiTemperature はモデル生成時の多様性を制御します。
	defaultGeminiTemperature = float32(0.8)
	// defaultInitialDelay はリトライ時の初期待ち時間です。
	defaultInitialDelay = 60 * time.Second
	// defaultVertexLocationID はVertex AI のデフォルトロケーション
	defaultVertexLocationID = "global"
	// defaultVertexTemperature は生成パラメータ
	defaultVertexTemperature = float32(0.2)
	// defaultVertexInitialDelay はリトライ遅延
	defaultVertexInitialDelay = 60 * time.Second
)

// NewGeminiAIAdapter は Google AI (Gemini API) クライアントを初期化します。
func NewGeminiAIAdapter(ctx context.Context, cfg *config.Config) (gemini.GenerativeModel, error) {
	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY が設定されていません")
	}

	clientConfig := gemini.Config{
		APIKey:       cfg.GeminiAPIKey,
		Temperature:  genai.Ptr(defaultGeminiTemperature),
		InitialDelay: defaultInitialDelay,
	}

	aiClient, err := gemini.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("Gemini API クライアントの初期化に失敗しました: %w", err)
	}

	return aiClient, nil
}

// NewVertexAIAdapter は GCP Vertex AI クライアントを初期化します。
func NewVertexAIAdapter(ctx context.Context, cfg *config.Config) (gemini.GenerativeModel, error) {
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("GCP_PROJECT_ID が設定されていません")
	}

	clientConfig := gemini.Config{
		ProjectID:    cfg.ProjectID,
		LocationID:   defaultVertexLocationID,
		Temperature:  genai.Ptr(defaultVertexTemperature),
		InitialDelay: defaultVertexInitialDelay,
	}

	aiClient, err := gemini.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("Vertex AI クライアントの初期化に失敗しました: %w", err)
	}

	return aiClient, nil
}
