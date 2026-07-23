package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	paymentGRPC "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/grpc"
	paymentHandler "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/handler"
	paymentPublisher "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/publisher"
	paymentRepository "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/repository"
	paymentService "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/service"
	paymentWorker "github.com/bashocode/gowallet/microservices/payment-service/internal/payment/worker"
	pb "github.com/bashocode/gowallet/microservices/payment-service/proto/payment"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("Starting Payment Microservice...")

	cfg := config.LoadConfig()

	rdb, err := database.ConnectRedis(cfg.RedisAddr)
	if err != nil {
		logger.Fatal(context.Background(), "Could not connect to Redis", "error", err)
	}

	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(context.Background(), "Could not connect to MySQL", "error", err)
	}

	pub, err := paymentPublisher.NewRabbitMQPaymentPublisher(cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to initialize RabbitMQ publisher", "error", err)
	}

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

	worker, err := paymentWorker.NewOutboxWorker(outboxRepo, cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to initialize outbox worker", "error", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go worker.Start(ctx)

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CorrelationID())

	r.GET("/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	r.GET("/ready", func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN", "reason": "MySQL database not responding"})
			return
		}
		if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN", "reason": "Redis cache not responding"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "READY"})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "HEALTHY"})
	})

	v1 := r.Group("/api/v1")
	{
		v1.POST("/payments/webhook", payHandler.ProcessWebhook)
		v1.GET("/payments/success", payHandler.SuccessCallback)
		v1.GET("/payments/cancel", payHandler.CancelCallback)

		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.POST("/payments/stripe/checkout", payHandler.CreateCheckoutSession)
		}
	}

	_, grpcPort, err := net.SplitHostPort(cfg.PaymentGRPCAddr)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to split gRPC host port", "error", err)
	}

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to listen gRPC port "+grpcPort, "error", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPaymentServiceServer(grpcServer, paymentGRPC.NewPaymentGRPCServer(outboxRepo))

	go func() {
		logger.Log.Info("Payment gRPC Server running on port " + grpcPort + "...")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal(context.Background(), "Failed to serve gRPC", "error", err)
		}
	}()

	srv := &http.Server{
		Addr:    ":" + cfg.PaymentPort,
		Handler: r,
	}

	go func() {
		logger.Log.Info("Payment Service listening on port " + cfg.PaymentPort + "...")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal(context.Background(), "Server listen failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Shutdown signal received. Starting graceful shutdown...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.Error(ctxShutdown, "HTTP Server forced to shutdown", "error", err.Error())
	} else {
		logger.Log.Info("HTTP Server closed cleanly.")
	}

	logger.Log.Info("Stopping gRPC server...")
	grpcServer.GracefulStop()
	logger.Log.Info("gRPC Server closed cleanly.")

	logger.Log.Info("Stopping background workers...")
	cancel()
	worker.Stop()

	logger.Log.Info("Closing database and cache connections...")

	if err := rdb.Close(); err != nil {
		logger.Error(ctxShutdown, "Failed to close Redis client", "error", err.Error())
	}

	if err := db.Close(); err != nil {
		logger.Error(ctxShutdown, "Failed to close MySQL connection", "error", err.Error())
	}

	logger.Log.Info("Payment Microservice successfully stopped.")
}
