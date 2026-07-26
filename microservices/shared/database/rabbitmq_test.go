package database

import (
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func TestConnectRabbitMQ(t *testing.T) {
	logger.InitLogger()

	cfg := config.LoadConfig()
	url := cfg.RabbitMQURL
	conn, err := ConnectRabbitMQ(url)
	if err != nil {
		t.Skipf("Skipping RabbitMQ integration test: rabbitmq not reachable: %v", err)
		return
	}
	defer conn.Close()

	if conn == nil {
		t.Fatal("expected rabbitmq connection to be non-nil")
	}
}
