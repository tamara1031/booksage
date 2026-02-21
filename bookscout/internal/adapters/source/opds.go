package source

import (
	"bookscout/internal/adapters/util"
	"bookscout/internal/core/domain/models"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/mmcdole/gofeed/atom"
)

type OPDSAdapter struct {
	catalogURL string
	username   string
	password   string
	client     *http.Client
	maxSize    int64
}

func NewOPDSAdapter(catalogURL, username, password string, maxSize int64, logLevel string) *OPDSAdapter {
	// Automated path generation
	if catalogURL != "" {
		if u, err := url.Parse(catalogURL); err == nil && u.Scheme != "" {
			if u.Path == "" || u.Path == "/" {
				u.Path = "/feed.xml"
				catalogURL = u.String()
			}
		}
	}

	return &OPDSAdapter{
		catalogURL: catalogURL,
		username:   username,
		password:   password,
		client: &http.Client{
			Transport: &util.LoggingTransport{LogLevel: logLevel},
			Timeout:   5 * time.Minute,
		},
		maxSize: maxSize,
	}
}

func (a *OPDSAdapter) FetchNewBooks(ctx context.Context, lastCheckTimestamp int64) ([]models.BookMetadata, error) {
	log.Printf("DEBUG OPDS: Fetching new books since %d (%s)", lastCheckTimestamp, time.Unix(lastCheckTimestamp, 0).Format(time.RFC3339))
	if a.catalogURL == "" {
		return nil, fmt.Errorf("OPDS URL is not configured")
	}

	var allBooks []models.BookMetadata
	visitedURLs := make(map[string]bool)
	queue := []struct {
		url   string
		depth int
	}{
		{a.catalogURL, 0},
	}

	const maxDepth = 3
	processedPages := 0
	const maxPages = 50 // Avoid infinite loops or memory exhaustion from massive catalogs

	for len(queue) > 0 && processedPages < maxPages {
		current := queue[0]
		queue = queue[1:]

		if visitedURLs[current.url] {
			continue
		}
		visitedURLs[current.url] = true
		processedPages++

		books, next, subsections, err := a.fetchPage(ctx, current.url, lastCheckTimestamp)
		if err != nil {
			log.Printf("DEBUG OPDS: Error fetching page %s: %v", current.url, err)
			continue
		}
		if len(books) > 0 {
			log.Printf("DEBUG OPDS: Found %d ingestible books on %s", len(books), current.url)
		}
		allBooks = append(allBooks, books...)

		// Pagination remains at the same depth
		if next != "" && !visitedURLs[next] {
			queue = append(queue, struct {
				url   string
				depth int
			}{next, current.depth})
		}

		// Traversal to subsections/sub-catalogs increments depth
		if current.depth < maxDepth {
			for _, sub := range subsections {
				if !visitedURLs[sub] {
					queue = append(queue, struct {
						url   string
						depth int
					}{sub, current.depth + 1})
				}
			}
		}
	}

	return allBooks, nil
}

const (
	relNext        = "next"
	relAcquisition = "http://opds-spec.org/acquisition"
	relOpenAccess  = "http://opds-spec.org/acquisition/open-access"
	relImage       = "http://opds-spec.org/image"
	relThumbnail   = "http://opds-spec.org/image/thumbnail"
	relSubsection  = "subsection"
	relCatalog     = "http://opds-spec.org/catalog"
)

func (a *OPDSAdapter) fetchPage(ctx context.Context, targetURL string, lastCheckTimestamp int64) ([]models.BookMetadata, string, []string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, "", nil, err
	}

	if a.username != "" {
		req.SetBasicAuth(a.username, a.password)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to fetch OPDS feed from %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", nil, fmt.Errorf("OPDS feed returned status: %d", resp.StatusCode)
	}

	fp := &atom.Parser{}
	feed, err := fp.Parse(resp.Body)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to parse OPDS feed as Atom: %w", err)
	}

	var books []models.BookMetadata
	var subsections []string
	baseURL, _ := url.Parse(targetURL)

	for _, entry := range feed.Entries {
		// Capture subsection/catalog links from entries
		for _, link := range entry.Links {
			if link.Rel == relSubsection || link.Rel == relCatalog {
				if ref, err := url.Parse(link.Href); err == nil {
					subsections = append(subsections, baseURL.ResolveReference(ref).String())
				}
			}
		}

		var entryTime time.Time
		if entry.UpdatedParsed != nil {
			entryTime = *entry.UpdatedParsed
		} else if entry.PublishedParsed != nil {
			entryTime = *entry.PublishedParsed
		}

		if !entryTime.IsZero() && entryTime.Unix() <= lastCheckTimestamp {
			log.Printf("DEBUG OPDS: Skipping book '%s' (updated: %s) - added before last check (%s)",
				entry.Title, entryTime.Format(time.RFC3339), time.Unix(lastCheckTimestamp, 0).Format(time.RFC3339))
			continue
		}

		book := models.BookMetadata{
			ID:          entry.ID,
			Title:       entry.Title,
			Author:      "Unknown",
			Description: entry.Summary,
			Source:      "opds",
			AddedAt:     entryTime,
		}

		if book.AddedAt.IsZero() {
			book.AddedAt = time.Now()
		}

		if len(entry.Authors) > 0 {
			book.Author = entry.Authors[0].Name
		}

		// Look for acquisition and image links
		var (
			bestAcquisition string
			thumbnail       string
		)

		for _, link := range entry.Links {
			// Find thumbnail
			if link.Rel == relThumbnail || (thumbnail == "" && link.Rel == relImage) {
				thumbnail = link.Href
			}

			// Prioritized acquisition finding
			if link.Rel == relOpenAccess {
				if bestAcquisition == "" || link.Type == "application/epub+zip" {
					bestAcquisition = link.Href
				}
			} else if link.Rel == relAcquisition {
				if bestAcquisition == "" || link.Type == "application/epub+zip" {
					bestAcquisition = link.Href
				}
			}
		}

		if bestAcquisition != "" {
			if ref, err := url.Parse(bestAcquisition); err == nil {
				book.DownloadURL = baseURL.ResolveReference(ref).String()
			}
			if thumbnail != "" {
				if ref, err := url.Parse(thumbnail); err == nil {
					book.ThumbnailURL = baseURL.ResolveReference(ref).String()
				}
			}
			books = append(books, book)
		}
	}

	// Capture subsection/catalog links from top-level feed links
	for _, link := range feed.Links {
		if link.Rel == relSubsection || link.Rel == relCatalog {
			if ref, err := url.Parse(link.Href); err == nil {
				subsections = append(subsections, baseURL.ResolveReference(ref).String())
			}
		}
	}

	// Find the next page link
	nextPageURL := ""
	for _, link := range feed.Links {
		if link.Rel == relNext {
			if ref, err := url.Parse(link.Href); err == nil {
				nextPageURL = baseURL.ResolveReference(ref).String()
			}
			break
		}
	}

	return books, nextPageURL, subsections, nil
}

func (a *OPDSAdapter) DownloadBookContent(ctx context.Context, book models.BookMetadata) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", book.DownloadURL, nil)
	if err != nil {
		return nil, err
	}

	if a.username != "" {
		req.SetBasicAuth(a.username, a.password)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download book from OPDS link: %d", resp.StatusCode)
	}

	limitReader := io.LimitReader(resp.Body, a.maxSize+1)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > a.maxSize {
		return nil, fmt.Errorf("book content exceeds maximum allowed size")
	}

	return data, nil
}
