package database

import (
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/config"
	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func TestConnectWithRetry(t *testing.T) {
	logger.InitLogger()

	// Try with an invalid DSN that fails immediately (e.g. invalid driver or bad format)
	// But mysql driver open doesn't fail, Ping does. Ping fails with network error.
	// To prevent 30 second delay, we can run this test if DB is available, otherwise skip.
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
