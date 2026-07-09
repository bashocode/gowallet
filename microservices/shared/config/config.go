package config

import (
	"os"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/joho/godotenv"
)

type Config struct {
	DBDSN                 string
	RedisAddr             string
	RabbitMQURL           string
	SMTPHost              string
	SMTPPort              string
	SMTPFrom              string
	GoogleClientID        string
	GoogleClientSecret    string
	GoogleRedirectURL     string
	AuthServiceURL        string
	UserServiceURL        string
	WalletServiceURL      string
	LedgerServiceURL      string
	TransactionServiceURL string
	PaymentServiceURL     string
	UserGRPCAddr          string
	WalletGRPCAddr        string
	LedgerGRPCAddr        string
	TransactionGRPCAddr   string
	StripeSecretKey       string
	AuthGRPCAddr          string
	StripeWebhookSecret   string
	BaseURL               string
	GatewayPort           string
	AuthPort              string
	WalletPort            string
	PaymentPort           string
	UserPort              string
	LedgerPort            string
	TransactionPort       string
	WebhookSecretKey      string
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

	rabbitmqHost := os.Getenv("RABBITMQ_HOST")
	if rabbitmqHost == "" {
		rabbitmqHost = "localhost"
	}
	rabbitmqPort := os.Getenv("RABBITMQ_PORT")
	if rabbitmqPort == "" {
		rabbitmqPort = "5672"
	}
	rabbitmqUser := os.Getenv("RABBITMQ_USER")
	if rabbitmqUser == "" {
		rabbitmqUser = "guest"
	}
	rabbitmqPassword := os.Getenv("RABBITMQ_PASSWORD")
	if rabbitmqPassword == "" {
		rabbitmqPassword = "guest"
	}
	rabbitmqURL := "amqp://" + rabbitmqUser + ":" + rabbitmqPassword + "@" + rabbitmqHost + ":" + rabbitmqPort + "/"

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

	ledgerServiceURL := os.Getenv("LEDGER_SERVICE_URL")
	if ledgerServiceURL == "" {
		ledgerServiceURL = "http://localhost:8085"
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

	transactionGRPCAddr := os.Getenv("TRANSACTION_GRPC_ADDR")
	if transactionGRPCAddr == "" {
		transactionGRPCAddr = "localhost:50055"
	}

	authGRPCAddr := os.Getenv("AUTH_GRPC_ADDR")
	if authGRPCAddr == "" {
		authGRPCAddr = "localhost:50051"
	}

	stripeSecretKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeSecretKey == "" {
		stripeSecretKey = ""
	}
	stripeWebhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if stripeWebhookSecret == "" {
		stripeWebhookSecret = ""
	}
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	webhookSecretKey := os.Getenv("WEBHOOK_SECRET_KEY")
	if webhookSecretKey == "" {
		webhookSecretKey = "super-secret-key-change-this"
	}

	gatewayPort := os.Getenv("GATEWAY_PORT")
	if gatewayPort == "" {
		gatewayPort = "8080"
	}

	authPort := os.Getenv("AUTH_PORT")
	if authPort == "" {
		authPort = "8081"
	}

	walletPort := os.Getenv("WALLET_PORT")
	if walletPort == "" {
		walletPort = "8082"
	}

	paymentPort := os.Getenv("PAYMENT_PORT")
	if paymentPort == "" {
		paymentPort = "8083"
	}

	userPort := os.Getenv("USER_PORT")
	if userPort == "" {
		userPort = "8084"
	}

	ledgerPort := os.Getenv("LEDGER_PORT")
	if ledgerPort == "" {
		ledgerPort = "8085"
	}

	transactionPort := os.Getenv("TRANSACTION_PORT")
	if transactionPort == "" {
		transactionPort = "8086"
	}

	return &Config{
		DBDSN:                 dsn,
		RedisAddr:             redisAddr,
		RabbitMQURL:           rabbitmqURL,
		SMTPHost:              smtpHost,
		SMTPPort:              smtpPort,
		SMTPFrom:              smtpFrom,
		GoogleClientID:        googleClientID,
		GoogleClientSecret:    googleClientSecret,
		GoogleRedirectURL:     googleRedirectURL,
		AuthServiceURL:        authServiceURL,
		UserServiceURL:        userServiceURL,
		WalletServiceURL:      walletServiceURL,
		LedgerServiceURL:      ledgerServiceURL,
		TransactionServiceURL: transactionServiceURL,
		PaymentServiceURL:     paymentServiceURL,
		UserGRPCAddr:          userGRPCAddr,
		WalletGRPCAddr:        walletGRPCAddr,
		LedgerGRPCAddr:        ledgerGRPCAddr,
		TransactionGRPCAddr:   transactionGRPCAddr,
		AuthGRPCAddr:          authGRPCAddr,
		StripeSecretKey:       stripeSecretKey,
		StripeWebhookSecret:   stripeWebhookSecret,
		BaseURL:               baseURL,
		GatewayPort:           gatewayPort,
		AuthPort:              authPort,
		WalletPort:            walletPort,
		PaymentPort:           paymentPort,
		UserPort:              userPort,
		LedgerPort:            ledgerPort,
		TransactionPort:       transactionPort,
		WebhookSecretKey:      webhookSecretKey,
	}
}
