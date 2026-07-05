package main

import (
	"os"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	paymentHandler "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/handler"
	paymentRepository "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/repository"
	paymentService "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/service"
	pbTransaction "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("Starting Payment Microservice...")

	cfg := config.LoadConfig()

	// Connect to Redis (required by AuthMiddleware)
	rdb, err := database.ConnectRedis(cfg.RedisAddr)
	if err != nil {
		logger.Fatal(nil, "Could not connect to Redis", "error", err)
	}
	defer rdb.Close()

	// Connect to MySQL
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(nil, "Could not connect to MySQL", "error", err)
	}
	defer db.Close()

	// Connect to Transaction Service via gRPC
	transactionGRPCAddr := os.Getenv("TRANSACTION_GRPC_ADDR")
	if transactionGRPCAddr == "" {
		transactionGRPCAddr = "localhost:50055"
	}

	txConn, err := grpc.NewClient(
		transactionGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{
			"loadBalancingConfig": [{"round_robin":{}}],
			"methodConfig": [{
				"name": [{}],
				"retryPolicy": {
					"maxAttempts": 3,
					"initialBackoff": "0.1s",
					"maxBackoff": "1s",
					"backoffMultiplier": 2.0,
					"retryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED"]
				}
			}]
		}`),
	)
	if err != nil {
		logger.Fatal(nil, "Failed to connect to transaction service gRPC", "error", err)
	}
	defer txConn.Close()

	txClient := pbTransaction.NewTransactionServiceClient(txConn)

	// Read Stripe config from environment
	stripeSecretKey := os.Getenv("STRIPE_SECRET_KEY")
	stripeWebhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Initialize layers
	payRepo := paymentRepository.NewMySQLPaymentRepository(db)
	paySvc := paymentService.NewPaymentService(
		payRepo,
		stripeSecretKey,
		stripeWebhookSecret,
		txClient,
		baseURL,
	)
	payHandler := paymentHandler.NewPaymentHandler(paySvc)

	// Setup HTTP Server
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())

	v1 := r.Group("/api/v1")
	{
		// Public endpoints
		v1.POST("/payments/webhook", payHandler.ProcessWebhook)
		v1.GET("/payments/success", payHandler.SuccessCallback)
		v1.GET("/payments/cancel", payHandler.CancelCallback)

		// Protected endpoints (JWT required)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.POST("/payments/stripe/checkout", payHandler.CreateCheckoutSession)
		}
	}

	logger.Log.Info("Payment Service listening on port 8083...")
	if err := r.Run(":8083"); err != nil {
		logger.Fatal(nil, "Failed to run HTTP server", "error", err)
	}
}
