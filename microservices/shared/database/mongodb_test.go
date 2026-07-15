package database

import (
	"context"
	"os"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func TestConnectMongoDB(t *testing.T) {
	logger.InitLogger()

	mongoURL := os.Getenv("MONGO_URL")
	if mongoURL == "" {
		mongoURL = "mongodb://localhost:27017"
	}

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
