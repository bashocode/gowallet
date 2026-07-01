package database

import (
	"database/sql"
	"time"

	"github.com/bashocode/gowallet/microservices/shared/logger"
	_ "github.com/go-sql-driver/mysql"
)

func ConnectWithRetry(dsn string) (*sql.DB, error) {
	var db *sql.DB
	var err error
	maxRetries := 5
	backoff := 2 * time.Second

	for i := 1; i <= maxRetries; i++ {
		logger.Log.Info("Connecting to database", "attempt", i, "max_retries", maxRetries)

		db, err = sql.Open("mysql", dsn)
		if err == nil {
			// do ping for make sure connection is alive
			err = db.Ping()
			if err == nil {
				logger.Log.Info("Successfully connected to database")

				// setup connection pool properties
				db.SetMaxOpenConns(25)
				db.SetMaxIdleConns(25)
				db.SetConnMaxLifetime(5 * time.Minute)

				return db, nil
			}
		}

		logger.Log.Info("Database connection failed, retrying", "error", err, "backoff", backoff)
		time.Sleep(backoff)

		// double backoff for waiting to next retry, or exponential backoff
		backoff *= 2
	}

	return nil, err
}
