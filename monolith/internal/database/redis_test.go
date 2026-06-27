package database

import (
	"os"
	"testing"

	"github.com/bashocode/gowallet/monolith/internal/logger"
)

func TestConnectRedis(t *testing.T) {
	logger.InitLogger()

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	addr := redisHost + ":" + redisPort
	rdb, err := ConnectRedis(addr)
	if err != nil {
		t.Skipf("Skipping Redis integration test: redis not reachable: %v", err)
		return
	}
	defer rdb.Close()

	if rdb == nil {
		t.Fatal("expected redis client to be non-nil")
	}
}
