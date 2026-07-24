package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/bashocode/gowallet/microservices/shared/storage"
	userCache "github.com/bashocode/gowallet/microservices/user-service/internal/user/cache"
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

	// Connect to MySQL
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(context.Background(), "Could not connect to database", "error", err)
	}

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

	walletClient := pbWallet.NewWalletServiceClient(conn)

	// Initialize MinIO storage
	minioStorage, err := storage.NewMinioStorage(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioPublicURL, false)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to initialize MinIO storage", "error", err)
	}

	// Ensure avatars bucket exists and is public
	ctxInit := context.Background()
	if err := minioStorage.EnsureBucket(ctxInit, "avatars"); err != nil {
		logger.Fatal(ctxInit, "Failed to ensure avatars bucket exists", "error", err)
	}
	if err := minioStorage.MakeBucketPublic(ctxInit, "avatars"); err != nil {
		logger.Fatal(ctxInit, "Failed to make avatars bucket public", "error", err)
	}

	// Initialize layers
	userRepo := repository.NewMySQLUserRepository(db)
	userCacheRepo := userCache.NewUserCacheRepository(rdb)
	otpRepo := repository.NewMySQLOTPRepository(db)
	notificationOutboxRepo := repository.NewMySQLNotificationOutboxRepository(db)

	userSvc := service.NewUserService(db, rdb, userRepo, userCacheRepo, walletClient, otpRepo, notificationOutboxRepo, cfg.BaseURL)
	userHandler := handler.NewUserHandler(userSvc, minioStorage)

	// Initialize and start the notification outbox worker
	notifWorker := userWorker.NewNotificationOutboxWorker(notificationOutboxRepo, cfg.RabbitMQURL)

	ctx, cancel := context.WithCancel(context.Background())

	go notifWorker.Start(ctx)

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CorrelationID())

	r.GET("/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	r.GET("/ready", func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN", "reason": "MySQL database not responding"})
			return
		}
		if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN", "reason": "Redis cache not responding"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "READY"})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "HEALTHY"})
	})

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
	pb.RegisterUserServiceServer(grpcServer, userGRPC.NewUserGRPCServer(userRepo, otpRepo, notificationOutboxRepo))

	go func() {
		logger.Log.Info("User gRPC Server running on port " + port + "...")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal(context.Background(), "Failed to serve gRPC", "error", err)
		}
	}()

	srv := &http.Server{
		Addr:    ":" + cfg.UserPort,
		Handler: r,
	}

	go func() {
		logger.Log.Info("User Service listening on port " + cfg.UserPort + "...")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal(context.Background(), "Server listen failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Shutdown signal received. Starting graceful shutdown...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.Error(ctxShutdown, "HTTP Server forced to shutdown", "error", err.Error())
	} else {
		logger.Log.Info("HTTP Server closed cleanly.")
	}

	logger.Log.Info("Stopping gRPC server...")
	grpcServer.GracefulStop()
	logger.Log.Info("gRPC Server closed cleanly.")

	logger.Log.Info("Stopping background workers...")
	cancel()

	logger.Log.Info("Closing gRPC client connections...")
	if err := conn.Close(); err != nil {
		logger.Error(ctxShutdown, "Failed to close wallet service connection", "error", err.Error())
	}

	logger.Log.Info("Closing database and cache connections...")

	if err := rdb.Close(); err != nil {
		logger.Error(ctxShutdown, "Failed to close Redis client", "error", err.Error())
	}

	if err := db.Close(); err != nil {
		logger.Error(ctxShutdown, "Failed to close MySQL connection", "error", err.Error())
	}

	logger.Log.Info("User Microservice successfully stopped.")
}
