package main

import (
	"github.com/bashocode/gowallet/monolith/internal/config"
	"github.com/bashocode/gowallet/monolith/internal/database"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/bashocode/gowallet/monolith/internal/middleware"
	userHandler "github.com/bashocode/gowallet/monolith/internal/user/handler"
	userRepository "github.com/bashocode/gowallet/monolith/internal/user/repository"
	userService "github.com/bashocode/gowallet/monolith/internal/user/service"
	"github.com/gin-gonic/gin"
)

func main() {
	// initialize the log
	logger.InitLogger()
	logger.Log.Info("Starting Monolith Wallet Application...")

	// 1. load configuration
	cfg := config.LoadConfig()

	// 2. connect to database with retry
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		logger.Log.Error("Critical Error: Could not connect to database after retries", "error", err)
	}
	defer db.Close()

	// 1. initiate layer
	uRepo := userRepository.NewMySQLUserRepository(db)
	uSvc := userService.NewUserService(uRepo)
	uHandler := userHandler.NewUserHandler(uSvc)

	// 2. setup gin router
	r := gin.Default()

	// Register global error handling middleware
	r.Use(middleware.ErrorHandler())

	// Route grouping
	v1 := r.Group("/api/v1")
	{
		// Public routes
		v1.POST("/users/register", uHandler.Register)
		v1.POST("/users/login", uHandler.Login)
		v1.POST("/users", uHandler.Register)
		v1.GET("/users/:id", uHandler.GetProfile)
		v1.PUT("/users/:id", uHandler.UpdateProfile)

		// Protected routes (requires valid JWT token)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.GET("/users/me", uHandler.GetProfileMe)
		}
	}

	// start server
	logger.Log.Info("Server running on port 8080....")
	if err := r.Run(":8080"); err != nil {
		logger.Log.Error("Server failed to run", "error", err)
	}
}
