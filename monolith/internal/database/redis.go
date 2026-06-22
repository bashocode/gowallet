package database

import (
	"context"
	"time"

	"github.com/bashocode/gowallet/monolith/internal/logger"
	"github.com/redis/go-redis/v9"
)

func ConnectRedis(addr string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	// check connection
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	logger.Log.Info("Successfully connected to Redis!")
	return rdb, nil
}
