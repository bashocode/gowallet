package main

import (
	"log"
	"net/http"

	"github.com/bashocode/gowallet/microservices/api-gateway/internal/middleware"
	"github.com/bashocode/gowallet/microservices/api-gateway/internal/proxy"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("Starting API Gateway on port 8080...")

	// Load configuration
	cfg := config.LoadConfig()

	// 2. Create reverse proxy for each target microservice
	authProxy, err := proxy.NewReverseProxy(cfg.AuthServiceURL)
	if err != nil {
		log.Fatalf("Failed to initialize auth proxy: %v", err)
	}

	userProxy, err := proxy.NewReverseProxy(cfg.UserServiceURL)
	if err != nil {
		log.Fatalf("Failed to initialize user proxy: %v", err)
	}

	walletProxy, err := proxy.NewReverseProxy(cfg.WalletServiceURL)
	if err != nil {
		log.Fatalf("Failed to initialize wallet proxy: %v", err)
	}

	transactionProxy, err := proxy.NewReverseProxy(cfg.TransactionServiceURL)
	if err != nil {
		log.Fatalf("Failed to initialize transaction proxy: %v", err)
	}

	paymentProxy, err := proxy.NewReverseProxy(cfg.PaymentServiceURL)
	if err != nil {
		log.Fatalf("Failed to initialize payment proxy: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Enable CORS Middleware
	r.Use(middleware.CORSMiddleware())

	// 3. Define proxy routing rules
	// /api/v1/auth/* is forwarded to Auth Service (or User Service for Google OAuth)
	r.Any("/api/v1/auth/*path", func(c *gin.Context) {
		path := c.Param("path")
		// Forward Google OAuth requests to user-service, others to auth-service
		if len(path) >= 7 && path[:7] == "/google" {
			userProxy.ServeHTTP(c.Writer, c.Request)
			return
		}
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

	// /api/v1/transactions/* is forwarded to Transaction Service on port 8086
	r.Any("/api/v1/transactions/*path", func(c *gin.Context) {
		transactionProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/payments/* is forwarded to Payment Service on port 8083
	r.Any("/api/v1/payments/*path", func(c *gin.Context) {
		paymentProxy.ServeHTTP(c.Writer, c.Request)
	})

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "api-gateway",
		})
	})

	log.Println("API Gateway listening on port 8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Gateway failed: %v", err)
	}
}
