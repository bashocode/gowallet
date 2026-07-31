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

	pbLedger "github.com/bashocode/gowallet/microservices/ledger-service/proto/ledger"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	sharedGRPC "github.com/bashocode/gowallet/microservices/shared/grpc"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/bashocode/gowallet/microservices/shared/tracing"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/dlq"
	transactionCache "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/cache"
	transactionGRPC "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/grpc"
	transactionHandler "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/handler"
	transactionRepository "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/repository"
	transferRepository "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/repository"
	transactionService "github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/service"
	"github.com/bashocode/gowallet/microservices/transaction-service/internal/transaction/worker"
	pb "github.com/bashocode/gowallet/microservices/transaction-service/proto/transaction"
	pbUser "github.com/bashocode/gowallet/microservices/user-service/proto/user"
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
		logger.WithServiceName("transaction-service"),
		logger.WithLogstashAddr(cfg.LogstashAddr),
	)
	logger.Log.Info("Starting Transaction Microservice...")

	// Initialize OpenTelemetry Tracer
	tp, err := tracing.InitTracer("transaction-service", cfg.OTELCollectorAddr)
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

	// Initialize & Start Outbox Worker
	outboxWorker := worker.NewOutboxWorker(db, cfg.RabbitMQURL)

	bgCtx, cancelWorker := context.WithCancel(context.Background())

	go outboxWorker.Start(bgCtx)

	userCreds, err := sharedGRPC.GetClientDialCredentials(
		cfg.IsProduction(),
		cfg.GRPCSSLCertPath,
		cfg.GRPCSSLKeyPath,
		cfg.GRPCSSLCAPath,
		"user-service",
	)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load gRPC client credentials for user-service", "error", err)
	}

	// Connect to User Service gRPC
	userConn, err := grpc.NewClient(
		cfg.UserGRPCAddr,
		grpc.WithTransportCredentials(userCreds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			sharedGRPC.UnaryClientIdentity("transaction-service"),
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
		logger.Fatal(context.Background(), "Failed to connect to user service", "error", err)
	}

	userClient := pbUser.NewUserServiceClient(userConn)

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

	// Connect to Wallet Service gRPC
	walletConn, err := grpc.NewClient(
		cfg.WalletGRPCAddr,
		grpc.WithTransportCredentials(walletCreds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			sharedGRPC.UnaryClientIdentity("transaction-service"),
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

	walletClient := pbWallet.NewWalletServiceClient(walletConn)

	ledgerCreds, err := sharedGRPC.GetClientDialCredentials(
		cfg.IsProduction(),
		cfg.GRPCSSLCertPath,
		cfg.GRPCSSLKeyPath,
		cfg.GRPCSSLCAPath,
		"ledger-service",
	)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load gRPC client credentials for ledger-service", "error", err)
	}

	// Connect to Ledger Service gRPC
	ledgerConn, err := grpc.NewClient(
		cfg.LedgerGRPCAddr,
		grpc.WithTransportCredentials(ledgerCreds),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			sharedGRPC.UnaryClientIdentity("transaction-service"),
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
		logger.Fatal(context.Background(), "Failed to connect to ledger service", "error", err)
	}

	dlqPublisher := dlq.NewNoOpPublisher()

	ledgerClient := pbLedger.NewLedgerServiceClient(ledgerConn)

	// Initialize layers
	txRepo := transactionRepository.NewMySQLTransactionRepository(db)
	txCache := transactionCache.NewTransactionCacheRepository(rdb)
	cacheEvictionRepo := transactionRepository.NewCacheEvictionRepository(rdb)
	outboundTransferRepo := transferRepository.NewMySQLOutboundTransferRepository(db)
	transferOutboxRepo := transferRepository.NewMySQLTransferOutboxRepository(db)
	txSvc := transactionService.NewTransactionService(db, txRepo, txCache, cacheEvictionRepo, outboundTransferRepo, transferOutboxRepo, userClient, walletClient, ledgerClient, dlqPublisher, cfg.MonolithBaseURL, cfg.TransactionBaseURL, cfg.WebhookSecret)
	txHandler := transactionHandler.NewTransactionHandler(txSvc)

	externalHandler := transactionHandler.NewTransferHandler(txSvc, cfg.WebhookSecret, cfg.MonolithBaseURL)

	// Start the transfer outbox publisher worker (publishes transfer.* events to transfer.events).
	transferOutboxWorker := worker.NewTransferOutboxWorker(db, cfg.RabbitMQURL, transferOutboxRepo)
	go transferOutboxWorker.Start(bgCtx)

	// Start the transfer consumer worker (consumes transfer.initiated from queue,
	// validates receiver, notifies monolith, then settles the outbound transfer).
	transferConsumerWorker := worker.NewTransferConsumerWorker(cfg.RabbitMQURL, txSvc)
	go transferConsumerWorker.Start(bgCtx)

	// Start the payment consumer worker (consumes payment.settled from queue,
	// triggers TopUp transaction).
	paymentConsumerWorker := worker.NewPaymentConsumerWorker(cfg.RabbitMQURL, txSvc)
	go paymentConsumerWorker.Start(bgCtx)

	// Start reconciliation worker: checks for stale pending transfers every 2 minutes.
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				if err := txSvc.ReconcilePendingTransfers(bgCtx); err != nil {
					logger.Log.Error("Transfer reconciliation failed", "error", err)
				}
			}
		}
	}()

	// =========================================================
	// Start gRPC Server (for internal service-to-service calls)
	// =========================================================
	_, port, err := net.SplitHostPort(cfg.TransactionGRPCAddr)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to split host port: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+port)

	if err != nil {
		logger.Fatal(context.Background(), "Failed to listen gRPC", "error", err)
	}

	serverOpts, err := sharedGRPC.GetServerOptions(
		cfg.IsProduction(),
		cfg.GRPCSSLCertPath,
		cfg.GRPCSSLKeyPath,
		cfg.GRPCSSLCAPath,
	)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load gRPC server credentials", "error", err)
	}

	serverOpts = append(
		serverOpts,
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			sharedGRPC.RequireServiceIdentity(
				!cfg.IsProduction(),
				"api-gateway",
				"payment-service",
				"scheduler-service",
			),
		),
	)

	grpcServer := grpc.NewServer(serverOpts...)
	pb.RegisterTransactionServiceServer(grpcServer, transactionGRPC.NewTransactionGRPCServer(txSvc, txRepo))

	go func() {
		logger.Log.Info("Transaction gRPC server listening on port " + port + "...")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal(context.Background(), "gRPC server failed", "error", err)
		}
	}()

	// =========================================================
	// Setup HTTP Server (public routes only — no topup exposed)
	// =========================================================
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.PrometheusMetrics())
	r.Use(otelgin.Middleware("transaction-service"))
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
			protected.POST("/transactions/transfer", txHandler.Transfer)
			protected.GET("/transactions/history", txHandler.GetHistory)

			protected.POST("/transactions/inquiry/external", externalHandler.InquiryExternal)
			protected.POST("/transactions/transfers/external", externalHandler.CreateExternalTransfer)
		}

		// Webhook endpoint for external transfer callbacks (protected by API key, not JWT)
		internal := v1.Group("")
		internal.Use(middleware.APIKeyMiddleware(cfg.WebhookSecret))
		{
			internal.POST("/transactions/transfers/webhook", externalHandler.ProcessTransferWebhook)
		}
	}

	srv := &http.Server{
		Addr:    ":" + cfg.TransactionPort,
		Handler: r,
	}

	go func() {
		logger.Log.Info("Transaction Service HTTP server listening on port " + cfg.TransactionPort + "...")
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

	logger.Log.Info("Stopping background workers...")
	cancelWorker()

	logger.Log.Info("Closing gRPC client connections...")
	if err := userConn.Close(); err != nil {
		logger.Error(ctx, "Failed to close user service connection", "error", err.Error())
	}
	if err := walletConn.Close(); err != nil {
		logger.Error(ctx, "Failed to close wallet service connection", "error", err.Error())
	}
	if err := ledgerConn.Close(); err != nil {
		logger.Error(ctx, "Failed to close ledger service connection", "error", err.Error())
	}

	logger.Log.Info("Closing database and cache connections...")

	if err := rdb.Close(); err != nil {
		logger.Error(ctx, "Failed to close Redis client", "error", err.Error())
	}

	if err := db.Close(); err != nil {
		logger.Error(ctx, "Failed to close MySQL connection", "error", err.Error())
	}

	logger.Log.Info("Transaction Microservice successfully stopped.")
}
