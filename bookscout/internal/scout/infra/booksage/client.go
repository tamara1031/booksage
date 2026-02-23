package booksage

import (
	"bookscout/internal/pkg/httpclient"
	"bookscout/internal/scout/domain"
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

type APIIngestor struct {
	baseURL string
	client  *http.Client
}

func NewAPIIngestor(baseURL string) *APIIngestor {
	return &APIIngestor{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Transport: &httpclient.RetryTransport{
				MaxRetries: 3,
				Base:       http.DefaultTransport,
			},
			Timeout: 5 * time.Minute,
		},
	}
}

// Renaming for consistency with interfaces
type APIIngester = APIIngestor

func (a *APIIngestor) Ingest(ctx context.Context, book domain.Book, content io.Reader) (string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		md, err := json.Marshal(book)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if err := writer.WriteField("metadata", string(md)); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		filename := filepath.Base(book.DownloadURL)
		if filename == "" || filename == "." {
			filename = fmt.Sprintf("%s.epub", book.ID)
		}
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, content); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
	}()

	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/ingest", pr)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var result struct {
		Hash string `json:"hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Hash == "" {
		return "", fmt.Errorf("API response missing hash")
	}

	return result.Hash, nil
}

func (a *APIIngestor) GetStatusByHash(ctx context.Context, fileHash string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/ingest/status?hash="+fileHash, nil)
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
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	return result.Status, result.Error, nil
}
