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
	logger.Info(nil, "starting audit-service...")

	cfg := config.LoadConfig()

	mongoClient, err := database.ConnectMongoDB(cfg.MongoURL)
	if err != nil {
		logger.Fatal(nil, "failed to connect to MongoDB", "error", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			logger.Error(nil, "error disconnecting from MongoDB", "error", err)
		}
	}()

	db := mongoClient.Database("gowallet_audit")
	logger.Info(nil, "connected to MongoDB database: gowallet_audit")

	auditRepo := repository.NewAuditRepository(db)
	auditConsumer := consumer.NewAuditConsumer(cfg.RabbitMQURL, auditRepo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go auditConsumer.Start(ctx)

	logger.Info(nil, "audit-service started successfully")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(nil, "shutting down audit-service...")
}
