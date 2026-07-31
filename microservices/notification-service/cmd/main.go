package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bashocode/gowallet/microservices/notification-service/internal/consumer"
	"github.com/bashocode/gowallet/microservices/notification-service/internal/email"
	"github.com/bashocode/gowallet/microservices/notification-service/internal/repository"
	"github.com/bashocode/gowallet/microservices/notification-service/internal/websocket"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	sharedGRPC "github.com/bashocode/gowallet/microservices/shared/grpc"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/tracing"
	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.LoadConfig()

	logger.InitLogger(
		logger.WithServiceName("notification-service"),
		logger.WithLogstashAddr(cfg.LogstashAddr),
	)
	logger.Info(context.Background(), "starting notification-service...")

	// Initialize OpenTelemetry Tracer
	tp, err := tracing.InitTracer("notification-service", cfg.OTELCollectorAddr)
	if err != nil {
		logger.Log.Warn("Failed to initialize tracer, continuing without tracing: " + err.Error())
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tp.Shutdown(shutdownCtx)
		}()
	}

	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(context.Background(), "could not connect to database", "error", err)
	}

	// Connect to Redis for WebSocket notifications
	rdb, err := database.ConnectRedis(cfg.RedisAddr)
	if err != nil {
		logger.Fatal(context.Background(), "could not connect to Redis", "error", err)
	}
	logger.Info(context.Background(), "connected to Redis successfully")

	userCreds, err := sharedGRPC.GetClientDialCredentials(
		cfg.IsProduction(),
		cfg.GRPCSSLCertPath,
		cfg.GRPCSSLKeyPath,
		cfg.GRPCSSLCAPath,
		"user-service",
	)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load gRPC client credentials for user-service", "error", err)
	}

	userConn, err := grpc.NewClient(
		cfg.UserGRPCAddr,
		grpc.WithTransportCredentials(userCreds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			sharedGRPC.UnaryClientIdentity("notification-service"),
			sharedGRPC.UnaryClientTimeout(5*time.Second),
		),
	)
	if err != nil {
		logger.Fatal(context.Background(), "could not connect to user-service gRPC", "error", err)
	}

	userClient := pb.NewUserServiceClient(userConn)
	emailSender := email.NewSMTPEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)
	notificationRepo := repository.NewNotificationRepository(db)
	// Initialize WebSocket publisher
	wsPublisher := websocket.NewPublisher(rdb, cfg.WebSocketChannel)
	logger.Info(context.Background(), "WebSocket publisher initialized", "channel", cfg.WebSocketChannel)

	// Initialize consumers with WebSocket publisher
	paymentConsumer := consumer.NewPaymentNotificationConsumer(cfg.RabbitMQURL, notificationRepo, userClient, emailSender, wsPublisher)
	transferConsumer := consumer.NewTransferNotificationConsumer(cfg.RabbitMQURL, notificationRepo, userClient, emailSender, wsPublisher)
	emailConsumer := consumer.NewEmailNotificationConsumer(cfg.RabbitMQURL, notificationRepo, emailSender)

	ctx, cancel := context.WithCancel(context.Background())

	go paymentConsumer.Start(ctx)
	go transferConsumer.Start(ctx)
	go emailConsumer.Start(ctx)

	logger.Info(context.Background(), "notification-service started successfully")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(context.Background(), "Shutdown signal received. Starting graceful shutdown...")

	cancel()

	logger.Info(context.Background(), "Closing gRPC client connections...")
	if err := userConn.Close(); err != nil {
		logger.Error(context.Background(), "Failed to close user service connection", "error", err.Error())
	}

	logger.Info(context.Background(), "Closing Redis connection...")
	if err := rdb.Close(); err != nil {
		logger.Error(context.Background(), "Failed to close Redis client", "error", err.Error())
	}

	logger.Info(context.Background(), "Closing database connection...")
	if err := db.Close(); err != nil {
		logger.Error(context.Background(), "Failed to close database connection", "error", err.Error())
	}

	logger.Info(context.Background(), "Notification Microservice successfully stopped.")
}
