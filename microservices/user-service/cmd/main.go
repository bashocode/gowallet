package main

import (
	"context"
	"net"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	userGRPC "github.com/bashocode/gowallet/microservices/user-service/internal/user/grpc"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/handler"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/repository"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/service"
	userWorker "github.com/bashocode/gowallet/microservices/user-service/internal/user/worker"
	pb "github.com/bashocode/gowallet/microservices/user-service/proto/user"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("Starting User Microservice...")

	cfg := config.LoadConfig()

	// Connect to Redis
	rdb, err := database.ConnectRedis(cfg.RedisAddr)
	if err != nil {
		logger.Fatal(context.Background(), "Could not connect to Redis", "error", err)
	}
	defer rdb.Close()

	// Connect to MySQL
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(context.Background(), "Could not connect to database", "error", err)
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
		logger.Fatal(context.Background(), "Failed to connect to wallet service", "error", err)
	}
	defer conn.Close()
	// Note: isn't deferred here since the main function blocks on HTTP server, but we can defer it or keep it open.

	walletClient := pbWallet.NewWalletServiceClient(conn)

	// Initialize layers
	userRepo := repository.NewMySQLUserRepository(db)
	otpRepo := repository.NewMySQLOTPRepository(db)
	notificationOutboxRepo := repository.NewMySQLNotificationOutboxRepository(db)

	userSvc := service.NewUserService(db, rdb, userRepo, walletClient, otpRepo, notificationOutboxRepo, cfg.BaseURL)
	userHandler := handler.NewUserHandler(userSvc)

	// Initialize and start the notification outbox worker
	notifWorker := userWorker.NewNotificationOutboxWorker(notificationOutboxRepo, cfg.RabbitMQURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go notifWorker.Start(ctx)

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CorrelationID())

	v1 := r.Group("/api/v1")
	{
		// Public Routes
		v1.POST("/users/register", userHandler.Register)
		v1.POST("/users/forgot-password", userHandler.ForgotPassword)
		v1.POST("/users/verify-password-reset", userHandler.VerifyPasswordReset)
		v1.GET("/users/verify-email", userHandler.VerifyEmail)

		// Protected Routes
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.GET("/users/me", userHandler.GetProfileMe)
			protected.POST("/users/avatar", userHandler.UploadAvatar)
			protected.PUT("/users/:id", userHandler.UpdateProfile)
			protected.GET("/users/:id", userHandler.GetProfile)
			protected.DELETE("/users/me", userHandler.DeleteAccount)

			// Admin Routes
			adminOnly := protected.Group("/admin")
			adminOnly.Use(middleware.RequireRole("admin"))
			{
				adminOnly.GET("/users", userHandler.AdminGetUsers)
			}
		}
	}

	// Start gRPC server
	// Dynamic approach:
	_, port, err := net.SplitHostPort(cfg.UserGRPCAddr)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to split gRPC host port: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to listen gRPC port"+port, "error", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, userGRPC.NewUserGRPCServer(userRepo, otpRepo))

	go func() {
		logger.Log.Info("User gRPC Server running on port " + port + "...")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal(context.Background(), "Failed to serve gRPC", "error", err)
		}
	}()

	logger.Log.Info("User Service listening on port " + cfg.UserPort + "...")
	if err := r.Run(":" + cfg.UserPort); err != nil {
		logger.Fatal(context.Background(), "User Service failed", "error", err)
	}
}
