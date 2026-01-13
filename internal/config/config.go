package config

import (
	"fmt"
	"os"
	"path"
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
	DefaultHTTPTimeout    = 60 * time.Second
	DefaultRateLimit      = 30 * time.Second
	DefaultCharactersFile = "internal/config/characters.json"
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
	BaseOutputDir       string // GCS内のベースルート (例: "output")
	SlackWebhookURL     string
	GeminiAPIKey        string
	GeminiModel         string // 台本生成用モデル
	ImageModel          string // 画像生成用モデル
	TemplateDir         string // HTMLテンプレートの格納ディレクトリ
	ShutdownTimeout     time.Duration

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

	// 実行環境（Cloud Run, ko）に応じたパスの解決
	baseDir := "."
	if os.Getenv("KO_DATA_PATH") != "" || os.Getenv("K_SERVICE") != "" {
		baseDir = "/app"
	}

	templateDir := path.Join(baseDir, "templates")
	charConfig := path.Join(baseDir, DefaultCharactersFile)

	return Config{
		ServiceURL:          serviceURL,
		Port:                envutil.GetEnv("PORT", "8080"),
		ProjectID:           envutil.GetEnv("GCP_PROJECT_ID", "your-gcp-project"),
		LocationID:          envutil.GetEnv("GCP_LOCATION_ID", "asia-northeast1"),
		QueueID:             envutil.GetEnv("CLOUD_TASKS_QUEUE_ID", "manga-queue"),
		TaskAudienceURL:     envutil.GetEnv("TASK_AUDIENCE_URL", serviceURL),
		ServiceAccountEmail: envutil.GetEnv("SERVICE_ACCOUNT_EMAIL", ""),
		GCSBucket:           envutil.GetEnv("GCS_MANGA_BUCKET", "your-manga-archive-bucket"),
		BaseOutputDir:       envutil.GetEnv("BASE_OUTPUT_DIR", "output"),
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

		CharacterConfig: charConfig,
		StyleSuffix:     DefaultStyleSuffix,
	}
}

// --- パス管理用ヘルパーメソッド ---

// GetWorkDir は特定のリクエストに対する一意の作業ディレクトリを返します。
// 例: "output/20260113-ABCD"
func (c Config) GetWorkDir(requestID string) string {
	return fmt.Sprintf("%s/%s", c.BaseOutputDir, requestID)
}

// GetImageDir は画像保存用のサブディレクトリパスを返します。
func (c Config) GetImageDir(requestID string) string {
	return fmt.Sprintf("%s/images", c.GetWorkDir(requestID))
}

// GetGCSObjectURL はGCS内のオブジェクトパスを組み立てます。
func (c Config) GetGCSObjectURL(path string) string {
	return fmt.Sprintf("gs://%s/%s", c.GCSBucket, path)
}

// --- バリデーション ---

// ValidateEssentialConfig はアプリケーション実行に不可欠な設定を検証します。
func ValidateEssentialConfig(cfg Config) error {
	if !IsSecureURL(cfg.ServiceURL) {
		return fmt.Errorf("security error: SERVICE_URL ('%s') must be HTTPS in production", cfg.ServiceURL)
	}

	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" || cfg.SessionSecret == "" {
		return fmt.Errorf("configuration error: OAuth settings are missing")
	}

	if len(cfg.AllowedEmails) == 0 && len(cfg.AllowedDomains) == 0 {
		return fmt.Errorf("configuration error: authorization lists are empty")
	}

	if cfg.GeminiAPIKey == "" {
		return fmt.Errorf("configuration error: GEMINI_API_KEY is not set")
	}

	return nil
}

// IsSecureURL は指定された URL が HTTPS または localhost であるか判定します。
func IsSecureURL(rawURL string) bool {
	return urlpath.IsSecureServiceURL(rawURL)
}
