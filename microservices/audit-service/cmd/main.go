package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bashocode/gowallet/microservices/audit-service/internal/consumer"
	"github.com/bashocode/gowallet/microservices/audit-service/internal/repository"
	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/database"
	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func main() {
	logger.InitLogger()
	logger.Info(context.Background(), "starting audit-service...")

	cfg := config.LoadConfig()

	mongoClient, err := database.ConnectMongoDB(cfg.MongoURL)
	if err != nil {
		logger.Fatal(context.Background(), "failed to connect to MongoDB", "error", err)
	}

	db := mongoClient.Database("gowallet_audit")
	logger.Info(context.Background(), "connected to MongoDB database: gowallet_audit")

	auditRepo := repository.NewAuditRepository(db)
	auditConsumer := consumer.NewAuditConsumer(cfg.RabbitMQURL, auditRepo)

	ctx, cancel := context.WithCancel(context.Background())

	go auditConsumer.Start(ctx)

	logger.Info(context.Background(), "audit-service started successfully")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(context.Background(), "Shutdown signal received. Starting graceful shutdown...")

	logger.Info(context.Background(), "Stopping consumer workers...")
	cancel()

	logger.Info(context.Background(), "Closing MongoDB connection...")
	if err := mongoClient.Disconnect(context.Background()); err != nil {
		logger.Error(context.Background(), "Failed to disconnect from MongoDB", "error", err)
	}

	logger.Info(context.Background(), "Audit Microservice successfully stopped.")
}
