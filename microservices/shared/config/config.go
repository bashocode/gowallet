package config

import (
	"log"
	"os"
	"reflect"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv                string `mapstructure:"APP_ENV"`
	JWTSecret             string `mapstructure:"JWT_SECRET"`
	DBDSN                 string `mapstructure:"DB_DSN"`
	RedisAddr             string `mapstructure:"REDIS_ADDR"`
	RabbitMQURL           string `mapstructure:"RABBITMQ_URL"`
	MongoURL              string `mapstructure:"MONGO_URL"`
	SMTPHost              string `mapstructure:"SMTP_HOST"`
	SMTPPort              string `mapstructure:"SMTP_PORT"`
	SMTPFrom              string `mapstructure:"SMTP_FROM"`
	GoogleClientID        string `mapstructure:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret    string `mapstructure:"GOOGLE_CLIENT_SECRET"`
	GoogleRedirectURL     string `mapstructure:"GOOGLE_REDIRECT_URL"`
	AuthServiceURL        string `mapstructure:"AUTH_SERVICE_URL"`
	UserServiceURL        string `mapstructure:"USER_SERVICE_URL"`
	WalletServiceURL      string `mapstructure:"WALLET_SERVICE_URL"`
	LedgerServiceURL      string `mapstructure:"LEDGER_SERVICE_URL"`
	TransactionServiceURL string `mapstructure:"TRANSACTION_SERVICE_URL"`
	PaymentServiceURL     string `mapstructure:"PAYMENT_SERVICE_URL"`
	UserGRPCAddr          string `mapstructure:"USER_GRPC_ADDR"`
	WalletGRPCAddr        string `mapstructure:"WALLET_GRPC_ADDR"`
	LedgerGRPCAddr        string `mapstructure:"LEDGER_GRPC_ADDR"`
	TransactionGRPCAddr   string `mapstructure:"TRANSACTION_GRPC_ADDR"`
	PaymentGRPCAddr       string `mapstructure:"PAYMENT_GRPC_ADDR"`
	StripeSecretKey       string `mapstructure:"STRIPE_SECRET_KEY"`
	AuthGRPCAddr          string `mapstructure:"AUTH_GRPC_ADDR"`
	StripeWebhookSecret   string `mapstructure:"STRIPE_WEBHOOK_SECRET"`
	BaseURL               string `mapstructure:"BASE_URL"`
	MonolithBaseURL       string `mapstructure:"MONOLITH_BASE_URL"`
	TransactionBaseURL    string `mapstructure:"TRANSACTION_BASE_URL"`
	WebhookSecret         string `mapstructure:"WEBHOOK_SECRET"`
	GatewayCallbackURL    string `mapstructure:"GATEWAY_CALLBACK_URL"`
	GatewayPort           string `mapstructure:"GATEWAY_PORT"`
	AuthPort              string `mapstructure:"AUTH_PORT"`
	WalletPort            string `mapstructure:"WALLET_PORT"`
	PaymentPort           string `mapstructure:"PAYMENT_PORT"`
	UserPort              string `mapstructure:"USER_PORT"`
	LedgerPort            string `mapstructure:"LEDGER_PORT"`
	TransactionPort       string `mapstructure:"TRANSACTION_PORT"`
	MinioEndpoint         string `mapstructure:"MINIO_ENDPOINT"`
	MinioAccessKey        string `mapstructure:"MINIO_ACCESS_KEY"`
	MinioSecretKey        string `mapstructure:"MINIO_SECRET_KEY"`
	MinioPublicURL        string `mapstructure:"MINIO_PUBLIC_URL"`
	OutboxArchiveAge      string `mapstructure:"OUTBOX_ARCHIVE_AGE"`
	DBHost                string `mapstructure:"DB_HOST"`
	DBPort                string `mapstructure:"DB_PORT"`
	DBUser                string `mapstructure:"DB_USER"`
	DBPassword            string `mapstructure:"DB_PASSWORD"`
	DBName                string `mapstructure:"DB_NAME"`
	RedisHost             string `mapstructure:"REDIS_HOST"`
	RedisPort             string `mapstructure:"REDIS_PORT"`
	RabbitMQHost          string `mapstructure:"RABBITMQ_HOST"`
	RabbitMQPort          string `mapstructure:"RABBITMQ_PORT"`
	RabbitMQUser          string `mapstructure:"RABBITMQ_USER"`
	RabbitMQPassword      string `mapstructure:"RABBITMQ_PASSWORD"`
}

func LoadConfig() *Config {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	if env != "development" && env != "staging" && env != "production" {
		log.Fatalf("unsupported APP_ENV %q", env)
	}

	viper.SetConfigName(".env." + env)
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../..")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: .env.%s file not found, loading config from system environment variables", env)
	}

	bindEnvVars(Config{})
	setDefaults()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("Unable to decode config into struct: %v", err)
	}

	if cfg.DBDSN == "" && cfg.DBUser != "" {
		cfg.DBDSN = cfg.DBUser + ":" + cfg.DBPassword + "@tcp(" + cfg.DBHost + ":" + cfg.DBPort + ")/" + cfg.DBName + "?parseTime=true"
	}

	if cfg.RedisAddr == "" && cfg.RedisHost != "" {
		cfg.RedisAddr = cfg.RedisHost + ":" + cfg.RedisPort
	}

	if cfg.RabbitMQURL == "" && cfg.RabbitMQHost != "" {
		cfg.RabbitMQURL = "amqp://" + cfg.RabbitMQUser + ":" + cfg.RabbitMQPassword + "@" + cfg.RabbitMQHost + ":" + cfg.RabbitMQPort + "/"
	}

	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	if env == "production" && len(cfg.JWTSecret) < 32 {
		log.Fatal("JWT_SECRET must contain at least 32 bytes in production")
	}

	if env == "production" {
		validateProductionSecrets(&cfg)
	}

	return &cfg
}

func setDefaults() {
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("RABBITMQ_HOST", "localhost")
	viper.SetDefault("RABBITMQ_PORT", "5672")
	viper.SetDefault("RABBITMQ_USER", "guest")
	viper.SetDefault("RABBITMQ_PASSWORD", "guest")
	viper.SetDefault("MONGO_URL", "mongodb://localhost:27017")
	viper.SetDefault("SMTP_HOST", "localhost")
	viper.SetDefault("SMTP_PORT", "1025")
	viper.SetDefault("SMTP_FROM", "no-reply@gowallet.com")
	viper.SetDefault("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback")
	viper.SetDefault("AUTH_SERVICE_URL", "http://localhost:8081")
	viper.SetDefault("USER_SERVICE_URL", "http://localhost:8084")
	viper.SetDefault("WALLET_SERVICE_URL", "http://localhost:8082")
	viper.SetDefault("LEDGER_SERVICE_URL", "http://localhost:8085")
	viper.SetDefault("TRANSACTION_SERVICE_URL", "http://localhost:8086")
	viper.SetDefault("PAYMENT_SERVICE_URL", "http://localhost:8083")
	viper.SetDefault("USER_GRPC_ADDR", "localhost:50052")
	viper.SetDefault("WALLET_GRPC_ADDR", "localhost:50053")
	viper.SetDefault("LEDGER_GRPC_ADDR", "localhost:50054")
	viper.SetDefault("TRANSACTION_GRPC_ADDR", "localhost:50055")
	viper.SetDefault("PAYMENT_GRPC_ADDR", "localhost:50056")
	viper.SetDefault("AUTH_GRPC_ADDR", "localhost:50051")
	viper.SetDefault("BASE_URL", "http://localhost:8080")
	viper.SetDefault("MONOLITH_BASE_URL", "http://localhost:8080")
	viper.SetDefault("TRANSACTION_BASE_URL", "http://localhost:8086")
	viper.SetDefault("WEBHOOK_SECRET", "gowallet-webhook-secret-change-me")
	viper.SetDefault("GATEWAY_CALLBACK_URL", "http://localhost:8080")
	viper.SetDefault("GATEWAY_PORT", "8080")
	viper.SetDefault("AUTH_PORT", "8081")
	viper.SetDefault("WALLET_PORT", "8082")
	viper.SetDefault("PAYMENT_PORT", "8083")
	viper.SetDefault("USER_PORT", "8084")
	viper.SetDefault("LEDGER_PORT", "8085")
	viper.SetDefault("TRANSACTION_PORT", "8086")
	viper.SetDefault("MINIO_ENDPOINT", "localhost:9000")
	viper.SetDefault("MINIO_ACCESS_KEY", "minioadmin")
	viper.SetDefault("MINIO_SECRET_KEY", "minioadmin")
	viper.SetDefault("MINIO_PUBLIC_URL", "http://localhost:9000")
	viper.SetDefault("OUTBOX_ARCHIVE_AGE", "24h")
}

func validateProductionSecrets(cfg *Config) {
	placeholders := []string{"REPLACE_WITH", "INJECT_AT_RUNTIME", "your-", "change-me", "placeholder", "local-only"}
	secrets := map[string]string{
		"JWT_SECRET":           cfg.JWTSecret,
		"STRIPE_SECRET_KEY":    cfg.StripeSecretKey,
		"STRIPE_WEBHOOK_SECRET": cfg.StripeWebhookSecret,
		"WEBHOOK_SECRET":       cfg.WebhookSecret,
	}

	for name, value := range secrets {
		if value == "" {
			continue
		}
		for _, placeholder := range placeholders {
			if len(value) > 0 && containsIgnoreCase(value, placeholder) {
				log.Fatalf("Production secret %s contains placeholder value", name)
			}
		}
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && 
		(s[:len(substr)] == substr || containsIgnoreCase(s[1:], substr)))
}

func bindEnvVars(cfg interface{}) {
	t := reflect.TypeOf(cfg)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag != "" {
			viper.BindEnv(tag)
		}
	}
}
