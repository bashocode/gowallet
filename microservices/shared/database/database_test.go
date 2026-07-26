package database

import (
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func TestConnectWithRetry(t *testing.T) {
	logger.InitLogger()

	cfg := config.LoadConfig()
	dsn := cfg.DBDSN
	db, err := ConnectWithRetry(dsn)
	if err != nil {
		t.Skipf("Skipping database integration test: database not reachable: %v", err)
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Errorf("expected database to be pingable, got: %v", err)
	}
}
