package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	paymentHandler "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/handler"
	paymentPublisher "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/publisher"
	paymentRepository "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/repository"
	paymentService "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/service"
	paymentWorker "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/worker"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/gin-gonic/gin"
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

	pub, err := paymentPublisher.NewRabbitMQPaymentPublisher(cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal(nil, "Failed to initialize RabbitMQ publisher", "error", err)
	}

	// Initialize layers
	payRepo := paymentRepository.NewMySQLPaymentRepository(db)
	outboxRepo := paymentRepository.NewMySQLOutboxRepository(db)
	paySvc := paymentService.NewPaymentService(
		db,
		payRepo,
		outboxRepo,
		cfg.StripeSecretKey,
		cfg.StripeWebhookSecret,
		pub,
		cfg.BaseURL,
	)
	payHandler := paymentHandler.NewPaymentHandler(paySvc)

	// Start outbox worker
	worker, err := paymentWorker.NewOutboxWorker(outboxRepo, cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal(nil, "Failed to initialize outbox worker", "error", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go worker.Start(ctx)

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

		// Protected endpoints (JWT required)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.POST("/payments/stripe/checkout", payHandler.CreateCheckoutSession)
		}
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Log.Info("Payment Service listening on port " + cfg.PaymentPort + "...")
		if err := r.Run(":" + cfg.PaymentPort); err != nil {
			logger.Fatal(nil, "Failed to run HTTP server", "error", err)
		}
	}()

	<-quit
	logger.Log.Info("Shutting down Payment Service...")
	cancel()
	worker.Stop()
	logger.Log.Info("Payment Service stopped")
}
