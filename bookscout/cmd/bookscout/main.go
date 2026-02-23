package main

import (
	"bookscout/internal/client"
	"bookscout/internal/config"
	"bookscout/internal/scraper"
	"bookscout/internal/tracker"
	"bookscout/internal/worker"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bookscout",
	Short: "BookScout is a worker that scrapes books and sends them to BookSage API",
	Run: func(cmd *cobra.Command, args []string) {
		runWorker()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Execution failed: %v", err)
	}
}

func runWorker() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize State Store
	state, err := tracker.NewSQLiteStateStore(cfg.StateFilePath)
	if err != nil {
		log.Fatalf("Failed to initialize state store: %v", err)
	}
	defer state.Close()

	// Initialize Adapters
	src := scraper.NewOPDSAdapter(
		cfg.OPDSBaseURL,
		cfg.OPDSUsername,
		cfg.OPDSPassword,
		cfg.MaxBookSizeBytes,
		cfg.LogLevel,
	)

	dest := client.NewBookSageAPIAdapter(cfg.APIBaseURL)

	// Initialize Service
	svc := worker.NewService(cfg, src, dest, state)

	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetWorkerTimeout())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, cancelling context...")
		cancel()
	}()

	// Run Worker
	log.Println("Starting BookScout Worker...")
	if err := svc.Run(ctx); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
	log.Println("BookScout Worker completed successfully.")
}
