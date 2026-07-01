package main

import (
	"log"

	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/handler"
	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/repository"
	"github.com/bashocode/gowallet/microservices/auth-service/internal/auth/service"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	// Logger initializes automatically on import, but InitLogger remains available
	logger.InitLogger()
	logger.Log.Info("Starting Auth Microservice...")

	cfg := config.LoadConfig()

	// Connect to Redis (for token blacklisting)
	rdb, err := database.ConnectRedis(cfg.RedisAddr)
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	defer rdb.Close()

	// Connect to MySQL (for refresh tokens)
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer db.Close()

	// Initialize layers
	rtRepo := repository.NewMySQLRefreshTokenRepository(db)
	userRepo := repository.NewMySQLUserRepository(db)
	authSvc := service.NewAuthService(rdb, rtRepo, userRepo)
	authHandler := handler.NewAuthHandler(authSvc)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())

	// Auth Routes
	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/login", authHandler.Login)
		v1.POST("/auth/refresh", authHandler.RefreshToken)

		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.POST("/auth/logout", authHandler.Logout)
		}
	}

	logger.Log.Info("Auth Service listening on port 8081...")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("Auth Service failed: %v", err)
	}
}
