package opds_test

import (
	"bookscout/internal/scout/infra/opds"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOPDSScraper_Scrape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom"><entry><title>Test</title><id>1</id><link rel="http://opds-spec.org/acquisition" href="http://e.com/1.epub"/></entry></feed>`)
	}))
	defer server.Close()

	scraper := opds.NewOPDSScraper(server.URL, "", "", 100, "info")
	books, err := scraper.Scrape(context.Background(), time.Time{})
	assert.NoError(t, err)
	assert.Len(t, books, 1)
}
