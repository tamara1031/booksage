package service

import (
	"bookscout/internal/config"
	"bookscout/internal/core/domain/models"
	"bookscout/internal/core/domain/ports"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type WorkerService struct {
	cfg   *config.Config
	src   ports.BookSource
	dest  ports.BookDestination
	state ports.StateStore
}

func NewWorkerService(
	cfg *config.Config,
	src ports.BookSource,
	dest ports.BookDestination,
	state ports.StateStore,
) *WorkerService {
	return &WorkerService{
		cfg:   cfg,
		src:   src,
		dest:  dest,
		state: state,
	}
}

// Run executes the batch ingestion process.
func (s *WorkerService) Run(ctx context.Context) error {
	// 1. Determine the starting point (watermark)
	// Config takes precedence if set to a non-zero value, otherwise use state
	since := s.cfg.WorkerSinceTimestamp
	if since == 0 {
		since = s.state.GetWatermark()
	}

	sinceTime := time.Unix(since, 0)
	log.Printf("Starting Batch Ingestion (since %d [%s])...", since, sinceTime.Format(time.RFC3339))

	// 2. Fetch new books from the source
	// Note: The source adapter is responsible for filtering by timestamp if possible,
	// but we will also filter by processed ID to ensure idempotency.
	books, err := s.src.FetchNewBooks(ctx, since)
	if err != nil {
		return fmt.Errorf("failed to fetch books: %w", err)
	}

	if len(books) == 0 {
		log.Println("No new books found. Exiting.")
		return nil
	}

	log.Printf("Found %d potential books. Filtering...", len(books))

	// 3. Filter books
	var actionableBooks []models.BookMetadata
	for _, b := range books {
		// Skip if already processed (idempotency check)
		if s.state.IsProcessed(b.ID) {
			log.Printf("Skipping already processed book: %s (ID: %s)", b.Title, b.ID)
			continue
		}
		actionableBooks = append(actionableBooks, b)
	}

	if len(actionableBooks) == 0 {
		log.Println("All books have already been processed.")
		return nil
	}

	// 4. Apply Batch Size Limit
	if s.cfg.WorkerBatchSize > 0 && len(actionableBooks) > s.cfg.WorkerBatchSize {
		log.Printf("Limiting batch to first %d books (out of %d)", s.cfg.WorkerBatchSize, len(actionableBooks))
		actionableBooks = actionableBooks[:s.cfg.WorkerBatchSize]
	}

	log.Printf("Processing %d books...", len(actionableBooks))

	// 5. Process concurrently
	var (
		wg           sync.WaitGroup
		mu           sync.Mutex
		maxTimestamp = since
		successCount = 0
		failCount    = 0
	)

	// Semaphore to control concurrency
	sem := make(chan struct{}, s.cfg.WorkerConcurrency)

	for _, book := range actionableBooks {
		wg.Add(1)
		sem <- struct{}{} // Acquire token

		// "Politeness" delay to avoid hammering the source
		if s.cfg.WorkerDelayMS > 0 {
			time.Sleep(time.Duration(s.cfg.WorkerDelayMS) * time.Millisecond)
		}

		go func(b models.BookMetadata) {
			defer wg.Done()
			defer func() { <-sem }() // Release token

			if err := s.processBook(ctx, b); err != nil {
				log.Printf("ERROR processing book '%s' (%s): %v", b.Title, b.ID, err)
				mu.Lock()
				failCount++
				mu.Unlock()
				return
			}

			// On success, update state and watermark tracking
			mu.Lock()
			successCount++
			if err := s.state.MarkProcessed(b.ID); err != nil {
				log.Printf("WARNING: Failed to mark book %s as processed: %v", b.ID, err)
			}
			if b.AddedAt.Unix() > maxTimestamp {
				maxTimestamp = b.AddedAt.Unix()
			}
			mu.Unlock()
		}(book)
	}

	wg.Wait()

	// 6. Finalize State
	log.Printf("Batch Complete. Success: %d, Failed: %d", successCount, failCount)

	// Only update watermark if we processed something successfully and the new timestamp is greater
	if maxTimestamp > since {
		if err := s.state.UpdateWatermark(maxTimestamp); err != nil {
			log.Printf("WARNING: Failed to update watermark in memory: %v", err)
		}
	}

	// Persist state to disk
	if err := s.state.Save(); err != nil {
		return fmt.Errorf("failed to save state file: %w", err)
	}

	log.Printf("State saved. New Watermark: %d", s.state.GetWatermark())
	return nil
}

func (s *WorkerService) processBook(ctx context.Context, book models.BookMetadata) error {
	// A. Download
	content, err := s.src.DownloadBookContent(ctx, book)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer content.Close()

	// B. Send to Destination
	if err := s.dest.Send(ctx, book, content); err != nil {
		return fmt.Errorf("send failed: %w", err)
	}

	log.Printf("Successfully ingested: %s", book.Title)
	return nil
}
