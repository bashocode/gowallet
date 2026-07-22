package main

import (
	"context"
	"net"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	walletCache "github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/cache"
	walletGRPC "github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/grpc"
	walletHandler "github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/handler"
	walletRepository "github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/repository"
	walletService "github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/service"
	pb "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("Starting Wallet Microservice...")

	cfg := config.LoadConfig()

	// Connect to Redis (required by AuthMiddleware)
	rdb, err := database.ConnectRedis(cfg.RedisAddr)
	if err != nil {
		logger.Fatal(context.Background(), "Could not connect to Redis", "error", err)
	}
	defer rdb.Close()

	// Connect to MySQL
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(context.Background(), "Could not connect to MySQL", "error", err)
	}
	defer db.Close()

	wRepo := walletRepository.NewMySQLWalletRepository(db)
	wCache := walletCache.NewWalletCacheRepository(rdb)
	wSvc := walletService.NewWalletService(wRepo, wCache)
	wHandler := walletHandler.NewWalletHandler(wSvc)

	// Setup gRPC Server
	_, port, err := net.SplitHostPort(cfg.WalletGRPCAddr)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to split host port", "error", err)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to listen gRPC", "error", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterWalletServiceServer(grpcServer, walletGRPC.NewWalletGRPCServer(wSvc))

	go func() {
		logger.Log.Info("Wallet gRPC Server running on port " + cfg.WalletGRPCAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal(context.Background(), "Failed to serve gRPC", "error", err)
		}
	}()

	// Setup HTTP Server
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CorrelationID())

	v1 := r.Group("/api/v1")
	{
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.GET("/wallets/me", wHandler.GetBalance)
		}
	}

	logger.Log.Info("Wallet HTTP Server running on port " + cfg.WalletPort + "...")
	if err := r.Run(":" + cfg.WalletPort); err != nil {
		logger.Fatal(context.Background(), "Failed to run HTTP server", "error", err)
	}
}
