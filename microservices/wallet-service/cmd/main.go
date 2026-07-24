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

	// Connect to MySQL
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Fatal(context.Background(), "Could not connect to MySQL", "error", err)
	}

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

	// Register Health, Readiness, & Liveness Endpoints
	r.GET("/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	r.GET("/ready", func(c *gin.Context) {
		// Make sure MySQL is connected properly
		if err := db.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN", "reason": "MySQL database not responding"})
			return
		}
		// Make sure Redis is connected properly
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
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.GET("/wallets/me", wHandler.GetBalance)
		}
	}

	srv := &http.Server{
		Addr:    ":" + cfg.WalletPort,
		Handler: r,
	}

	// Run HTTP server in background (goroutine)
	go func() {
		logger.Log.Info("Wallet HTTP Server running on port " + cfg.WalletPort + "...")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal(context.Background(), "Server listen failed", "error", err)
		}
	}()

	// Wait for shutdown signal from OS (Ctrl+C or Docker stop)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Shutdown signal received. Starting graceful shutdown...")

	// Set wait time tolerance for request completion (e.g., 15 seconds)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Stop accepting new HTTP requests & wait for running requests to finish
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error(ctx, "HTTP Server forced to shutdown", "error", err.Error())
	} else {
		logger.Log.Info("HTTP Server closed cleanly.")
	}

	// Stop gRPC server gracefully
	logger.Log.Info("Stopping gRPC server...")
	grpcServer.GracefulStop()
	logger.Log.Info("gRPC Server closed cleanly.")

	// Clean up all other resource connections
	logger.Log.Info("Closing database and cache connections...")

	// Close Redis Client
	if err := rdb.Close(); err != nil {
		logger.Error(ctx, "Failed to close Redis client", "error", err.Error())
	}

	// Close MySQL Connection
	if err := db.Close(); err != nil {
		logger.Error(ctx, "Failed to close MySQL connection", "error", err.Error())
	}

	logger.Log.Info("Wallet Microservice successfully stopped.")
}
