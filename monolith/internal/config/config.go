package config

import (
	"os"

	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/joho/godotenv"
)

type Config struct {
	DBDSN              string
	RedisAddr          string
	SMTPHost           string
	SMTPPort           string
	SMTPFrom           string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	WebhookSecret      string
}

func LoadConfig() *Config {
	// load file .env if there is any
	if err := godotenv.Load(); err != nil {
		logger.Log.Info("Warning: .env file not found, using environment variables")
	}

	dsn := os.Getenv("DB_USER") + ":" +
		os.Getenv("DB_PASSWORD") + "@tcp(" +
		os.Getenv("DB_HOST") + ":" +
		os.Getenv("DB_PORT") + ")/" +
		os.Getenv("DB_NAME") + "?parseTime=true"

	redisAddr := os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT")

	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		smtpHost = "localhost"
	}
	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "1025"
	}
	smtpFrom := os.Getenv("SMTP_FROM")
	if smtpFrom == "" {
		smtpFrom = "no-reply@gowallet.com"
	}

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		googleClientID = ""
	}
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleClientSecret == "" {
		googleClientSecret = ""
	}
	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if googleRedirectURL == "" {
		googleRedirectURL = "http://localhost:8080/api/v1/auth/google/callback"
	}
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" {
		webhookSecret = "gowallet-webhook-secret-change-me"
	}

	return &Config{
		DBDSN:              dsn,
		RedisAddr:          redisAddr,
		SMTPHost:           smtpHost,
		SMTPPort:           smtpPort,
		SMTPFrom:           smtpFrom,
		GoogleClientID:     googleClientID,
		GoogleClientSecret: googleClientSecret,
		GoogleRedirectURL:  googleRedirectURL,
		WebhookSecret:      webhookSecret,
	}
}
