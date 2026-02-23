package booksage

import (
	"bookscout/internal/pkg/httpclient"
	"bookscout/internal/scout/domain"
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
	var buf bytes.Buffer
	tee := io.TeeReader(content, &buf)

	hash := sha256.New()
	if _, err := io.Copy(hash, tee); err != nil {
		return "", err
	}
	fileHash := hex.EncodeToString(hash.Sum(nil))

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		md, _ := json.Marshal(book)
		_ = writer.WriteField("metadata", string(md))
		filename := filepath.Base(book.DownloadURL)
		if filename == "" || filename == "." {
			filename = fmt.Sprintf("%s.epub", book.ID)
		}
		part, _ := writer.CreateFormFile("file", filename)
		_, _ = io.Copy(part, &buf)
		_ = writer.Close()
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

	return fileHash, nil
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
