package ports

import (
	"bookscout/internal/core/domain/models"
	"context"
	"io"
)

// BookSource defines the interface for fetching books from a remote source (e.g., OPDS).
type BookSource interface {
	// FetchNewBooks returns a list of books that are strictly newer than the given timestamp.
	FetchNewBooks(ctx context.Context, lastCheckTimestamp int64) ([]models.BookMetadata, error)
	// DownloadBookContent downloads the binary content of a book (e.g., PDF/EPUB).
	DownloadBookContent(ctx context.Context, book models.BookMetadata) (io.ReadCloser, error)
}

// BookDestination defines the interface for sending books to the ingestion system.
type BookDestination interface {
	// Send uploads the book content and metadata to the BookSage API.
	Send(ctx context.Context, book models.BookMetadata, content io.Reader) error
}

// StateStore defines the interface for tracking ingestion progress and preventing duplicates.
type StateStore interface {
	// GetWatermark returns the timestamp of the last successfully processed batch.
	GetWatermark() int64
	// IsProcessed checks if a specific book ID has already been processed (e.g., via hash set).
	IsProcessed(bookID string) bool
	// MarkProcessed records a book ID as processed.
	MarkProcessed(bookID string) error
	// UpdateWatermark updates the global high-water mark.
	UpdateWatermark(timestamp int64) error
	// Save persists the current state (watermark + processed IDs) to storage.
	Save() error
}
