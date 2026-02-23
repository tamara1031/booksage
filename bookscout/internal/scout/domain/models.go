package domain

import "time"

type DocumentStatus string

const (
	StatusProcessing DocumentStatus = "PROCESSING"
	StatusCompleted  DocumentStatus = "COMPLETED"
	StatusFailed     DocumentStatus = "FAILED"
)

// Book represents the core data of a book to be ingested.
// This is a Value Object in DDD terms.
type Book struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Author       string    `json:"author"`
	Description  string    `json:"description,omitempty"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	DownloadURL  string    `json:"download_url"`
	Source       string    `json:"source"`
	AddedAt      time.Time `json:"added_at"`
}

// TrackedDocument represents the ingestion state of a specific document.
// This is an Entity.
type TrackedDocument struct {
	ID           string
	FileHash     string
	Status       DocumentStatus
	ErrorMessage string
	UpdatedAt    time.Time
}

// Watermark tracks the progress of the scraper.
type Watermark struct {
	Timestamp int64
}
