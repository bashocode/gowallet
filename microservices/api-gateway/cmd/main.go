package main

import (
	"context"
	"net/http"
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
	// Load configuration
	cfg := config.LoadConfig()
	logger.Log.Info("Starting API Gateway on port " + cfg.GatewayPort + "...")

	// 1. Connect to Redis for RateLimiter (fail-open: skip if unavailable)
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

	// 2. Create proxy routers for each target microservice

	// 2. Create reverse proxy for each target microservice
	authProxy, err := proxy.NewReverseProxy(cfg.AuthServiceURL)
	if err != nil {
		logger.Fatal(nil, "Failed to initialize auth proxy", "error", err)
	}

	userProxy, err := proxy.NewReverseProxy(cfg.UserServiceURL)
	if err != nil {
		logger.Fatal(nil, "Failed to initialize user proxy", "error", err)
	}

	walletProxy, err := proxy.NewReverseProxy(cfg.WalletServiceURL)
	if err != nil {
		logger.Fatal(nil, "Failed to initialize wallet proxy", "error", err)
	}

	ledgerProxy, err := proxy.NewReverseProxy(cfg.LedgerServiceURL)
	if err != nil {
		logger.Fatal(nil, "Failed to initialize ledger proxy", "error", err)
	}

	transactionProxy, err := proxy.NewReverseProxy(cfg.TransactionServiceURL)
	if err != nil {
		logger.Fatal(nil, "Failed to initialize transaction proxy", "error", err)
	}

	paymentProxy, err := proxy.NewReverseProxy(cfg.PaymentServiceURL)
	if err != nil {
		logger.Fatal(nil, "Failed to initialize payment proxy", "error", err)
	}

	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 3. Register middleware chain (ORDER MATTERS!)
	//    ErrorHandler must be first so it catches errors from all subsequent middleware
	r.Use(sharedMiddleware.ErrorHandler())
	//    CorrelationID assigns a unique ID to every request
	r.Use(sharedMiddleware.CorrelationID())
	//    CORS allows browser-based clients
	r.Use(middleware.CORSMiddleware())

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "api-gateway",
		})
	})

	//    RateLimiter throttles abusive clients (60 req/min per IP)
	if rdb != nil {
		r.Use(sharedMiddleware.RateLimiter(rdb, 60, time.Minute))
	}

	// Swagger UI — registered before proxy routes so it's excluded from rate limiting
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 4. Define proxy routing rules
	// /api/v1/auth/* is forwarded to Auth Service (login, refresh, logout, Google OAuth)
	r.Any("/api/v1/auth/*path", func(c *gin.Context) {
		authProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/users/* is forwarded to User Service on port 8084
	r.Any("/api/v1/users/*path", func(c *gin.Context) {
		userProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/admin/* is forwarded to User Service on port 8084
	r.Any("/api/v1/admin/*path", func(c *gin.Context) {
		userProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/wallets/* is forwarded to Wallet Service on port 8082
	r.Any("/api/v1/wallets/*path", func(c *gin.Context) {
		walletProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/ledger/* is forwarded to Ledger Service on port 8085
	r.Any("/api/v1/ledger/*path", func(c *gin.Context) {
		ledgerProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/transactions/* is forwarded to Transaction Service on port 8086
	r.Any("/api/v1/transactions/*path", func(c *gin.Context) {
		transactionProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/transfers/* is forwarded to Transaction Service
	r.Any("/api/v1/transfers/*path", func(c *gin.Context) {
		transactionProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/payments/* is forwarded to Payment Service on port 8083
	r.Any("/api/v1/payments/*path", func(c *gin.Context) {
		paymentProxy.ServeHTTP(c.Writer, c.Request)
	})

	logger.Log.Info("API Gateway listening on port " + cfg.GatewayPort + "...")
	if err := r.Run(":" + cfg.GatewayPort); err != nil {
		logger.Fatal(nil, "Gateway failed", "error", err)
	}
}
