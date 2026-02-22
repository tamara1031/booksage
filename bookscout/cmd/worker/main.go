package main

import (
	"bookscout/internal/adapters/destination"
	"bookscout/internal/adapters/source"
	"bookscout/internal/adapters/tracker"
	"bookscout/internal/config"
	"bookscout/internal/core/service"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.GetConfig()

	// Initialize State Store
	state, err := tracker.NewFileStateStore(cfg.StateFilePath)
	if err != nil {
		log.Fatalf("Failed to initialize state store: %v", err)
	}

	// Initialize Adapters
	// Check source type if needed, but for now defaults to OPDS as per config logic
	src := source.NewOPDSAdapter(
		cfg.OPDSBaseURL,
		cfg.OPDSUsername,
		cfg.OPDSPassword,
		cfg.MaxBookSizeBytes,
		cfg.LogLevel,
	)

	dest := destination.NewBookSageAPIAdapter(cfg.APIBaseURL)

	// Initialize Service
	worker := service.NewWorkerService(cfg, src, dest, state)

	// Context with timeout (1 hour max for batch execution)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Received shutdown signal, cancelling context...")
		cancel()
	}()

	// Run Worker
	log.Println("Starting BookScout Worker...")
	if err := worker.Run(ctx); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
	log.Println("BookScout Worker completed successfully.")
}
