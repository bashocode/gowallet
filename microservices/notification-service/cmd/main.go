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
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	sharedGRPC "github.com/bashocode/gowallet/microservices/shared/grpc"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	"google.golang.org/grpc"
)

func main() {
	logger.InitLogger()
	logger.Info(context.Background(), "starting notification-service...")

	cfg := config.LoadConfig()

	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(context.Background(), "could not connect to database", "error", err)
	}

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
	paymentConsumer := consumer.NewPaymentNotificationConsumer(cfg.RabbitMQURL, notificationRepo, userClient, emailSender)
	emailConsumer := consumer.NewEmailNotificationConsumer(cfg.RabbitMQURL, notificationRepo, emailSender)

	ctx, cancel := context.WithCancel(context.Background())

	go paymentConsumer.Start(ctx)
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

	logger.Info(context.Background(), "Closing database connection...")
	if err := db.Close(); err != nil {
		logger.Error(context.Background(), "Failed to close database connection", "error", err.Error())
	}

	logger.Info(context.Background(), "Notification Microservice successfully stopped.")
}
