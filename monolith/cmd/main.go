package main

import (
	"log"

	"github.com/bashocode/gowallet/monolith/internal/config"
	"github.com/bashocode/gowallet/monolith/internal/database"
)

func main() {
	log.Println("Starting Monolith Wallet Application...")

	// 1. load configuration
	cfg := config.LoadConfig()

	// 2. connect to database with retry
	db, err := database.ConnectWithRetry(cfg.DBDSN)
	if err != nil {
		log.Fatalf("Critical Error: Could not connect to database after retries: %v", err)
	}
	defer db.Close()

	log.Println("Application successfully initialized...")
}
