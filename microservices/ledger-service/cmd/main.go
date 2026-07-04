package main

import (
	"net"

	ledgerGRPC "github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/grpc"
	ledgerHandler "github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/handler"
	ledgerRepository "github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/repository"
	ledgerService "github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/service"
	pb "github.com/bashocode/gowallet/microservices/ledger-service/proto/ledger"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("Starting Ledger Microservice...")

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

	// Connect to wallet-service gRPC
	conn, err := grpc.NewClient(
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
	defer conn.Close()

	walletClient := pbWallet.NewWalletServiceClient(conn)

	// Initialize layers
	lRepo := ledgerRepository.NewMySQLLedgerRepository(db)
	lSvc := ledgerService.NewLedgerService(lRepo, walletClient)
	lHandler := ledgerHandler.NewLedgerHandler(lSvc)

	// Setup gRPC Server
	// Dynamic approach:
	_, port, err := net.SplitHostPort(cfg.LedgerGRPCAddr)
	if err != nil {
		logger.Fatal(nil, "Failed to split host port: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal(nil, "Failed to listen gRPC", "error", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterLedgerServiceServer(grpcServer, ledgerGRPC.NewLedgerGRPCServer(lRepo))

	go func() {
		logger.Log.Info("Ledger gRPC Server running on port ", port)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal(nil, "Failed to serve gRPC", "error", err)
		}
	}()

	// Setup HTTP Server
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())

	v1 := r.Group("/api/v1")
	{
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.GET("/ledger/mutations", lHandler.GetMutations)
			protected.GET("/ledger/reconcile", lHandler.Reconcile)
		}
	}

	logger.Log.Info("Ledger HTTP Server running on port 8085...")
	if err := r.Run(":8085"); err != nil {
		logger.Fatal(nil, "Failed to run HTTP server", "error", err)
	}
}
