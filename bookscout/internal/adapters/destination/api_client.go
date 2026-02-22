package destination

import (
	"bookscout/internal/adapters/util"
	"bookscout/internal/core/domain/models"
	"bookscout/internal/core/domain/ports"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// Ensure BookSageAPIAdapter implements BookDestination
var _ ports.BookDestination = (*BookSageAPIAdapter)(nil)

type BookSageAPIAdapter struct {
	baseURL string
	client  *http.Client
}

// NewBookSageAPIAdapter creates a new API client for BookSage.
func NewBookSageAPIAdapter(baseURL string) *BookSageAPIAdapter {
	return &BookSageAPIAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			// Reusing the existing retry transport
			Transport: &util.RetryTransport{
				MaxRetries: 3,
				Base:       http.DefaultTransport,
			},
			Timeout: 5 * time.Minute, // Allow longer timeout for large file uploads
		},
	}
}

func (a *BookSageAPIAdapter) Send(ctx context.Context, book models.BookMetadata, content io.Reader) error {
	// Prepare multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file part
	// Infer filename from download URL or ID
	filename := filepath.Base(book.DownloadURL)
	if filename == "" || filename == "." {
		filename = fmt.Sprintf("%s.epub", book.ID)
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, content); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Add metadata part
	metadataJSON, err := json.Marshal(book)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := writer.WriteField("metadata", string(metadataJSON)); err != nil {
		return fmt.Errorf("failed to write metadata field: %w", err)
	}

	// Close writer to finalize content type
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/ingest", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned error status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
