package database

import (
	"github.com/bashocode/gowallet/microservices/shared/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

func ConnectRabbitMQ(url string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	logger.Log.Info("Successfully connected to RabbitMQ!")
	return conn, nil
}
