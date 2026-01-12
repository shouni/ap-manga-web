package config

import (
	"fmt"
	"os"
	"time"

	"github.com/shouni/go-utils/envutil"
	"github.com/shouni/go-utils/text"
	"github.com/shouni/go-utils/urlpath"
)

const (
	// SignedURLExpiration 生成された漫画を確認する時間を考慮した有効期限
	SignedURLExpiration = 1 * time.Hour
	DefaultModel        = "gemini-3-flash-preview"
	DefaultImageModel   = "gemini-3-pro-image-preview"
	// DefaultHTTPTimeout 画像生成や Gemini API の応答を考慮したタイムアウト
	DefaultHTTPTimeout    = 30 * time.Second
	DefaultRateLimit      = 30 * time.Second
	DefaultCharactersFile = "internal/config/characters.json" // キャラクターの視覚情報（DNA）を定義したJSONパス
	DefaultStyleSuffix    = "Japanese anime style, official art, cel-shaded, clean line art, high-quality manga coloring, expressive eyes, vibrant colors, cinematic lighting, masterpiece, ultra-detailed, flat shading, clear character features, no 3D effect, high resolution"
)

// Config は環境変数から読み込まれたアプリケーションの全設定を保持します。
type Config struct {
	ServiceURL          string
	Port                string
	ProjectID           string
	LocationID          string
	QueueID             string
	TaskAudienceURL     string // OIDC トークンの検証に使用する Audience URL
	ServiceAccountEmail string
	GCSBucket           string // 漫画画像とHTMLを保存するバケット
	GCSOutputPathFormat string // 出力パスのテンプレート (例: "manga/%d/index.html")
	SlackWebhookURL     string
	GeminiAPIKey        string
	GeminiModel         string        // 台本生成用モデル
	ImageModel          string        // 画像生成用モデル
	TemplateDir         string        // HTMLテンプレートの格納ディレクトリ
	ShutdownTimeout     time.Duration // 追加

	// OAuth & Session Settings
	GoogleClientID     string
	GoogleClientSecret string
	SessionSecret      string

	// Authz Settings
	AllowedEmails  []string
	AllowedDomains []string

	CharacterConfig string
	StyleSuffix     string
}

// LoadConfig は環境変数から設定を読み込み、Config 構造体を生成します。
func LoadConfig() Config {
	serviceURL := envutil.GetEnv("SERVICE_URL", "http://localhost:8080")
	allowedEmails := envutil.GetEnv("ALLOWED_EMAILS", "")
	allowedDomains := envutil.GetEnv("ALLOWED_DOMAINS", "")

	// 実行環境（Local, Cloud Run, ko）に応じたテンプレートパスの解決
	templateDir := "templates"
	if os.Getenv("KO_DATA_PATH") != "" || os.Getenv("K_SERVICE") != "" {
		templateDir = "/app/templates"
	}

	return Config{
		ServiceURL:          serviceURL,
		Port:                envutil.GetEnv("PORT", "8080"),
		ProjectID:           envutil.GetEnv("GCP_PROJECT_ID", "your-gcp-project"),
		LocationID:          envutil.GetEnv("GCP_LOCATION_ID", "asia-northeast1"),
		QueueID:             envutil.GetEnv("CLOUD_TASKS_QUEUE_ID", "manga-queue"),
		TaskAudienceURL:     envutil.GetEnv("TASK_AUDIENCE_URL", serviceURL), // デフォルトは ServiceURL
		ServiceAccountEmail: envutil.GetEnv("SERVICE_ACCOUNT_EMAIL", ""),
		GCSBucket:           envutil.GetEnv("GCS_MANGA_BUCKET", "your-manga-archive-bucket"),
		SlackWebhookURL:     envutil.GetEnv("SLACK_WEBHOOK_URL", ""),
		GeminiAPIKey:        envutil.GetEnv("GEMINI_API_KEY", ""),
		GeminiModel:         envutil.GetEnv("GEMINI_MODEL", DefaultModel),
		ImageModel:          envutil.GetEnv("IMAGE_MODEL", DefaultImageModel),
		TemplateDir:         templateDir,
		ShutdownTimeout:     15 * time.Second,

		// OAuth
		GoogleClientID:     envutil.GetEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: envutil.GetEnv("GOOGLE_CLIENT_SECRET", ""),
		SessionSecret:      envutil.GetEnv("SESSION_SECRET", ""),
		AllowedEmails:      text.ParseCommaSeparatedList(allowedEmails),
		AllowedDomains:     text.ParseCommaSeparatedList(allowedDomains),

		CharacterConfig: DefaultCharactersFile,
		StyleSuffix:     DefaultStyleSuffix,
	}
}

// ValidateEssentialConfig はアプリケーション実行に不可欠な設定が正しく提供されているか検証します。
func ValidateEssentialConfig(cfg Config) error {
	// 本番環境（localhost 以外）では HTTPS を強制
	if !IsSecureURL(cfg.ServiceURL) {
		return fmt.Errorf("security error: SERVICE_URL ('%s') must be HTTPS in production for session protection", cfg.ServiceURL)
	}

	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" || cfg.SessionSecret == "" {
		return fmt.Errorf("configuration error: OAuth requirements (CLIENT_ID, SECRET, SESSION_SECRET) are missing")
	}

	if len(cfg.AllowedEmails) == 0 && len(cfg.AllowedDomains) == 0 {
		return fmt.Errorf("configuration error: authorization lists (ALLOWED_EMAILS/DOMAINS) are empty")
	}

	if cfg.GeminiAPIKey == "" {
		return fmt.Errorf("configuration error: GEMINI_API_KEY is not set")
	}

	// Audience URL のチェック (OIDC検証に必須)
	if cfg.TaskAudienceURL == "" {
		return fmt.Errorf("configuration error: TASK_AUDIENCE_URL is required for secure worker communication")
	}

	return nil
}

// IsSecureURL は指定された URL がセキュア（HTTPS または開発用 localhost）であるか判定します。
func IsSecureURL(rawURL string) bool {
	return urlpath.IsSecureServiceURL(rawURL)
}
