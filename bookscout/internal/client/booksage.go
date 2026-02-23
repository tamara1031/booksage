package client

import (
	"bookscout/internal/domain"
	"bookscout/internal/pkg/httpclient"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func (a *BookSageAPIAdapter) Send(ctx context.Context, book domain.BookMetadata, content io.Reader) (string, error) {
	// We need the hash for tracking, so we'll buffer the content or hash it on the fly if streaming.
	// Documentation says API already does this, but since we can't change API to return hash,
	// we calculate it here.

	var buf bytes.Buffer
	tee := io.TeeReader(content, &buf)

	hash := sha256.New()
	if _, err := io.Copy(hash, tee); err != nil {
		return "", fmt.Errorf("failed to hash content: %w", err)
	}
	fileHash := hex.EncodeToString(hash.Sum(nil))

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()

		metadataJSON, _ := json.Marshal(book)
		_ = writer.WriteField("metadata", string(metadataJSON))

		filename := filepath.Base(book.DownloadURL)
		if filename == "" || filename == "." {
			filename = fmt.Sprintf("%s.epub", book.ID)
		}

		part, _ := writer.CreateFormFile("file", filename)
		_, _ = io.Copy(part, &buf)
		_ = writer.Close()
	}()

	url := fmt.Sprintf("%s/ingest", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, pr)
	if err != nil {
		pr.Close()
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("API error %d", resp.StatusCode)
	}

	return fileHash, nil
}

type IngestStatusResponse struct {
	SagaID int64  `json:"saga_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func (a *BookSageAPIAdapter) GetStatus(ctx context.Context, sagaID int64) (string, string, error) {
	url := fmt.Sprintf("%s/ingest/status/%d", a.baseURL, sagaID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("get status request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "NOT_FOUND", "", nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result IngestStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decode status response: %w", err)
	}

	return result.Status, result.Error, nil
}

func (a *BookSageAPIAdapter) GetStatusByHash(ctx context.Context, fileHash string) (string, string, error) {
	url := fmt.Sprintf("%s/ingest/status?hash=%s", a.baseURL, fileHash)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "NOT_FOUND", "", nil
	}

	var result struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	return result.Status, result.Error, nil
}
