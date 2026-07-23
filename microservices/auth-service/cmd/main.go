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
	logger.InitLogger()
	logger.Log.Info("Starting Auth Microservice...")

	cfg := config.LoadConfig()

	rdb, err := database.ConnectRedis(cfg.RedisAddr)
	if err != nil {
		logger.Fatal(context.Background(), "Could not connect to Redis", "error", err)
	}

	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(context.Background(), "Could not connect to database", "error", err)
	}

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
		logger.Fatal(context.Background(), "Failed to connect to user service", "error", err)
	}

	userClient := pb.NewUserServiceClient(userConn)

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
		logger.Fatal(context.Background(), "Failed to connect to wallet service", "error", err)
	}

	walletClient := pbWallet.NewWalletServiceClient(walletConn)

	rtRepo := repository.NewMySQLRefreshTokenRepository(db)
	authSvc := service.NewAuthService(rdb, rtRepo, userClient, walletClient)
	authHandler := handler.NewAuthHandler(authSvc)

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

	_, grpcPort, err := net.SplitHostPort(cfg.AuthGRPCAddr)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to split Auth gRPC host port", "error", err)
	}

	authLis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to listen Auth gRPC", "error", err)
	}

	grpcServer := grpc.NewServer()
	pbAuth.RegisterAuthServiceServer(grpcServer, authGRPC.NewAuthGRPCServer(rtRepo))

	go func() {
		logger.Log.Info("Auth gRPC Server running on " + cfg.AuthGRPCAddr)
		if err := grpcServer.Serve(authLis); err != nil {
			logger.Fatal(context.Background(), "Failed to serve Auth gRPC", "error", err)
		}
	}()

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

	srv := &http.Server{
		Addr:    ":" + cfg.AuthPort,
		Handler: r,
	}

	go func() {
		logger.Log.Info("Auth Service listening on port " + cfg.AuthPort + "...")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal(context.Background(), "Server listen failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Shutdown signal received. Starting graceful shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error(ctx, "HTTP Server forced to shutdown", "error", err.Error())
	} else {
		logger.Log.Info("HTTP Server closed cleanly.")
	}

	logger.Log.Info("Stopping gRPC server...")
	grpcServer.GracefulStop()
	logger.Log.Info("gRPC Server closed cleanly.")

	logger.Log.Info("Closing gRPC client connections...")
	if err := userConn.Close(); err != nil {
		logger.Error(ctx, "Failed to close user service connection", "error", err.Error())
	}
	if err := walletConn.Close(); err != nil {
		logger.Error(ctx, "Failed to close wallet service connection", "error", err.Error())
	}

	logger.Log.Info("Closing database and cache connections...")

	if err := rdb.Close(); err != nil {
		logger.Error(ctx, "Failed to close Redis client", "error", err.Error())
	}

	if err := db.Close(); err != nil {
		logger.Error(ctx, "Failed to close MySQL connection", "error", err.Error())
	}

	logger.Log.Info("Auth Microservice successfully stopped.")
}
