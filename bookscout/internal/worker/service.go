package worker

import (
	"bookscout/internal/config"
	"bookscout/internal/domain"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

// BookSource defines the interface for fetching books from a remote source.
type BookSource interface {
	FetchNewBooks(ctx context.Context, lastCheckTimestamp int64) ([]domain.BookMetadata, error)
	DownloadBookContent(ctx context.Context, book domain.BookMetadata) (io.ReadCloser, error)
}

// BookDestination defines the interface for sending books to the ingestion system.
type BookDestination interface {
	Send(ctx context.Context, book domain.BookMetadata, content io.Reader) error
}

// StateStore defines the interface for tracking ingestion progress and preventing duplicates.
type StateStore interface {
	GetWatermark() int64
	IsProcessed(bookID string) bool
	MarkProcessed(bookID string) error
	UpdateWatermark(timestamp int64) error
	Save() error
}

type Service struct {
	cfg   *config.Config
	src   BookSource
	dest  BookDestination
	state StateStore
}

func NewService(
	cfg *config.Config,
	src BookSource,
	dest BookDestination,
	state StateStore,
) *Service {
	return &Service{
		cfg:   cfg,
		src:   src,
		dest:  dest,
		state: state,
	}
}

// Run executes the batch ingestion process.
func (s *Service) Run(ctx context.Context) error {
	since := s.determineWatermark()

	books, err := s.fetchAndFilterBooks(ctx, since)
	if err != nil {
		return err
	}

	if len(books) == 0 {
		return nil
	}

	maxTimestamp, successCount, failCount := s.processBatch(ctx, books, since)

	if err := s.finalizeState(maxTimestamp, since, successCount, failCount); err != nil {
		return err
	}

	return nil
}

func (s *Service) determineWatermark() int64 {
	since := s.cfg.WorkerSinceTimestamp
	if since == 0 {
		since = s.state.GetWatermark()
	}
	log.Printf("Starting Batch Ingestion (since %d [%s])...", since, time.Unix(since, 0).Format(time.RFC3339))
	return since
}

func (s *Service) fetchAndFilterBooks(ctx context.Context, since int64) ([]domain.BookMetadata, error) {
	books, err := s.src.FetchNewBooks(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch books: %w", err)
	}

	if len(books) == 0 {
		log.Println("No new books found. Exiting.")
		return nil, nil
	}

	log.Printf("Found %d potential books. Filtering...", len(books))

	var actionableBooks []domain.BookMetadata
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
		return nil, nil
	}

	// Apply Batch Size Limit
	if s.cfg.WorkerBatchSize > 0 && len(actionableBooks) > s.cfg.WorkerBatchSize {
		log.Printf("Limiting batch to first %d books (out of %d)", s.cfg.WorkerBatchSize, len(actionableBooks))
		actionableBooks = actionableBooks[:s.cfg.WorkerBatchSize]
	}

	return actionableBooks, nil
}

func (s *Service) processBatch(ctx context.Context, books []domain.BookMetadata, since int64) (int64, int, int) {
	log.Printf("Processing %d books...", len(books))

	var (
		wg           sync.WaitGroup
		mu           sync.Mutex
		maxTimestamp = since
		successCount = 0
		failCount    = 0
	)

	// Semaphore to control concurrency
	sem := make(chan struct{}, s.cfg.WorkerConcurrency)

	for _, book := range books {
		wg.Add(1)
		sem <- struct{}{} // Acquire token

		// "Politeness" delay to avoid hammering the source
		if s.cfg.WorkerDelayMS > 0 {
			time.Sleep(time.Duration(s.cfg.WorkerDelayMS) * time.Millisecond)
		}

		go func(b domain.BookMetadata) {
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
	return maxTimestamp, successCount, failCount
}

func (s *Service) finalizeState(maxTimestamp, since int64, success, fail int) error {
	log.Printf("Batch Complete. Success: %d, Failed: %d", success, fail)

	// SAFETY CHECK: Only update watermark if ALL books succeeded.
	// This prevents data loss where an older book fails but a newer one succeeds,
	// causing the watermark to skip the failed book in future runs.
	if fail > 0 {
		log.Printf("Batch contained %d failures. NOT updating watermark to prevent data loss. Successful items are marked individually.", fail)
		// We still save the processed IDs.
	} else if maxTimestamp > since {
		if err := s.state.UpdateWatermark(maxTimestamp); err != nil {
			log.Printf("WARNING: Failed to update watermark in memory: %v", err)
		}
	}

	// Persist state to disk
	if err := s.state.Save(); err != nil {
		return fmt.Errorf("failed to save state file: %w", err)
	}

	log.Printf("State saved. Watermark: %d", s.state.GetWatermark())
	return nil
}

func (s *Service) processBook(ctx context.Context, book domain.BookMetadata) error {
	var err error
	const maxRetries = 3

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			// Logic-level retry with exponential backoff
			sleep := time.Duration(1<<i) * time.Second
			log.Printf("Retrying book '%s' in %s...", book.Title, sleep)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(sleep):
			}
		}

		err = s.processBookAttempt(ctx, book)
		if err == nil {
			log.Printf("Successfully ingested: %s", book.Title)
			return nil
		}
		log.Printf("Attempt %d/%d failed for book '%s': %v", i+1, maxRetries, book.Title, err)
	}
	return fmt.Errorf("after %d attempts: %w", maxRetries, err)
}

func (s *Service) processBookAttempt(ctx context.Context, book domain.BookMetadata) error {
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

	return nil
}
