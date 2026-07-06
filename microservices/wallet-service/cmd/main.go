package main

import (
	"net"
	"net/url"

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
	_, port, err := net.SplitHostPort(cfg.WalletGRPCAddr)
	if err != nil {
		logger.Fatal(nil, "Failed to split host port", "error", err)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal(nil, "Failed to listen gRPC", "error", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterWalletServiceServer(grpcServer, walletGRPC.NewWalletGRPCServer(wRepo))

	go func() {
		logger.Log.Info("Wallet gRPC Server running on port " + cfg.WalletGRPCAddr)
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
			protected.GET("/wallets/me", wHandler.GetBalance)
		}
	}

	u, err := url.Parse(cfg.WalletServiceURL)
	var httpPort string
	if err == nil && u.Port() != "" {
		httpPort = u.Port()
	} else {
		httpPort = "8082" // fallback
	}

	logger.Log.Info("Wallet HTTP Server running on port " + httpPort + "...")
	if err := r.Run(":" + httpPort); err != nil {
		logger.Fatal(nil, "Failed to run HTTP server", "error", err)
	}
}
