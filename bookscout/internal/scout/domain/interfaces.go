package domain

import (
	"context"
	"io"
	"time"
)

// Scraper defines the domain service for fetching books from external catalogs.
type Scraper interface {
	Scrape(ctx context.Context, since time.Time) ([]Book, error)
	DownloadContent(ctx context.Context, book Book) (io.ReadCloser, error)
}

// Ingestor defines the domain service for sending books to the target system.
type Ingestor interface {
	Ingest(ctx context.Context, book Book, content io.Reader) (string, error)
	GetStatusByHash(ctx context.Context, fileHash string) (string, string, error)
}

// StateRepository defines the persistence interface for tracking ingestion progress.
type StateRepository interface {
	GetWatermark(ctx context.Context) (int64, error)
	UpdateWatermark(ctx context.Context, timestamp int64) error

	IsProcessed(ctx context.Context, bookID string) (bool, error)
	GetStatus(ctx context.Context, bookID string) (DocumentStatus, bool, error)
	GetProcessingDocuments(ctx context.Context) ([]TrackedDocument, error)

	RecordIngestion(ctx context.Context, bookID string, fileHash string) error
	UpdateStatusByHash(ctx context.Context, fileHash string, status DocumentStatus, errMsg string) error
}
