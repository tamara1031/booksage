package main

import (
	"log"

	"github.com/booksage/booksage-api/internal/platform/conf"
	"github.com/booksage/booksage-api/internal/platform/server"
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
