package main

import (
	"net"

	pbLedger "github.com/bashocode/gowallet/microservices/ledger-service/proto/ledger"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	transactionGRPC "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/grpc"
	transactionHandler "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/handler"
	transactionRepository "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/repository"
	transactionService "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/service"
	pb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
	pbUser "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("Starting Transaction Microservice...")

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

	// Connect to User Service gRPC
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
		logger.Fatal(nil, "Failed to connect to user service", "error", err)
	}
	defer userConn.Close()

	userClient := pbUser.NewUserServiceClient(userConn)

	// Connect to Wallet Service gRPC
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
		logger.Fatal(nil, "Failed to connect to wallet service", "error", err)
	}
	defer walletConn.Close()

	walletClient := pbWallet.NewWalletServiceClient(walletConn)

	// Connect to Ledger Service gRPC
	ledgerConn, err := grpc.NewClient(
		cfg.LedgerGRPCAddr,
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
		logger.Fatal(nil, "Failed to connect to ledger service", "error", err)
	}
	defer ledgerConn.Close()

	ledgerClient := pbLedger.NewLedgerServiceClient(ledgerConn)

	// Initialize layers
	txRepo := transactionRepository.NewMySQLTransactionRepository(db)
	txSvc := transactionService.NewTransactionService(db, txRepo, userClient, walletClient, ledgerClient)
	txHandler := transactionHandler.NewTransactionHandler(txSvc)

	// =========================================================
	// Start gRPC Server (for internal service-to-service calls)
	// =========================================================
	_, port, err := net.SplitHostPort(cfg.TransactionGRPCAddr)
	if err != nil {
		logger.Fatal(nil, "Failed to split host port: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+port)

	if err != nil {
		logger.Fatal(nil, "Failed to listen gRPC", "error", err)
	}

	if err != nil {
		logger.Fatal(nil, "Failed to listen on gRPC port"+port, "error", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterTransactionServiceServer(grpcServer, transactionGRPC.NewTransactionGRPCServer(txSvc))

	go func() {
		logger.Log.Info("Transaction gRPC server listening on port " + port + "...")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal(nil, "gRPC server failed", "error", err)
		}
	}()

	// =========================================================
	// Setup HTTP Server (public routes only — no topup exposed)
	// =========================================================
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())

	v1 := r.Group("/api/v1")
	{
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.POST("/transactions/transfer", txHandler.Transfer)
			protected.GET("/transactions/history", txHandler.GetHistory)
		}
	}

	logger.Log.Info("Transaction Service HTTP server listening on port " + cfg.TransactionPort + "...")
	if err := r.Run(":" + cfg.TransactionPort); err != nil {
		logger.Fatal(nil, "Failed to run HTTP server", "error", err)
	}
}
