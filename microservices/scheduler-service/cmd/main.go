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
	sharedGRPC "github.com/bashocode/gowallet/microservices/shared/grpc"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/storage"
	"github.com/bashocode/gowallet/microservices/shared/tracing"
	txPb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
	userPb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	walletPb "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("Starting Centralized Scheduler Service...")

	cfg := config.LoadConfig()

	// Initialize OpenTelemetry Tracer
	tp, err := tracing.InitTracer("scheduler-service", cfg.OTELCollectorAddr)
	if err != nil {
		logger.Log.Warn("Failed to initialize tracer, continuing without tracing: " + err.Error())
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tp.Shutdown(shutdownCtx)
		}()
	}

	authCreds, err := sharedGRPC.GetClientDialCredentials(
		cfg.IsProduction(),
		cfg.GRPCSSLCertPath,
		cfg.GRPCSSLKeyPath,
		cfg.GRPCSSLCAPath,
		"auth-service")
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load gRPC client credentials for auth-service", "error", err)
	}

	// 1. gRPC connection to Auth Service
	authConn, err := grpc.NewClient(
		cfg.AuthGRPCAddr,
		grpc.WithTransportCredentials(authCreds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			sharedGRPC.UnaryClientIdentity("scheduler-service"),
			sharedGRPC.UnaryClientTimeout(5*time.Second),
		),
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

	// 2. gRPC connection to Wallet Service
	walletCreds, err := sharedGRPC.GetClientDialCredentials(
		cfg.IsProduction(),
		cfg.GRPCSSLCertPath,
		cfg.GRPCSSLKeyPath,
		cfg.GRPCSSLCAPath,
		"wallet-service",
	)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load gRPC client credentials for wallet-service", "error", err)
	}

	walletConn, err := grpc.NewClient(
		cfg.WalletGRPCAddr,
		grpc.WithTransportCredentials(walletCreds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			sharedGRPC.UnaryClientIdentity("scheduler-service"),
			sharedGRPC.UnaryClientTimeout(5*time.Second),
		),
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

	// 3. gRPC connection to Transaction Service
	txCreds, err := sharedGRPC.GetClientDialCredentials(
		cfg.IsProduction(),
		cfg.GRPCSSLCertPath,
		cfg.GRPCSSLKeyPath,
		cfg.GRPCSSLCAPath,
		"transaction-service",
	)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load gRPC client credentials for transaction-service", "error", err)
	}
	txConn, err := grpc.NewClient(
		cfg.TransactionGRPCAddr,
		grpc.WithTransportCredentials(txCreds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			sharedGRPC.UnaryClientIdentity("scheduler-service"),
			sharedGRPC.UnaryClientTimeout(5*time.Second),
		),
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

	// 4. gRPC connection to User Service
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
			sharedGRPC.UnaryClientIdentity("scheduler-service"),
			sharedGRPC.UnaryClientTimeout(5*time.Second),
		),
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

	// 5. gRPC connection to Payment Service
	payCreds, err := sharedGRPC.GetClientDialCredentials(
		cfg.IsProduction(),
		cfg.GRPCSSLCertPath,
		cfg.GRPCSSLKeyPath,
		cfg.GRPCSSLCAPath,
		"payment-service",
	)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load gRPC client credentials for payment-service", "error", err)
	}
	payConn, err := grpc.NewClient(
		cfg.PaymentGRPCAddr,
		grpc.WithTransportCredentials(payCreds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			sharedGRPC.UnaryClientIdentity("scheduler-service"),
			sharedGRPC.UnaryClientTimeout(5*time.Second),
		),
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

	// 6. Initialize & Start Scheduler
	sched := scheduler.NewScheduler(authClient, walletClient, txClient, userClient)
	sched.Start()

	// 7. Initialize MinIO for Outbox Archiver
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

	// 8. Start Outbox Archiver Workers
	bgCtx, cancelArchiver := context.WithCancel(context.Background())

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
