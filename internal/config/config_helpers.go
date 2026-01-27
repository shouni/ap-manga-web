package config

import (
	"fmt"
	"path"
	"strings"

	"github.com/shouni/netarmor/securenet"
)

// GetWorkDir は特定のリクエストに対する一意の作業ディレクトリを返します。
// 例: "output/20260113-ABCD"
func (c Config) GetWorkDir(requestID string) string {
	return path.Join(c.BaseOutputDir, requestID)
}

// GetImageDir は画像保存用のサブディレクトリパスを返します。
func (c Config) GetImageDir(requestID string) string {
	return path.Join(c.GetWorkDir(requestID), "images")
}

// GetGCSObjectURL は、指定されたパスから完全なGCSオブジェクトURL ("gs://...") を組み立てます。
// pathが既に "gs://" プレフィックスを持つ場合は、そのままpathを返します。
// c.GCSBucketが空文字列の場合、この関数は引数で与えられたpathをそのまま返します。
// これはローカルファイルシステムでの実行など、GCSを使用しないシナリオを想定しています。
func (c Config) GetGCSObjectURL(path string) string {
	if strings.HasPrefix(path, "gs://") {
		return path
	}
	if c.GCSBucket != "" {
		return fmt.Sprintf("gs://%s/%s", c.GCSBucket, path)
	}

	return path
}

// --- バリデーション ---

// ValidateEssentialConfig はアプリケーション実行に不可欠な設定を検証します。
func ValidateEssentialConfig(cfg *Config) error {
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

	if cfg.SessionEncryptKey == "" {
		return fmt.Errorf("SESSION_ENCRYPT_KEY が設定されていません。セキュアな運用のために必須です")
	}

	// SessionEncryptKey の長さチェック (AES要件: 16, 24, 32 bytes)
	keyLen := len([]byte(cfg.SessionEncryptKey))
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return fmt.Errorf("SESSION_ENCRYPT_KEY の長さが不正です (%d バイト)。16, 24, 32 バイトのいずれかにしてください", keyLen)
	}

	// SessionSecret の空チェック
	if cfg.SessionSecret == "" {
		return fmt.Errorf("SESSION_SECRET が設定されていません")
	}

	return nil
}

// IsSecureURL は指定された URL が HTTPS または localhost であるか判定します。
func IsSecureURL(rawURL string) bool {
	return securenet.IsSecureServiceURL(rawURL)
}
