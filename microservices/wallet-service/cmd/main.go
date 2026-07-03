package main

import (
	"log"
	"net"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	walletGRPC "github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/grpc"
	walletHandler "github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/handler"
	walletRepository "github.com/bashocode/gowallet/microservices/wallet-service/internal/wallet/repository"
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
		logger.Fatal(nil, "Could not connect to Redis", "error", err)
	}
	defer rdb.Close()

	// Connect to MySQL
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(nil, "Could not connect to MySQL", "error", err)
	}
	defer db.Close()

	wRepo := walletRepository.NewMySQLWalletRepository(db)
	wHandler := walletHandler.NewWalletHandler(wRepo)

	// Setup gRPC Server
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("Failed to listen gRPC: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterWalletServiceServer(grpcServer, walletGRPC.NewWalletGRPCServer(wRepo))

	go func() {
		logger.Log.Info("Wallet gRPC Server running on port 50053...")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
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
			protected.GET("/wallets/me", wHandler.GetBalance)
		}
	}

	logger.Log.Info("Wallet HTTP Server running on port 8082...")
	if err := r.Run(":8082"); err != nil {
		logger.Fatal(nil, "Failed to run HTTP server", "error", err)
	}
}
