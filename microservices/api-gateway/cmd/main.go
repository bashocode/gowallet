package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/bashocode/gowallet/microservices/api-gateway/docs"
	"github.com/bashocode/gowallet/microservices/api-gateway/internal/middleware"
	"github.com/bashocode/gowallet/microservices/api-gateway/internal/proxy"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	sharedMiddleware "github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           GoWallet API (Microservices)
// @version         2.0
// @description     Unified API documentation for all GoWallet microservices proxied through the API Gateway.
// @termsOfService  http://swagger.io/terms/

// @contact.name   GoWallet API Support
// @contact.email  support@gowallet.com

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	cfg := config.LoadConfig()
	logger.Log.Info("Starting API Gateway on port " + cfg.GatewayPort + "...")

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: "",
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Log.Warn("Redis unavailable — rate limiter will skip. addr=" + cfg.RedisAddr + " err=" + err.Error())
		rdb = nil
	} else {
		logger.Log.Info("Connected to Redis successfully!")
	}

	authProxy, err := proxy.NewReverseProxy(cfg.AuthServiceURL)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to initialize auth proxy", "error", err)
	}

	userProxy, err := proxy.NewReverseProxy(cfg.UserServiceURL)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to initialize user proxy", "error", err)
	}

	walletProxy, err := proxy.NewReverseProxy(cfg.WalletServiceURL)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to initialize wallet proxy", "error", err)
	}

	ledgerProxy, err := proxy.NewReverseProxy(cfg.LedgerServiceURL)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to initialize ledger proxy", "error", err)
	}

	transactionProxy, err := proxy.NewReverseProxy(cfg.TransactionServiceURL)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to initialize transaction proxy", "error", err)
	}

	paymentProxy, err := proxy.NewReverseProxy(cfg.PaymentServiceURL)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to initialize payment proxy", "error", err)
	}

	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.Use(sharedMiddleware.ErrorHandler())
	r.Use(sharedMiddleware.CorrelationID())
	r.Use(middleware.CORSMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "api-gateway",
		})
	})

	r.GET("/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	r.GET("/ready", func(c *gin.Context) {
		if rdb != nil {
			if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN", "reason": "Redis cache not responding"})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "READY"})
	})

	if rdb != nil {
		r.Use(sharedMiddleware.RateLimiter(rdb, 60, time.Minute))
	}

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.Any("/api/v1/auth/*path", func(c *gin.Context) {
		authProxy.ServeHTTP(c.Writer, c.Request)
	})

	r.Any("/api/v1/users/*path", func(c *gin.Context) {
		userProxy.ServeHTTP(c.Writer, c.Request)
	})

	r.Any("/api/v1/admin/*path", func(c *gin.Context) {
		userProxy.ServeHTTP(c.Writer, c.Request)
	})

	r.Any("/api/v1/wallets/*path", func(c *gin.Context) {
		walletProxy.ServeHTTP(c.Writer, c.Request)
	})

	r.Any("/api/v1/ledger/*path", func(c *gin.Context) {
		ledgerProxy.ServeHTTP(c.Writer, c.Request)
	})

	r.Any("/api/v1/transactions/*path", func(c *gin.Context) {
		transactionProxy.ServeHTTP(c.Writer, c.Request)
	})

	r.Any("/api/v1/payments/*path", func(c *gin.Context) {
		paymentProxy.ServeHTTP(c.Writer, c.Request)
	})

	srv := &http.Server{
		Addr:    ":" + cfg.GatewayPort,
		Handler: r,
	}

	go func() {
		logger.Log.Info("API Gateway listening on port " + cfg.GatewayPort + "...")
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

	logger.Log.Info("Closing Redis connection...")
	if rdb != nil {
		if err := rdb.Close(); err != nil {
			logger.Error(ctxShutdown, "Failed to close Redis client", "error", err.Error())
		}
	}

	logger.Log.Info("API Gateway successfully stopped.")
}
