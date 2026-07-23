package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	authPb "github.com/bashocode/gowallet/microservices/auth-service/proto/auth"
	paymentPb "github.com/bashocode/gowallet/microservices/payment-service/proto/payment"
	"github.com/bashocode/gowallet/microservices/scheduler-service/internal/archiver"
	"github.com/bashocode/gowallet/microservices/scheduler-service/internal/scheduler"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/storage"
	txPb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
	userPb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	walletPb "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("Starting Centralized Scheduler Service...")

	cfg := config.LoadConfig()

	authConn, err := grpc.NewClient(
		cfg.AuthGRPCAddr,
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
		logger.Fatal(context.Background(), "Could not connect to Auth gRPC", "error", err)
	}
	authClient := authPb.NewAuthServiceClient(authConn)

	walletConn, err := grpc.NewClient(
		cfg.WalletGRPCAddr,
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
		logger.Fatal(context.Background(), "Could not connect to Wallet gRPC", "error", err)
	}
	walletClient := walletPb.NewWalletServiceClient(walletConn)

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
		logger.Fatal(context.Background(), "Could not connect to Transaction gRPC", "error", err)
	}
	txClient := txPb.NewTransactionServiceClient(txConn)

	userConn, err := grpc.NewClient(
		cfg.UserGRPCAddr,
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
		logger.Fatal(context.Background(), "Could not connect to User gRPC", "error", err)
	}
	userClient := userPb.NewUserServiceClient(userConn)

	payConn, err := grpc.NewClient(
		cfg.PaymentGRPCAddr,
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
		logger.Fatal(context.Background(), "Could not connect to Payment gRPC", "error", err)
	}
	paymentClient := paymentPb.NewPaymentServiceClient(payConn)

	sched := scheduler.NewScheduler(authClient, walletClient, txClient, userClient)
	sched.Start()

	minioStorage, err := storage.NewMinioStorage(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioPublicURL, false)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to initialize MinIO storage", "error", err)
	}

	if err := minioStorage.EnsureBucket(context.Background(), "outbox-archives"); err != nil {
		logger.Fatal(context.Background(), "Failed to ensure outbox-archives bucket exists", "error", err)
	}

	archiveAge, err := time.ParseDuration(cfg.OutboxArchiveAge)
	if err != nil {
		logger.Fatal(context.Background(), "Invalid OUTBOX_ARCHIVE_AGE format", "error", err)
	}

	bgCtx, cancelArchiver := context.WithCancel(context.Background())

	txArchiver := archiver.NewOutboxArchiver("transaction", "outbox-archives", &archiver.TransactionOutboxAdapter{Client: txClient}, minioStorage, archiveAge, 1*time.Hour)
	userArchiver := archiver.NewOutboxArchiver("user", "outbox-archives", &archiver.UserOutboxAdapter{Client: userClient}, minioStorage, archiveAge, 1*time.Hour)
	payArchiver := archiver.NewOutboxArchiver("payment", "outbox-archives", &archiver.PaymentOutboxAdapter{Client: paymentClient}, minioStorage, archiveAge, 1*time.Hour)

	go txArchiver.Start(bgCtx)
	go userArchiver.Start(bgCtx)
	go payArchiver.Start(bgCtx)

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)
	<-stopChan

	logger.Log.Info("Shutdown signal received. Starting graceful shutdown...")

	logger.Log.Info("Stopping scheduler...")
	sched.Stop()

	logger.Log.Info("Stopping archiver workers...")
	cancelArchiver()

	logger.Log.Info("Closing gRPC client connections...")
	if err := authConn.Close(); err != nil {
		logger.Error(context.Background(), "Failed to close auth service connection", "error", err.Error())
	}
	if err := walletConn.Close(); err != nil {
		logger.Error(context.Background(), "Failed to close wallet service connection", "error", err.Error())
	}
	if err := txConn.Close(); err != nil {
		logger.Error(context.Background(), "Failed to close transaction service connection", "error", err.Error())
	}
	if err := userConn.Close(); err != nil {
		logger.Error(context.Background(), "Failed to close user service connection", "error", err.Error())
	}
	if err := payConn.Close(); err != nil {
		logger.Error(context.Background(), "Failed to close payment service connection", "error", err.Error())
	}

	logger.Log.Info("Scheduler Service successfully stopped.")
}
