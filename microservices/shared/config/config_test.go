package config

import (
	"os"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func TestLoadConfig(t *testing.T) {
	// Initialize logger
	logger.InitLogger()

	// Set environment variables
	os.Setenv("DB_USER", "testuser")
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", "3306")
	os.Setenv("DB_NAME", "testdb")
	os.Setenv("REDIS_HOST", "127.0.0.1")
	os.Setenv("REDIS_PORT", "6379")
	os.Setenv("SMTP_HOST", "smtp.test.com")
	os.Setenv("SMTP_PORT", "587")
	os.Setenv("SMTP_FROM", "test@test.com")
	os.Setenv("GOOGLE_CLIENT_ID", "gclientid")
	os.Setenv("GOOGLE_CLIENT_SECRET", "gsecret")
	os.Setenv("GOOGLE_REDIRECT_URL", "http://callback")

	cfg := LoadConfig()

	expectedDSN := "testuser:testpass@tcp(localhost:3306)/testdb?parseTime=true"
	if cfg.DBDSN != expectedDSN {
		t.Errorf("expected DBDSN to be %q, got %q", expectedDSN, cfg.DBDSN)
	}

	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Errorf("expected RedisAddr to be %q, got %q", "127.0.0.1:6379", cfg.RedisAddr)
	}

	if cfg.SMTPHost != "smtp.test.com" || cfg.SMTPPort != "587" || cfg.SMTPFrom != "test@test.com" {
		t.Errorf("incorrect SMTP settings: %+v", cfg)
	}

	if cfg.GoogleClientID != "gclientid" || cfg.GoogleClientSecret != "gsecret" || cfg.GoogleRedirectURL != "http://callback" {
		t.Errorf("incorrect Google settings: %+v", cfg)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	logger.InitLogger()
	// Clear env vars that have defaults
	os.Unsetenv("SMTP_HOST")
	os.Unsetenv("SMTP_PORT")
	os.Unsetenv("SMTP_FROM")
	os.Unsetenv("GOOGLE_REDIRECT_URL")

	cfg := LoadConfig()

	if cfg.SMTPHost != "127.0.0.1" {
		t.Errorf("expected default SMTPHost to be 'localhost', got %q", cfg.SMTPHost)
	}
	if cfg.SMTPPort != "1025" {
		t.Errorf("expected default SMTPPort to be '1025', got %q", cfg.SMTPPort)
	}
	if cfg.SMTPFrom != "no-reply@gowallet.com" {
		t.Errorf("expected default SMTPFrom to be 'no-reply@gowallet.com', got %q", cfg.SMTPFrom)
	}
	if cfg.GoogleRedirectURL != "http://localhost:8080/api/v1/auth/google/callback" {
		t.Errorf("expected default GoogleRedirectURL, got %q", cfg.GoogleRedirectURL)
	}
}
