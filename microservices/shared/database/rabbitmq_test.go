package database

import (
	"fmt"
	"os"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func TestConnectRabbitMQ(t *testing.T) {
	logger.InitLogger()

	rabbitmqHost := os.Getenv("RABBITMQ_HOST")
	if rabbitmqHost == "" {
		rabbitmqHost = "localhost"
	}
	rabbitmqPort := os.Getenv("RABBITMQ_PORT")
	if rabbitmqPort == "" {
		rabbitmqPort = "5672"
	}
	rabbitmqUser := os.Getenv("RABBITMQ_USER")
	if rabbitmqUser == "" {
		rabbitmqUser = "guest"
	}
	rabbitmqPass := os.Getenv("RABBITMQ_PASS")
	if rabbitmqPass == "" {
		rabbitmqPass = "guest"
	}

	url := fmt.Sprintf("amqp://%s:%s@%s:%s/", rabbitmqUser, rabbitmqPass, rabbitmqHost, rabbitmqPort)
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
