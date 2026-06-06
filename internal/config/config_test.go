package config

import (
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.ServiceURL != "http://localhost:8080" {
		t.Fatalf("ServiceURL = %q, want %q", cfg.ServiceURL, "http://localhost:8080")
	}
	if cfg.TaskAudienceURL != cfg.ServiceURL {
		t.Fatalf("TaskAudienceURL = %q, want ServiceURL %q", cfg.TaskAudienceURL, cfg.ServiceURL)
	}
	if cfg.RateInterval != 60*time.Second {
		t.Fatalf("RateInterval = %s, want 60s", cfg.RateInterval)
	}
	if len(cfg.AllowedEmails) != 0 {
		t.Fatalf("AllowedEmails = %v, want empty", cfg.AllowedEmails)
	}
}

func TestLoadConfigParsesEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SERVICE_URL", "https://example.com")
	t.Setenv("TASK_AUDIENCE_URL", "https://tasks.example.com")
	t.Setenv("ALLOWED_EMAILS", "alice@example.com, bob@example.com")
	t.Setenv("ALLOWED_DOMAINS", "example.com")
	t.Setenv("MAX_CONCURRENCY", "4")
	t.Setenv("RATE_INTERVAL_SEC", "30s")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.TaskAudienceURL != "https://tasks.example.com" {
		t.Fatalf("TaskAudienceURL = %q, want %q", cfg.TaskAudienceURL, "https://tasks.example.com")
	}
	if got, want := cfg.AllowedEmails, []string{"alice@example.com", "bob@example.com"}; !equalStringSlices(got, want) {
		t.Fatalf("AllowedEmails = %v, want %v", got, want)
	}
	if got, want := cfg.AllowedDomains, []string{"example.com"}; !equalStringSlices(got, want) {
		t.Fatalf("AllowedDomains = %v, want %v", got, want)
	}
	if cfg.MaxConcurrency != 4 {
		t.Fatalf("MaxConcurrency = %d, want 4", cfg.MaxConcurrency)
	}
	if cfg.RateInterval != 30*time.Second {
		t.Fatalf("RateInterval = %s, want 30s", cfg.RateInterval)
	}
}

func TestLoadConfigParsesLegacyRateIntervalSeconds(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("RATE_INTERVAL_SEC", "30")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.RateInterval != 30*time.Second {
		t.Fatalf("RateInterval = %s, want 30s", cfg.RateInterval)
	}
}

func TestLoadConfigInvalidRateInterval(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("RATE_INTERVAL_SEC", "not-a-number")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig() error = nil, want error")
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"SERVICE_URL",
		"PORT",
		"GCP_PROJECT_ID",
		"GCP_LOCATION_ID",
		"CLOUD_TASKS_QUEUE_ID",
		"TASK_AUDIENCE_URL",
		"SERVICE_ACCOUNT_EMAIL",
		"GCS_MANGA_BUCKET",
		"BASE_OUTPUT_DIR",
		"SLACK_WEBHOOK_URL",
		"GEMINI_API_KEY",
		"GEMINI_MODEL",
		"IMAGE_MODEL",
		"IMAGE_QUALITY_MODEL",
		"GOOGLE_CLIENT_ID",
		"GOOGLE_CLIENT_SECRET",
		"SESSION_SECRET",
		"SESSION_ENCRYPT_KEY",
		"ALLOWED_EMAILS",
		"ALLOWED_DOMAINS",
		"MAX_PANELS_PER_PAGE",
		"MAX_CONCURRENCY",
		"RATE_INTERVAL_SEC",
	} {
		t.Setenv(key, "")
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
