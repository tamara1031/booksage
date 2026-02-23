package client

import (
	"bookscout/internal/domain"
	"bookscout/internal/pkg/httpclient"
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
			Transport: &httpclient.RetryTransport{
				MaxRetries: 3,
				Base:       http.DefaultTransport,
			},
			Timeout: 5 * time.Minute, // Allow longer timeout for large file uploads
		},
	}
}

func (a *BookSageAPIAdapter) Send(ctx context.Context, book domain.BookMetadata, content io.Reader) error {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Go routine to write multipart data
	go func() {
		// Ensure pipe is closed last to signal EOF or error to the reader
		defer pw.Close()

		// Write Metadata
		metadataJSON, err := json.Marshal(book)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("marshal metadata: %w", err))
			return
		}
		if err := writer.WriteField("metadata", string(metadataJSON)); err != nil {
			pw.CloseWithError(fmt.Errorf("write metadata: %w", err))
			return
		}

		// Write File
		filename := filepath.Base(book.DownloadURL)
		if filename == "" || filename == "." {
			filename = fmt.Sprintf("%s.epub", book.ID)
		}

		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("create form file: %w", err))
			return
		}

		if _, err := io.Copy(part, content); err != nil {
			pw.CloseWithError(fmt.Errorf("copy content: %w", err))
			return
		}

        // Write trailing boundary
        if err := writer.Close(); err != nil {
             pw.CloseWithError(fmt.Errorf("close multipart writer: %w", err))
             return
        }
	}()

	url := fmt.Sprintf("%s/ingest", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, pr)
	if err != nil {
		// Ensure we close the pipe if request creation fails, to unblock the writer if it started (unlikely but safe)
		pr.Close()
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
