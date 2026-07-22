package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bashocode/gowallet/microservices/notification-service/internal/consumer"
	"github.com/bashocode/gowallet/microservices/notification-service/internal/email"
	"github.com/bashocode/gowallet/microservices/notification-service/internal/repository"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger.InitLogger()
	logger.Info(context.Background(), "starting notification-service...")

	cfg := config.LoadConfig()

	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(context.Background(), "could not connect to database", "error", err)
	}
	defer db.Close()

	userConn, err := grpc.NewClient(cfg.UserGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal(context.Background(), "could not connect to user-service gRPC", "error", err)
	}
	defer userConn.Close()
	userClient := pb.NewUserServiceClient(userConn)

	emailSender := email.NewSMTPEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)

	notificationRepo := repository.NewNotificationRepository(db)
	paymentConsumer := consumer.NewPaymentNotificationConsumer(cfg.RabbitMQURL, notificationRepo, userClient, emailSender)
	emailConsumer := consumer.NewEmailNotificationConsumer(cfg.RabbitMQURL, notificationRepo, emailSender)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go paymentConsumer.Start(ctx)
	go emailConsumer.Start(ctx)

	logger.Info(context.Background(), "notification-service started successfully")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(context.Background(), "shutting down notification-service...")
}
