package main

import (
	"os"
	"os/signal"
	"syscall"

	authPb "github.com/bashocode/gowallet/microservices/auth-service/proto/auth"
	"github.com/bashocode/gowallet/microservices/scheduler-service/internal/scheduler"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	txPb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
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
		logger.Fatal(nil, "Could not connect to Auth gRPC", "error", err)
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
		logger.Fatal(nil, "Could not connect to Wallet gRPC", "error", err)
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
		logger.Fatal(nil, "Could not connect to Transaction gRPC", "error", err)
	}
	defer txConn.Close()
	txClient := txPb.NewTransactionServiceClient(txConn)

	// 4. Initialize & Start Scheduler
	sched := scheduler.NewScheduler(authClient, walletClient, txClient)
	sched.Start()

	// Wait for shutdown signal (graceful shutdown)
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)
	<-stopChan

	sched.Stop()
}
