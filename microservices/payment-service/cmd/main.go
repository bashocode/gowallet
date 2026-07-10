package main

import (
	paymentHandler "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/handler"
	paymentRepository "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/repository"
	paymentService "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/service"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
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

	txConn, err := grpc.NewClient(
		cfg.TransactionGRPCAddr,
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

	// Initialize layers
	payRepo := paymentRepository.NewMySQLPaymentRepository(db)
	paySvc := paymentService.NewPaymentService(
		payRepo,
		cfg.StripeSecretKey,
		cfg.StripeWebhookSecret,
		txClient,
		cfg.BaseURL,
	)
	payHandler := paymentHandler.NewPaymentHandler(paySvc)
	webhookHandler := paymentHandler.NewWebhookHandler(cfg.RabbitMQURL, cfg.WebhookSecretKey)

	// Setup HTTP Server
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CorrelationID())

	v1 := r.Group("/api/v1")
	{
		// Public endpoints
		v1.POST("/payments/webhook", payHandler.ProcessWebhook)
		v1.GET("/payments/success", payHandler.SuccessCallback)
		v1.GET("/payments/cancel", payHandler.CancelCallback)

		// custom callback only for testing
		v1.POST("/payments/callback", webhookHandler.HandleWebhookCallback)

		// Protected endpoints (JWT required)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.POST("/payments/stripe/checkout", payHandler.CreateCheckoutSession)
		}
	}

	logger.Log.Info("Payment Service listening on port " + cfg.PaymentPort + "...")
	if err := r.Run(":" + cfg.PaymentPort); err != nil {
		logger.Fatal(nil, "Failed to run HTTP server", "error", err)
	}
}
