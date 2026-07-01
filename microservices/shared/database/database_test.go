package database

import (
	"os"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/logger"
)

func TestConnectWithRetry(t *testing.T) {
	logger.InitLogger()

	// Try with an invalid DSN that fails immediately (e.g. invalid driver or bad format)
	// But mysql driver open doesn't fail, Ping does. Ping fails with network error.
	// To prevent 30 second delay, we can run this test if DB is available, otherwise skip.
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "gowallet_user"
	}
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "gowallet_password"
	}
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "3306"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "gowallet"
	}

	dsn := dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?parseTime=true"
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
