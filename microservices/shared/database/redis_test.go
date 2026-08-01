package database

import (
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func TestConnectRedis(t *testing.T) {
	logger.InitLogger()

	cfg := config.LoadConfig()
	addr := cfg.RedisAddr
	rdb, err := ConnectRedis(addr)
	if err != nil {
		t.Skipf("Skipping Redis integration test: redis not reachable: %v", err)
		return
	}
	if rdb == nil {
		t.Fatal("expected redis client to be non-nil")
	}
	defer rdb.Close()
}
