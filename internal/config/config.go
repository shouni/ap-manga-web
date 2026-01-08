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
	DefaultHTTPTimeout  = 60 * time.Second // 画像生成は時間がかかるため少し長めに設定
	SignedURLExpiration = 1 * time.Hour    // 生成された漫画を確認する時間を考慮
)

// Config は環境変数からアプリケーション設定を読み込む構造体です。
type Config struct {
	ServiceURL      string
	Port            string
	ProjectID       string
	LocationID      string
	QueueID         string
	GCSBucket       string // 漫画画像とHTMLを保存するバケット
	SlackWebhookURL string
	GeminiAPIKey    string
	GeminiModel     string // 台本生成用
	ImageModel      string // 画像生成用 (Nano Banana等)
	TemplateDir     string // 指摘に基づきパスからディレクトリに変更

	// OAuth & Session Settings
	GoogleClientID     string
	GoogleClientSecret string
	SessionSecret      string

	// Authz Settings
	AllowedEmails  []string
	AllowedDomains []string
}

// LoadConfig は環境変数から設定を読み込みます。
func LoadConfig() Config {
	allowedEmails := getEnv("ALLOWED_EMAILS", "")
	allowedDomains := getEnv("ALLOWED_DOMAINS", "")

	// Cloud Run や ko でのデプロイ環境に合わせたテンプレートディレクトリの切り替え
	templateDir := "templates"
	if os.Getenv("KO_DATA_PATH") != "" || os.Getenv("K_SERVICE") != "" {
		templateDir = "/app/templates"
	}

	return Config{
		ServiceURL:      getEnv("SERVICE_URL", "http://localhost:8080"),
		Port:            getEnv("PORT", "8080"),
		ProjectID:       getEnv("GCP_PROJECT_ID", "your-gcp-project"),
		LocationID:      getEnv("GCP_LOCATION_ID", "asia-northeast1"),
		QueueID:         getEnv("CLOUD_TASKS_QUEUE_ID", "manga-queue"),
		GCSBucket:       getEnv("GCS_MANGA_BUCKET", "your-manga-archive-bucket"),
		SlackWebhookURL: getEnv("SLACK_WEBHOOK_URL", ""),
		GeminiAPIKey:    getEnv("GEMINI_API_KEY", ""),
		GeminiModel:     getEnv("GEMINI_MODEL", "gemini-3.0-flash-preview"),
		ImageModel:      getEnv("IMAGE_MODEL", "gemini-3.0-pro-image-preview"),
		TemplateDir:     templateDir,

		// OAuth
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		SessionSecret:      getEnv("SESSION_SECRET", ""),
		AllowedEmails:      text.ParseCommaSeparatedList(allowedEmails),
		AllowedDomains:     text.ParseCommaSeparatedList(allowedDomains),
	}
}

// ValidateEssentialConfig は設定バリデーションを行います。
func ValidateEssentialConfig(cfg Config) error {
	isSecure := IsSecureURL(cfg.ServiceURL)

	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" || cfg.SessionSecret == "" {
		return fmt.Errorf("認証関連の環境変数が不足しています (CLIENT_ID, SECRET, SESSION_SECRET)")
	}

	if !isSecure {
		return fmt.Errorf("セキュリティエラー: SERVICE_URL ('%s') は HTTPS である必要があります。本番環境ではセッション保護のため必須です", cfg.ServiceURL)
	}

	if len(cfg.AllowedEmails) == 0 && len(cfg.AllowedDomains) == 0 {
		return fmt.Errorf("設定エラー: 認可リスト (ALLOWED_EMAILS/DOMAINS) が空です")
	}

	if cfg.GeminiAPIKey == "" {
		return fmt.Errorf("設定エラー: GEMINI_API_KEY が設定されていません")
	}

	return nil
}

func IsSecureURL(rawURL string) bool {
	return urlpath.IsSecureServiceURL(rawURL)
}

func getEnv(key string, defaultValue string) string {
	return envutil.GetEnv(key, defaultValue)
}

func getEnvAsBool(key string, defaultValue bool) bool {
	return envutil.GetEnvAsBool(key, defaultValue)
}
