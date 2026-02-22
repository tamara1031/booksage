package source

import (
	"bookscout/internal/core/domain/models"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOPDSAdapter_FetchNewBooks_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Mock OPDS Feed</title>
  <entry>
    <title>Test Book 1</title>
    <id>urn:uuid:12345</id>
    <updated>2026-02-20T12:00:00Z</updated>
    <author><name>John Doe</name></author>
    <summary>A test book description.</summary>
    <link rel="http://opds-spec.org/acquisition/open-access" href="http://example.com/download/book1.epub" type="application/epub+zip"/>
    <link rel="http://opds-spec.org/image/thumbnail" href="http://example.com/thumb.jpg" type="image/jpeg"/>
  </entry>
</feed>`)
	}))
	defer server.Close()

	adapter := &OPDSAdapter{
		catalogURL: server.URL,
		client:     &http.Client{},
	}

	books, err := adapter.FetchNewBooks(context.Background(), 0)
	if err != nil {
		t.Fatalf("FetchNewBooks failed: %v", err)
	}
	if len(books) != 1 {
		t.Errorf("Expected 1 book, got %d", len(books))
	}
	if books[0].Description != "A test book description." {
		t.Errorf("Expected description, got '%s'", books[0].Description)
	}
	if books[0].ThumbnailURL != "http://example.com/thumb.jpg" {
		t.Errorf("Expected thumbnail, got '%s'", books[0].ThumbnailURL)
	}
}

func TestOPDSAdapter_FetchNewBooks_Pagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>Book Page 2</title>
    <id>urn:uuid:p2</id>
    <link rel="http://opds-spec.org/acquisition" href="http://example.com/p2.epub"/>
  </entry>
</feed>`)
		} else {
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>Book Page 1</title>
    <id>urn:uuid:p1</id>
    <link rel="http://opds-spec.org/acquisition" href="http://example.com/p1.epub"/>
  </entry>
  <link rel="next" href="%s?page=2"/>
</feed>`, r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := &OPDSAdapter{
		catalogURL: server.URL,
		client:     &http.Client{},
	}

	books, err := adapter.FetchNewBooks(context.Background(), 0)
	if err != nil {
		t.Fatalf("FetchNewBooks failed: %v", err)
	}
	if len(books) != 2 {
		t.Errorf("Expected 2 books across pages, got %d", len(books))
	}
}

func TestOPDSAdapter_FetchNewBooks_Traversal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		if r.URL.Path == "/recent" {
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>Traversed Book</title>
    <id>urn:uuid:traversed</id>
    <link rel="http://opds-spec.org/acquisition" href="http://example.com/traversed.epub"/>
  </entry>
</feed>`)
		} else {
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Root Catalog</title>
  <entry>
    <title>Recent Section</title>
    <link rel="subsection" href="/recent" type="application/atom+xml;profile=opds-catalog;kind=acquisition"/>
  </entry>
</feed>`)
		}
	}))
	defer server.Close()

	adapter := &OPDSAdapter{
		catalogURL: server.URL,
		client:     &http.Client{},
	}

	books, err := adapter.FetchNewBooks(context.Background(), 0)
	if err != nil {
		t.Fatalf("FetchNewBooks failed: %v", err)
	}
	if len(books) != 1 {
		t.Errorf("Expected 1 book via traversal, got %d", len(books))
	}
	if len(books) > 0 && books[0].Title != "Traversed Book" {
		t.Errorf("Expected 'Traversed Book', got %s", books[0].Title)
	}
}

func TestOPDSAdapter_FetchNewBooks_InvalidXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// FP.Parse doesn't necessarily error on bad XML if it finds zero entries,
		// but since FetchNewBooks now returns nil err and continues on fetchPage failure,
		// we should verify it handles it gracefully.
		fmt.Fprint(w, `not xml`)
	}))
	defer server.Close()

	adapter := &OPDSAdapter{
		catalogURL: server.URL,
		client:     &http.Client{},
	}

	books, err := adapter.FetchNewBooks(context.Background(), 0)
	if err != nil {
		t.Fatalf("Expected nil error (graceful skip), got %v", err)
	}
	if len(books) != 0 {
		t.Errorf("Expected 0 books, got %d", len(books))
	}
}

func TestOPDSAdapter_DownloadBookContent_Errors(t *testing.T) {
	// 1. Not Found
	server404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server404.Close()

	adapter := &OPDSAdapter{
		client:  &http.Client{},
		maxSize: 100,
	}

	_, err := adapter.DownloadBookContent(context.Background(), models.BookMetadata{DownloadURL: server404.URL})
	if err == nil {
		t.Fatal("Expected error for 404")
	}

	// 2. Too large
	serverLarge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, 200))
	}))
	defer serverLarge.Close()

	rc, err := adapter.DownloadBookContent(context.Background(), models.BookMetadata{DownloadURL: serverLarge.URL})
	if err != nil {
		t.Fatalf("Expected no error on call, got %v", err)
	}
	defer rc.Close()

	_, err = io.ReadAll(rc)
	if err == nil {
		t.Fatal("Expected error during read for too large content")
	}
}

func TestOPDSAdapter_FetchNewBooks_NoURL(t *testing.T) {
	adapter := &OPDSAdapter{}
	_, err := adapter.FetchNewBooks(context.Background(), 0)
	if err == nil {
		t.Fatal("Expected error when URL is missing")
	}
}

func TestOPDSAdapter_Authentication(t *testing.T) {
	username := "user"
	password := "pass"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != username || p != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom"><entry><title>Auth Book</title><id>auth-1</id><link rel="http://opds-spec.org/acquisition" href="http://example.com/f.epub"/></entry></feed>`)
	}))
	defer server.Close()

	// 1. Success with correct credentials
	adapter := &OPDSAdapter{
		catalogURL: server.URL,
		username:   username,
		password:   password,
		client:     &http.Client{},
	}

	books, err := adapter.FetchNewBooks(context.Background(), 0)
	if err != nil {
		t.Fatalf("FetchNewBooks with auth failed: %v", err)
	}
	if len(books) != 1 {
		t.Errorf("Expected 1 book, got %d", len(books))
	}

	// 2. Failure with wrong credentials
	adapterWrong := &OPDSAdapter{
		catalogURL: server.URL,
		username:   "wrong",
		password:   "wrong",
		client:     &http.Client{},
	}
	books, err = adapterWrong.FetchNewBooks(context.Background(), 0)
	if err != nil {
		t.Fatalf("Expected nil error (graceful skip), got %v", err)
	}
	if len(books) != 0 {
		t.Errorf("Expected 0 books with wrong credentials, got %d", len(books))
	}
}
