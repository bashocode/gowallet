package config

import (
	"os"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/joho/godotenv"
)

type Config struct {
	DBDSN                 string
	RedisAddr             string
	SMTPHost              string
	SMTPPort              string
	SMTPFrom              string
	GoogleClientID        string
	GoogleClientSecret    string
	GoogleRedirectURL     string
	AuthServiceURL        string
	UserServiceURL        string
	WalletServiceURL      string
	TransactionServiceURL string
	PaymentServiceURL     string
	UserGRPCAddr          string
	WalletGRPCAddr        string
	LedgerGRPCAddr        string
	LedgerServiceURL      string
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

	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		authServiceURL = "http://localhost:8081"
	}

	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://localhost:8084"
	}

	walletServiceURL := os.Getenv("WALLET_SERVICE_URL")
	if walletServiceURL == "" {
		walletServiceURL = "http://localhost:8082"
	}

	transactionServiceURL := os.Getenv("TRANSACTION_SERVICE_URL")
	if transactionServiceURL == "" {
		transactionServiceURL = "http://localhost:8086"
	}

	paymentServiceURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentServiceURL == "" {
		paymentServiceURL = "http://localhost:8083"
	}

	userGRPCAddr := os.Getenv("USER_GRPC_ADDR")
	if userGRPCAddr == "" {
		userGRPCAddr = "localhost:50052"
	}

	walletGRPCAddr := os.Getenv("WALLET_GRPC_ADDR")
	if walletGRPCAddr == "" {
		walletGRPCAddr = "localhost:50053"
	}

	ledgerGRPCAddr := os.Getenv("LEDGER_GRPC_ADDR")
	if ledgerGRPCAddr == "" {
		ledgerGRPCAddr = "localhost:50054"
	}

	ledgerServiceURL := os.Getenv("LEDGER_SERVICE_URL")
	if ledgerServiceURL == "" {
		ledgerServiceURL = "http://localhost:8085"
	}

	return &Config{
		DBDSN:                 dsn,
		RedisAddr:             redisAddr,
		SMTPHost:              smtpHost,
		SMTPPort:              smtpPort,
		SMTPFrom:              smtpFrom,
		GoogleClientID:        googleClientID,
		GoogleClientSecret:    googleClientSecret,
		GoogleRedirectURL:     googleRedirectURL,
		AuthServiceURL:        authServiceURL,
		UserServiceURL:        userServiceURL,
		WalletServiceURL:      walletServiceURL,
		TransactionServiceURL: transactionServiceURL,
		PaymentServiceURL:     paymentServiceURL,
		UserGRPCAddr:          userGRPCAddr,
		WalletGRPCAddr:        walletGRPCAddr,
		LedgerGRPCAddr:        ledgerGRPCAddr,
		LedgerServiceURL:      ledgerServiceURL,
	}
}
