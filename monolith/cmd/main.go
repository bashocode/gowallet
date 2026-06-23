package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/bashocode/gowallet/monolith/docs"
	"github.com/bashocode/gowallet/monolith/internal/config"
	"github.com/bashocode/gowallet/monolith/internal/database"
	"github.com/bashocode/gowallet/monolith/internal/email"
	ledgerRepository "github.com/bashocode/gowallet/monolith/internal/ledger/repository"
	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/bashocode/gowallet/monolith/internal/middleware"
	otpRepository "github.com/bashocode/gowallet/monolith/internal/otp/repository"
	"github.com/bashocode/gowallet/monolith/internal/scheduler"
	txHandler "github.com/bashocode/gowallet/monolith/internal/transaction/handler"
	txRepository "github.com/bashocode/gowallet/monolith/internal/transaction/repository"
	txService "github.com/bashocode/gowallet/monolith/internal/transaction/service"
	userHandler "github.com/bashocode/gowallet/monolith/internal/user/handler"
	userRepository "github.com/bashocode/gowallet/monolith/internal/user/repository"
	userService "github.com/bashocode/gowallet/monolith/internal/user/service"
	walletHandler "github.com/bashocode/gowallet/monolith/internal/wallet/handler"
	walletRepository "github.com/bashocode/gowallet/monolith/internal/wallet/repository"
	walletService "github.com/bashocode/gowallet/monolith/internal/wallet/service"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title			GoWallet Monolith API
// @version			1.0
// @description		API Documentation for GoWallet
// @termOfService	http://swagger.io/terms/

// @contact.name	API Support
// @contact.email	bashocode@gmail.com

// @host			localhost:8080
// @basepath		/api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer <your_token>" to authenticate.
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

	// connect to redis
	rdb, err := database.ConnectRedis(cfg.RedisAddr)
	if err != nil {
		logger.Log.Error("Critical Error: Could not connect to Redis", "error", err)
	}
	defer rdb.Close()

	// initiate email sender & otp repository
	emailSender := email.NewSMTPEmailSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)
	otpRepo := otpRepository.NewMySQLOTPRRepository(db)

	// 1. initiate layer
	uRepo := userRepository.NewMySQLUserRepository(db)
	wRepo := walletRepository.NewMySQLWalletRepository(db)
	tRepo := txRepository.NewMySQLTransactionRepository(db)
	lRepo := ledgerRepository.NewMysqlLedgerRepository(db)

	// inject db to user service for transaction
	uSvc := userService.NewUserService(db, rdb, uRepo, wRepo, otpRepo, emailSender)
	wSvc := walletService.NewWalletService(wRepo, rdb)
	tSvc := txService.NewTransactionService(db, rdb, tRepo, uRepo, wRepo, lRepo)

	uHandler := userHandler.NewUserHandler(uSvc)
	wHandler := walletHandler.NewWalletHandler(wSvc)
	tHandler := txHandler.NewTransactionHandler(tSvc)

	cronSched := scheduler.NewScheduler(db, wRepo, lRepo)
	cronSched.Start()

	// 2. setup gin router
	r := gin.New()
	r.Use(gin.Recovery())
	// Register global error handling middleware
	r.Use(middleware.ErrorHandler())

	// apply global rate limiter max 60 request per minutes per ip
	r.Use(middleware.RateLimiter(rdb, 60, time.Minute))

	// register the swagger api
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Route grouping
	v1 := r.Group("/api/v1")
	{
		// Public routes
		v1.POST("/users/register", uHandler.Register)
		v1.POST("/users/login", uHandler.Login)

		// Protected routes (requires valid JWT token)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		{
			protected.GET("/users/me", uHandler.GetProfileMe)
			protected.POST("/users/avatar", uHandler.UploadAvatar)
			protected.PUT("/users/:id", uHandler.UpdateProfile)
			protected.GET("/users/:id", uHandler.GetProfile)
			protected.DELETE("/users/me", uHandler.DeleteAccount)
			protected.POST("/users/logout", uHandler.Logout)
			protected.POST("/users/verify-email", uHandler.VerifyEmail)

			protected.GET("/wallets/me", wHandler.GetMyWallet)

			protected.POST("/transactions/transfer", tHandler.Transfer)
			protected.GET("/transactions/history", tHandler.GetHistory)
		}
	}

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	// run server in separate goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Error("Server failed to run", "error", err)
		}
	}()

	// start server
	logger.Log.Info("Server running on port 8080....")

	// graceful shutdown - wait for signal from os
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Server shutting down gracefully...")

	// give 10 seconds to complet in-flight requests
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Log.Error("Server forced to shutdown", "error", err)
	}

	// stop scheduler after http server shutdown
	cronSched.Stop()

	logger.Log.Info("Server exited gracefully")
}
