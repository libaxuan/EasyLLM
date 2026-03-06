package main

import (
	"easyllm/config"
	"easyllm/internal/server"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if present
	godotenv.Load()

	// Load configuration
	cfg := config.Load()

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.App.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize and run application
	app, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
