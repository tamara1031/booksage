package main

import (
	"bookscout/internal/adapters/util"
	"bookscout/internal/config"
	"bookscout/internal/core/domain/models"
	"bookscout/internal/core/domain/ports"
	"bookscout/internal/core/service"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

func main() {
	cfg := config.GetConfig()
	source := service.CreateBookSource(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := Run(ctx, cfg, source); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}

// Run executes the worker batch ingestion. Exposed for testing.
func Run(ctx context.Context, cfg *config.Config, source ports.BookDataSource) error {
	state, err := util.NewFileStateStore(cfg.StateFilePath)
	if err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}

	ingestSvc := service.CreateIngestService(cfg, source, state)

	// Determine since timestamp. Config takes precedence over state watermark.
	since := cfg.WorkerSinceTimestamp
	if since == 0 {
		since = state.GetWatermark()
	}

	sinceTime := time.Unix(since, 0)
	log.Printf("Starting Go Worker (Batch Execution, since %d [%s])...", since, sinceTime.Format(time.RFC3339))

	books, err := source.FetchNewBooks(ctx, since)
	if err != nil {
		return fmt.Errorf("error fetching books: %w", err)
	}

	if len(books) == 0 {
		log.Println("No new books found. Exiting.")
		return nil
	}

	if cfg.WorkerBatchSize > 0 && len(books) > cfg.WorkerBatchSize {
		log.Printf("Limiting ingestion to first %d books (found %d total)", cfg.WorkerBatchSize, len(books))
		books = books[:cfg.WorkerBatchSize]
	}

	log.Printf("Processing %d books. Starting concurrent ingestion...", len(books))

	var maxTimestamp int64 = since
	var tsMu sync.Mutex

	sem := make(chan struct{}, cfg.WorkerConcurrency)
	var wg sync.WaitGroup
	for _, book := range books {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		// Politeness delay - stagger the starts
		if cfg.WorkerDelayMS > 0 {
			time.Sleep(time.Duration(cfg.WorkerDelayMS) * time.Millisecond)
		}

		go func(b models.BookMetadata) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			if err := ingestSvc.IngestBook(ctx, b); err != nil {
				log.Printf("Error processing %s: %v", b.Title, err)
				return
			}

			// Track max timestamp for watermark
			tsMu.Lock()
			ts := b.AddedAt.Unix()
			if ts > maxTimestamp {
				maxTimestamp = ts
			}
			tsMu.Unlock()
		}(book)
	}

	wg.Wait()

	// Update watermark if we made progress
	if maxTimestamp > since {
		if err := state.UpdateWatermark(maxTimestamp); err != nil {
			log.Printf("Warning: failed to update watermark: %v", err)
		} else {
			log.Printf("Watermark updated to: %d (%s)", maxTimestamp, time.Unix(maxTimestamp, 0).Format(time.RFC3339))
		}
	}

	log.Println("Batch ingestion complete. Exiting.")
	return nil
}
