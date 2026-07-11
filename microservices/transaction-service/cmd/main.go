package main

import (
	"context"
	"net"
	"time"

	pbLedger "github.com/bashocode/gowallet/microservices/ledger-service/proto/ledger"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/dlq"
	transactionGRPC "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/grpc"
	transactionHandler "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/handler"
	transactionRepository "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/repository"
	transactionService "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/service"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/worker"
	transferHandler "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/handler"
	transferRepository "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/repository"
	transferWorker "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/worker"
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

	// Initialize & Start Outbox Worker
	outboxWorker := worker.NewOutboxWorker(db, cfg.RabbitMQURL)

	bgCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	go outboxWorker.Start(bgCtx)

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

	dlqPublisher := dlq.NewNoOpPublisher()

	ledgerClient := pbLedger.NewLedgerServiceClient(ledgerConn)

	// Initialize layers
	txRepo := transactionRepository.NewMySQLTransactionRepository(db)
	outboundTransferRepo := transferRepository.NewMySQLOutboundTransferRepository(db)
	transferOutboxRepo := transferRepository.NewMySQLTransferOutboxRepository(db)
	txSvc := transactionService.NewTransactionService(db, txRepo, outboundTransferRepo, transferOutboxRepo, userClient, walletClient, ledgerClient, dlqPublisher, cfg.MonolithBaseURL, cfg.WebhookSecret)
	txHandler := transactionHandler.NewTransactionHandler(txSvc)
	transferH := transferHandler.NewTransferHandler(txSvc, cfg.WebhookSecret)

	// Start the transfer outbox publisher worker (publishes transfer.* events to transfer.events).
	transferOutboxWorker := transferWorker.NewTransferOutboxWorker(db, cfg.RabbitMQURL, transferOutboxRepo)
	go transferOutboxWorker.Start(bgCtx)

	// Start the transfer consumer worker (consumes transfer.initiated from queue,
	// async validates receiver + notifies monolith).
	transferConsumerWorker := transferWorker.NewTransferConsumerWorker(cfg.RabbitMQURL, txSvc)
	go transferConsumerWorker.Start(bgCtx)

	// Start reconciliation worker: checks for stale pending transfers every 2 minutes.
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				if err := txSvc.ReconcilePendingTransfers(bgCtx); err != nil {
					logger.Log.Error("Transfer reconciliation failed", "error", err)
				}
			}
		}
	}()

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
	r.Use(middleware.CorrelationID())

	v1 := r.Group("/api/v1")
	{
		// Public webhook endpoint (called by monolith, no JWT)
		v1.POST("/transfers/webhook", transferH.ProcessTransferWebhook)

		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.POST("/transactions/transfer", txHandler.Transfer)
			protected.GET("/transactions/history", txHandler.GetHistory)

			// Episode 35: external transfer to monolith ewallet
			protected.POST("/transfers/external", transferH.CreateExternalTransfer)
			protected.GET("/transfers/external/:id", transferH.GetExternalTransfer)
		}
	}

	logger.Log.Info("Transaction Service HTTP server listening on port " + cfg.TransactionPort + "...")
	if err := r.Run(":" + cfg.TransactionPort); err != nil {
		logger.Fatal(nil, "Failed to run HTTP server", "error", err)
	}
}
