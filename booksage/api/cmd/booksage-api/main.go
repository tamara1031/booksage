package main

import (
	"log"

	"github.com/booksage/booksage-api/internal/conf"
	"github.com/booksage/booksage-api/internal/infrastructure/server"
)

func main() {
	log.Println("Starting BookSage API Orchestrator...")

	// Load Configuration
	cfg := conf.Load()

	srv := server.New(cfg)
	if err := srv.Run(); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}
