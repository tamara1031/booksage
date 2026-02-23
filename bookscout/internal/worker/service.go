package worker

import (
	"bookscout/internal/config"
	"bookscout/internal/domain"
	"bookscout/internal/tracker"
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
	Send(ctx context.Context, book domain.BookMetadata, content io.Reader) (string, error)
	GetStatusByHash(ctx context.Context, fileHash string) (string, string, error)
}

// StateStore defines the interface for tracking ingestion progress and preventing duplicates.
type StateStore interface {
	GetWatermark() int64
	IsProcessed(bookID string) bool
	UpdateWatermark(timestamp int64) error

	// Async tracking methods
	GetProcessingDocuments() ([]tracker.TrackedDocument, error)
	UpdateStatusByHash(fileHash string, status tracker.DocumentStatus, errMsg string) error
	RecordIngestion(bookID string, fileHash string) error
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
	// Phase 1: Status Sync
	if err := s.syncStatus(ctx); err != nil {
		log.Printf("WARNING: Status sync failed: %v", err)
	}

	// Phase 2: Scrape & Ingest
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

func (s *Service) syncStatus(ctx context.Context) error {
	log.Println("Phase 1: Syncing statuses of processing documents...")
	docs, err := s.state.GetProcessingDocuments()
	if err != nil {
		return fmt.Errorf("failed to get processing documents: %w", err)
	}

	for _, d := range docs {
		status, errMsg, err := s.dest.GetStatusByHash(ctx, d.FileHash)
		if err != nil {
			log.Printf("Failed to get status for hash %s (book %s): %v", d.FileHash, d.ID, err)
			continue
		}

		switch status {
		case "completed":
			log.Printf("Hash %s COMPLETED for book %s", d.FileHash, d.ID)
			if err := s.state.UpdateStatusByHash(d.FileHash, tracker.StatusCompleted, ""); err != nil {
				log.Printf("WARNING: Failed to update status to COMPLETED for hash %s: %v", d.FileHash, err)
			}
		case "failed":
			log.Printf("Hash %s FAILED for book %s: %s", d.FileHash, d.ID, errMsg)
			if err := s.state.UpdateStatusByHash(d.FileHash, tracker.StatusFailed, errMsg); err != nil {
				log.Printf("WARNING: Failed to update status to FAILED for hash %s: %v", d.FileHash, err)
			}
		case "NOT_FOUND":
			log.Printf("Hash %s NOT FOUND in API. Still waiting or maybe failed in early stage.", d.FileHash)
		default:
			log.Printf("Hash %s is still in state: %s", d.FileHash, status)
		}
	}
	return nil
}

func (s *Service) determineWatermark() int64 {
	since := s.cfg.WorkerSinceTimestamp
	if since == 0 {
		since = s.state.GetWatermark()
	}
	log.Printf("Phase 2: Starting Batch Ingestion (since %d [%s])...", since, time.Unix(since, 0).Format(time.RFC3339))
	return since
}

func (s *Service) fetchAndFilterBooks(ctx context.Context, since int64) ([]domain.BookMetadata, error) {
	books, err := s.src.FetchNewBooks(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch books: %w", err)
	}

	if len(books) == 0 {
		log.Println("No new books found.")
		return nil, nil
	}

	log.Printf("Found %d potential books. Filtering...", len(books))

	var actionableBooks []domain.BookMetadata
	for _, b := range books {
		// Skip if already processed or processing
		if s.state.IsProcessed(b.ID) {
			log.Printf("Skipping already processed book: %s (ID: %s)", b.Title, b.ID)
			continue
		}
		actionableBooks = append(actionableBooks, b)
	}

	if len(actionableBooks) == 0 {
		log.Println("All books have already been processed or are in progress.")
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

	sem := make(chan struct{}, s.cfg.WorkerConcurrency)

	for _, book := range books {
		wg.Add(1)
		sem <- struct{}{}

		if s.cfg.WorkerDelayMS > 0 {
			time.Sleep(time.Duration(s.cfg.WorkerDelayMS) * time.Millisecond)
		}

		go func(b domain.BookMetadata) {
			defer wg.Done()
			defer func() { <-sem }()

			fileHash, err := s.processBook(ctx, b)
			if err != nil {
				log.Printf("ERROR processing book '%s' (%s): %v", b.Title, b.ID, err)
				mu.Lock()
				failCount++
				mu.Unlock()
				return
			}

			mu.Lock()
			successCount++
			if err := s.state.RecordIngestion(b.ID, fileHash); err != nil {
				log.Printf("WARNING: Failed to record ingestion for book %s (Hash: %s): %v", b.ID, fileHash, err)
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
	log.Printf("Batch Complete. Ingestion Requests Sent: %d, Failed: %d", success, fail)

	if fail > 0 {
		log.Printf("Batch contained %d failures. NOT updating watermark.", fail)
	} else if maxTimestamp > since {
		if err := s.state.UpdateWatermark(maxTimestamp); err != nil {
			log.Printf("WARNING: Failed to update watermark: %v", err)
		}
	}

	log.Printf("Current Watermark: %d", s.state.GetWatermark())
	return nil
}

func (s *Service) processBook(ctx context.Context, book domain.BookMetadata) (string, error) {
	var err error
	var fileHash string
	const maxRetries = 3

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			sleep := time.Duration(1<<i) * time.Second
			log.Printf("Retrying book '%s' in %s...", book.Title, sleep)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(sleep):
			}
		}

		fileHash, err = s.processBookAttempt(ctx, book)
		if err == nil {
			log.Printf("Successfully sent for ingestion: %s (Hash: %s)", book.Title, fileHash)
			return fileHash, nil
		}
		log.Printf("Attempt %d/%d failed for book '%s': %v", i+1, maxRetries, book.Title, err)
	}
	return "", fmt.Errorf("after %d attempts: %w", maxRetries, err)
}

func (s *Service) processBookAttempt(ctx context.Context, book domain.BookMetadata) (string, error) {
	// A. Download
	content, err := s.src.DownloadBookContent(ctx, book)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer content.Close()

	// B. Send to Destination
	fileHash, err := s.dest.Send(ctx, book, content)
	if err != nil {
		return "", fmt.Errorf("send failed: %w", err)
	}

	return fileHash, nil
}
