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

	ledgerCache "github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/cache"
	ledgerGRPC "github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/grpc"
	ledgerHandler "github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/handler"
	ledgerRepository "github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/repository"
	ledgerService "github.com/bashocode/gowallet/microservices/ledger-service/internal/ledger/service"
	pb "github.com/bashocode/gowallet/microservices/ledger-service/proto/ledger"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	sharedGRPC "github.com/bashocode/gowallet/microservices/shared/grpc"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/bashocode/gowallet/microservices/shared/tracing"
	pbWallet "github.com/bashocode/gowallet/microservices/wallet-service/proto/wallet"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.LoadConfig()
	logger.InitLogger(
		logger.WithServiceName("ledger-service"),
		logger.WithLogstashAddr(cfg.LogstashAddr),
	)
	logger.Log.Info("Starting Ledger Microservice...")

	// Initialize OpenTelemetry Tracer
	tp, err := tracing.InitTracer("ledger-service", cfg.OTELCollectorAddr)
	if err != nil {
		logger.Log.Warn("Failed to initialize tracer, continuing without tracing: " + err.Error())
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tp.Shutdown(shutdownCtx)
		}()
	}

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

	walletCreds, err := sharedGRPC.GetClientDialCredentials(
		cfg.IsProduction(),
		cfg.GRPCSSLCertPath,
		cfg.GRPCSSLKeyPath,
		cfg.GRPCSSLCAPath,
		"wallet-service",
	)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load gRPC client credentials for wallet-service", "error", err)
	}

	// Connect to wallet-service gRPC
	conn, err := grpc.NewClient(
		cfg.WalletGRPCAddr,
		grpc.WithTransportCredentials(walletCreds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			sharedGRPC.UnaryClientIdentity("ledger-service"),
			sharedGRPC.UnaryClientTimeout(5*time.Second),
		),
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

	// Initialize layers
	lRepo := ledgerRepository.NewMySQLLedgerRepository(db)
	lCache := ledgerCache.NewLedgerCacheRepository(rdb)
	lSvc := ledgerService.NewLedgerService(lRepo, lCache, walletClient)
	lHandler := ledgerHandler.NewLedgerHandler(lSvc)

	// Setup gRPC Server
	// Dynamic approach:
	_, port, err := net.SplitHostPort(cfg.LedgerGRPCAddr)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to split host port: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to listen gRPC", "error", err)
	}

	serverOpts, err := sharedGRPC.GetServerOptions(cfg.IsProduction(), cfg.GRPCSSLCertPath, cfg.GRPCSSLKeyPath, cfg.GRPCSSLCAPath)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load gRPC server credentials", "error", err)
	}
	serverOpts = append(serverOpts, grpc.StatsHandler(otelgrpc.NewServerHandler()), grpc.ChainUnaryInterceptor(sharedGRPC.RequireServiceIdentity(!cfg.IsProduction(), "transaction-service", "wallet-service", "api-gateway")))

	grpcServer := grpc.NewServer(serverOpts...)
	pb.RegisterLedgerServiceServer(grpcServer, ledgerGRPC.NewLedgerGRPCServer(lRepo, lCache))

	go func() {
		logger.Log.Info("Ledger gRPC Server running on", "port", port)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal(context.Background(), "Failed to serve gRPC", "error", err)
		}
	}()

	// Setup HTTP Server
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.PrometheusMetrics())
	r.Use(otelgin.Middleware("ledger-service"))
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CorrelationID())

	// Metrics endpoint for Prometheus
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

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
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb, cfg.JWTSecret))
		{
			protected.GET("/ledger/mutations", lHandler.GetMutations)
			protected.GET("/ledger/reconcile", lHandler.Reconcile)
		}
	}

	srv := &http.Server{
		Addr:    ":" + cfg.LedgerPort,
		Handler: r,
	}

	go func() {
		logger.Log.Info("Ledger HTTP Server running", "port", cfg.LedgerPort)
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
	if err := conn.Close(); err != nil {
		logger.Error(ctx, "Failed to close wallet service connection", "error", err.Error())
	}

	logger.Log.Info("Closing database and cache connections...")

	if err := rdb.Close(); err != nil {
		logger.Error(ctx, "Failed to close Redis client", "error", err.Error())
	}

	if err := db.Close(); err != nil {
		logger.Error(ctx, "Failed to close MySQL connection", "error", err.Error())
	}

	logger.Log.Info("Ledger Microservice successfully stopped.")
}
