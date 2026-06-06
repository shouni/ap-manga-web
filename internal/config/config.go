package config

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/shouni/go-utils/text"
)

const (
	// SignedURLExpiration 生成された漫画を確認する時間を考慮した有効期限
	SignedURLExpiration = 5 * time.Minute
	// DefaultHTTPTimeout 画像生成や Gemini API の応答を考慮したタイムアウト
	DefaultHTTPTimeout = 60 * time.Second
	DefaultStyleSuffix = "Japanese anime style, official art, cel-shaded, clean line art, high-quality manga coloring, expressive eyes, vibrant colors, cinematic lighting, masterpiece, ultra-detailed, flat shading, clear character features, no 3D effect, high resolution"
)

// Config は環境変数から読み込まれたアプリケーションの全設定を保持します。
type Config struct {
	ServiceURL          string `env:"SERVICE_URL" envDefault:"http://localhost:8080"`
	Port                string `env:"PORT" envDefault:"8080"`
	ProjectID           string `env:"GCP_PROJECT_ID" envDefault:"your-gcp-project"`
	LocationID          string `env:"GCP_LOCATION_ID" envDefault:"asia-northeast1"`
	QueueID             string `env:"CLOUD_TASKS_QUEUE_ID" envDefault:"manga-queue"`
	TaskAudienceURL     string `env:"TASK_AUDIENCE_URL"` // OIDC トークンの検証に使用する Audience URL
	ServiceAccountEmail string `env:"SERVICE_ACCOUNT_EMAIL"`
	GCSBucket           string `env:"GCS_MANGA_BUCKET" envDefault:"your-manga-archive-bucket"` // 漫画画像とHTMLを保存するバケット
	BaseOutputDir       string `env:"BASE_OUTPUT_DIR" envDefault:"output"`                     // GCS内のベースルート (例: "output")
	SignedURLExpiration time.Duration
	SlackWebhookURL     string `env:"SLACK_WEBHOOK_URL"`
	GeminiAPIKey        string `env:"GEMINI_API_KEY"`
	GeminiModel         string `env:"GEMINI_MODEL" envDefault:"gemini-3-flash-preview"`            // 台本生成用モデル
	ImageStandardModel  string `env:"IMAGE_MODEL" envDefault:"gemini-3.1-flash-image-preview"`     // 標準・高速（パネル用）
	ImageQualityModel   string `env:"IMAGE_QUALITY_MODEL" envDefault:"gemini-3-pro-image-preview"` // 高品質・高知能（ページ用）
	ShutdownTimeout     time.Duration

	// OAuth & Session Settings
	GoogleClientID     string `env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
	// SessionSecret はセッションデータのHMAC署名用シークレットキーです。
	SessionSecret string `env:"SESSION_SECRET"`
	// SessionEncryptKey はセッションデータのAES暗号化用シークレットキーです。 16, 24, 32 バイトのいずれかである必要があります。
	SessionEncryptKey string `env:"SESSION_ENCRYPT_KEY"`

	// Authz Settings
	AllowedEmails  []string `env:"ALLOWED_EMAILS"`
	AllowedDomains []string `env:"ALLOWED_DOMAINS"`

	// Generation Settings
	MaxPanelsPerPage int           `env:"MAX_PANELS_PER_PAGE" envDefault:"6"`
	MaxConcurrency   int           `env:"MAX_CONCURRENCY" envDefault:"2"`
	RateInterval     time.Duration `env:"RATE_INTERVAL_SEC" envDefault:"60s"`
	StyleSuffix      string
}

// LoadConfig は環境変数から設定を読み込み、Config 構造体を生成します。
func LoadConfig() (*Config, error) {
	cfg := &Config{
		SignedURLExpiration: SignedURLExpiration,
		ShutdownTimeout:     15 * time.Second,
		StyleSuffix:         DefaultStyleSuffix,
	}

	if err := env.ParseWithOptions(cfg, env.Options{
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeOf(time.Duration(0)): parseSecondsDuration,
			reflect.TypeOf([]string{}):       parseCommaSeparatedStrings,
		},
	}); err != nil {
		return nil, fmt.Errorf("load config from environment: %w", err)
	}

	if cfg.TaskAudienceURL == "" {
		cfg.TaskAudienceURL = cfg.ServiceURL
	}

	return cfg, nil
}

func parseSecondsDuration(value string) (interface{}, error) {
	if duration, err := time.ParseDuration(value); err == nil {
		return duration, nil
	}

	seconds, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("parse duration or seconds: %w", err)
	}

	return time.Duration(seconds) * time.Second, nil
}

func parseCommaSeparatedStrings(value string) (interface{}, error) {
	return text.ParseCommaSeparatedList(value), nil
}
