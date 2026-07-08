package main

import (
	"net"

	authGRPC "github.com/bashocode/gowallet/microservices/auth-service/internal/auth/grpc"
	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/handler"
	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/repository"
	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/service"
	pbAuth "github.com/bashocode/gowallet/microservices/auth-service/proto/auth"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Logger initializes automatically on import, but InitLogger remains available
	logger.InitLogger()
	logger.Log.Info("Starting Auth Microservice...")

	cfg := config.LoadConfig()

	// Connect to Redis (for token blacklisting)
	rdb, err := database.ConnectRedis(cfg.RedisAddr)
	if err != nil {
		logger.Fatal(nil, "Could not connect to Redis", "error", err)
	}
	defer rdb.Close()

	// Connect to MySQL (for refresh tokens)
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(nil, "Could not connect to database", "error", err)
	}
	defer db.Close()

	// Connect to User Service via gRPC
	userConn, err := grpc.NewClient(
		cfg.UserGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{
			"methodConfig": [{
				"name": [{"service": "user.UserService"}],
				"retryPolicy": {
					"maxAttempts": 3,
					"initialBackoff": "0.1s",
					"maxBackoff": "1s",
					"backoffMultiplier": 2,
					"retryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED"]
				}
			}]
		}`),
	)
	if err != nil {
		logger.Fatal(nil, "Failed to connect to user service", "error", err)
	}
	defer userConn.Close()

	userClient := pb.NewUserServiceClient(userConn)

	// Connect to Wallet Service via gRPC (for OAuth user wallet creation)
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

	// Initialize layers
	rtRepo := repository.NewMySQLRefreshTokenRepository(db)
	authSvc := service.NewAuthService(rdb, rtRepo, userClient, walletClient)
	authHandler := handler.NewAuthHandler(authSvc)

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CorrelationID())

	// =========================================================
	// Start gRPC Server (for internal service-to-service calls,
	// e.g. scheduler-service triggering cleanup jobs)
	// =========================================================
	_, grpcPort, err := net.SplitHostPort(cfg.AuthGRPCAddr)
	if err != nil {
		logger.Fatal(nil, "Failed to split Auth gRPC host port", "error", err)
	}

	authLis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Fatal(nil, "Failed to listen Auth gRPC", "error", err)
	}

	grpcServer := grpc.NewServer()
	pbAuth.RegisterAuthServiceServer(grpcServer, authGRPC.NewAuthGRPCServer(rtRepo))

	go func() {
		logger.Log.Info("Auth gRPC Server running on " + cfg.AuthGRPCAddr)
		if err := grpcServer.Serve(authLis); err != nil {
			logger.Fatal(nil, "Failed to serve Auth gRPC", "error", err)
		}
	}()

	// Auth Routes
	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/login", authHandler.Login)
		v1.POST("/auth/refresh", authHandler.RefreshToken)
		v1.GET("/auth/google/login", authHandler.GoogleLogin)
		v1.GET("/auth/google/callback", authHandler.GoogleCallback)

		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.POST("/auth/logout", authHandler.Logout)
		}
	}

	logger.Log.Info("Auth Service listening on port " + cfg.AuthPort + "...")
	if err := r.Run(":" + cfg.AuthPort); err != nil {
		logger.Fatal(nil, "Auth Service failed", "error", err)
	}
}
