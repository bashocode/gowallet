package database

import (
	"context"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func TestConnectMongoDB(t *testing.T) {
	logger.InitLogger()

	cfg := config.LoadConfig()
	mongoURL := cfg.MongoURL

	client, err := ConnectMongoDB(mongoURL)
	if err != nil {
		t.Skipf("Skipping MongoDB integration test: mongodb not reachable: %v", err)
		return
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("failed to disconnect from MongoDB: %v", err)
		}
	}()

	ctx := context.Background()
	if err := client.Ping(ctx, nil); err != nil {
		t.Errorf("expected MongoDB to be pingable, got: %v", err)
	}
}
