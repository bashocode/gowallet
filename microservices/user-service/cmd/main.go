package main

import (
	"log"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
	"github.com/bashocode/gowallet/microservices/shared/middleware"
	"github.com/bashocode/gowallet/microservices/user-service/internal/email"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/handler"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/repository"
	"github.com/bashocode/gowallet/microservices/user-service/internal/user/service"
	"github.com/gin-gonic/gin"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("Starting User Microservice...")

	cfg := config.LoadConfig()

	// Connect to Redis
	rdb, err := database.ConnectRedis(cfg.RedisAddr)
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	defer rdb.Close()

	// Connect to MySQL
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer db.Close()

	// Initialize email sender
	emailSender := email.NewSMTPEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)

	// Initialize layers
	userRepo := repository.NewMySQLUserRepository(db)
	walletRepo := repository.NewMySQLWalletRepository(db)
	otpRepo := repository.NewMySQLOTPRepository(db)

	userSvc := service.NewUserService(db, rdb, userRepo, walletRepo, otpRepo, emailSender)
	userHandler := handler.NewUserHandler(userSvc)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())

	v1 := r.Group("/api/v1")
	{
		// Public Routes
		v1.POST("/users/register", userHandler.Register)
		v1.POST("/users/forgot-password", userHandler.ForgotPassword)
		v1.POST("/users/verify-password-reset", userHandler.VerifyPasswordReset)

		// Google OAuth Routes (Using specific path matching so it aligns with gateway redirect)
		v1.GET("/auth/google/login", userHandler.GoogleLogin)
		v1.GET("/auth/google/callback", userHandler.GoogleCallback)

		// Protected Routes
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.GET("/users/me", userHandler.GetProfileMe)
			protected.POST("/users/avatar", userHandler.UploadAvatar)
			protected.PUT("/users/:id", userHandler.UpdateProfile)
			protected.GET("/users/:id", userHandler.GetProfile)
			protected.DELETE("/users/me", userHandler.DeleteAccount)
			protected.POST("/users/verify-email", userHandler.VerifyEmail)

			// Admin Routes
			adminOnly := protected.Group("/admin")
			adminOnly.Use(middleware.RequireRole("admin"))
			{
				adminOnly.GET("/users", userHandler.AdminGetUsers)
			}
		}
	}

	logger.Log.Info("User Service listening on port 8084...")
	if err := r.Run(":8084"); err != nil {
		log.Fatalf("User Service failed: %v", err)
	}
}
