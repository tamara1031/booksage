package main

import (
	"bookscout/internal/scout/app"
	"bookscout/internal/scout/config"
	"bookscout/internal/scout/infra/booksage"
	"bookscout/internal/scout/infra/opds"
	"bookscout/internal/scout/infra/sqlite"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bookscout",
	Short: "BookScout is a DDD-organized worker",
	Run: func(cmd *cobra.Command, args []string) {
		run()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	repo, err := sqlite.NewSQLiteRepository(cfg.StateFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()

	scraper := opds.NewOPDSScraper(cfg.OPDSBaseURL, cfg.OPDSUsername, cfg.OPDSPassword, cfg.MaxBookSizeBytes, cfg.LogLevel)
	ingestor := booksage.NewAPIIngestor(cfg.APIBaseURL)

	worker := app.NewScoutWorker(cfg, scraper, ingestor, repo)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetWorkerTimeout())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	if err := worker.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
