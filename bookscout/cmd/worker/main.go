package main

import (
	"bookscout/internal/config"
	"bookscout/internal/core/domain/models"
	"bookscout/internal/core/domain/ports"
	"bookscout/internal/core/service"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
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

	// Determine since timestamp
	since := cfg.WorkerSinceTimestamp
	sinceTime := time.Unix(since, 0)
	log.Printf("Starting Go Worker (Batch Execution, since %d [%s])...", since, sinceTime.Format(time.RFC3339))
	log.Printf("Debug Config: WorkerSinceTimestamp=%d, WorkerBatchSize=%d", cfg.WorkerSinceTimestamp, cfg.WorkerBatchSize)

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

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

	concurrency := cfg.WorkerConcurrency
	log.Printf("Concurrency limit set to: %d", concurrency)

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, book := range books {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		go func(b models.BookMetadata) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore
			log.Printf("Processing: %s", b.Title)

			content, err := source.DownloadBookContent(ctx, b)
			if err != nil {
				log.Printf("Error downloading book %s: %v", b.ID, err)
				return
			}

			// Check if already registered
			registered, err := isRegistered(cfg.APIBaseURL, b.ID)
			if err != nil {
				log.Printf("Warning: Failed to check registration for %s: %v", b.ID, err)
			}
			if registered {
				log.Printf("Skipping already registered book: %s", b.Title)
				return
			}

			if err := ingestToAPI(cfg.APIBaseURL, b, content); err != nil {
				log.Printf("Error ingesting book %s to API: %v", b.ID, err)
				return
			}
			log.Printf("Successfully queued for ingestion: %s", b.Title)
		}(book)
	}

	wg.Wait()
	log.Println("Batch ingestion complete. Exiting.")
	return nil
}

// ingestToAPI sends book metadata and content to the API server. Exposed for testing.
func ingestToAPI(baseURL string, book models.BookMetadata, content []byte) error {
	url := fmt.Sprintf("%s/ingest", baseURL)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("file", book.ID+".epub") // Using ID as filename for consistency
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, bytes.NewReader(content)); err != nil {
		return err
	}

	// Generate standard JSON metadata according to API.md
	metadata := map[string]string{
		"title":         book.Title,
		"author":        book.Author,
		"description":   book.Description,
		"thumbnail_url": book.ThumbnailURL,
	}
	metadataJSON, _ := json.Marshal(metadata)

	// Add metadata field
	_ = w.WriteField("metadata", string(metadataJSON))

	w.Close()

	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// isRegistered checks if the document already exists in the destination API.
func isRegistered(baseURL string, docID string) (bool, error) {
	url := fmt.Sprintf("%s/documents/%s", baseURL, docID)
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}
