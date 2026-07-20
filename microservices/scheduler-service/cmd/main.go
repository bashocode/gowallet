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

	// 1. gRPC connection to Auth Service
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
	defer authConn.Close()
	authClient := authPb.NewAuthServiceClient(authConn)

	// 2. gRPC connection to Wallet Service
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
	defer walletConn.Close()
	walletClient := walletPb.NewWalletServiceClient(walletConn)

	// 3. gRPC connection to Transaction Service
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
	defer txConn.Close()
	txClient := txPb.NewTransactionServiceClient(txConn)

	// 4. gRPC connection to User Service
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
	defer userConn.Close()
	userClient := userPb.NewUserServiceClient(userConn)

	// gRPC connection to Payment Service
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
	defer payConn.Close()
	paymentClient := paymentPb.NewPaymentServiceClient(payConn)

	// 5. Initialize & Start Scheduler
	sched := scheduler.NewScheduler(authClient, walletClient, txClient, userClient)
	sched.Start()

	// 6. Initialize MinIO for Outbox Archiver
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

	// 7. Start Outbox Archiver Workers
	bgCtx, cancelArchiver := context.WithCancel(context.Background())
	defer cancelArchiver()

	txArchiver := archiver.NewOutboxArchiver("transaction", "outbox-archives", &archiver.TransactionOutboxAdapter{Client: txClient}, minioStorage, archiveAge, 1*time.Hour)
	userArchiver := archiver.NewOutboxArchiver("user", "outbox-archives", &archiver.UserOutboxAdapter{Client: userClient}, minioStorage, archiveAge, 1*time.Hour)
	payArchiver := archiver.NewOutboxArchiver("payment", "outbox-archives", &archiver.PaymentOutboxAdapter{Client: paymentClient}, minioStorage, archiveAge, 1*time.Hour)

	go txArchiver.Start(bgCtx)
	go userArchiver.Start(bgCtx)
	go payArchiver.Start(bgCtx)

	// Wait for shutdown signal (graceful shutdown)
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)
	<-stopChan

	sched.Stop()
}
