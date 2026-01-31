package config

import (
	"os"
	"path"
	"time"

	"github.com/shouni/go-utils/envutil"
	"github.com/shouni/go-utils/text"
)

const (
	// SignedURLExpiration 生成された漫画を確認する時間を考慮した有効期限
	SignedURLExpiration = 5 * time.Minute
	DefaultModel        = "gemini-3-flash-preview"
	DefaultImageModel   = "gemini-3-pro-image-preview"
	// DefaultHTTPTimeout 画像生成や Gemini API の応答を考慮したタイムアウト
	DefaultHTTPTimeout      = 60 * time.Second
	DefaultRateLimit        = 5 * time.Second
	DefaultMaxPanelsPerPage = 5
	DefaultCharactersFile   = "internal/config/characters.json"
	DefaultStyleSuffix      = "Japanese anime style, official art, cel-shaded, clean line art, high-quality manga coloring, expressive eyes, vibrant colors, cinematic lighting, masterpiece, ultra-detailed, flat shading, clear character features, no 3D effect, high resolution"
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
	SignedURLExpiration time.Duration
	SlackWebhookURL     string
	GeminiAPIKey        string
	GeminiModel         string // 台本生成用モデル
	ImageModel          string // 画像生成用モデル
	TemplateDir         string // HTMLテンプレートの格納ディレクトリ
	ShutdownTimeout     time.Duration

	// OAuth & Session Settings
	GoogleClientID     string
	GoogleClientSecret string
	// SessionSecret はセッションデータのHMAC署名用シークレットキーです。
	SessionSecret string
	// SessionEncryptKey はセッションデータのAES暗号化用シークレットキーです。 16, 24, 32 バイトのいずれかである必要があります。
	SessionEncryptKey string

	// Authz Settings
	AllowedEmails  []string
	AllowedDomains []string

	CharacterConfig string
	StyleSuffix     string
}

// LoadConfig は環境変数から設定を読み込み、Config 構造体を生成します。
func LoadConfig() *Config {
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

	return &Config{
		ServiceURL:          serviceURL,
		Port:                envutil.GetEnv("PORT", "8080"),
		ProjectID:           envutil.GetEnv("GCP_PROJECT_ID", "your-gcp-project"),
		LocationID:          envutil.GetEnv("GCP_LOCATION_ID", "asia-northeast1"),
		QueueID:             envutil.GetEnv("CLOUD_TASKS_QUEUE_ID", "manga-queue"),
		TaskAudienceURL:     envutil.GetEnv("TASK_AUDIENCE_URL", serviceURL),
		ServiceAccountEmail: envutil.GetEnv("SERVICE_ACCOUNT_EMAIL", ""),
		GCSBucket:           envutil.GetEnv("GCS_MANGA_BUCKET", "your-manga-archive-bucket"),
		BaseOutputDir:       envutil.GetEnv("BASE_OUTPUT_DIR", "output"),
		SignedURLExpiration: SignedURLExpiration,
		SlackWebhookURL:     envutil.GetEnv("SLACK_WEBHOOK_URL", ""),
		GeminiAPIKey:        envutil.GetEnv("GEMINI_API_KEY", ""),
		GeminiModel:         envutil.GetEnv("GEMINI_MODEL", DefaultModel),
		ImageModel:          envutil.GetEnv("IMAGE_MODEL", DefaultImageModel),
		TemplateDir:         templateDir,
		ShutdownTimeout:     15 * time.Second,

		// OAuth & Session
		GoogleClientID:     envutil.GetEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: envutil.GetEnv("GOOGLE_CLIENT_SECRET", ""),
		SessionSecret:      envutil.GetEnv("SESSION_SECRET", ""),
		SessionEncryptKey:  envutil.GetEnv("SESSION_ENCRYPT_KEY", ""),

		AllowedEmails:  text.ParseCommaSeparatedList(allowedEmails),
		AllowedDomains: text.ParseCommaSeparatedList(allowedDomains),

		CharacterConfig: charConfig,
		StyleSuffix:     DefaultStyleSuffix,
	}
}
