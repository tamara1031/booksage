package api

import (
	"net/http"

	"github.com/booksage/booksage-api/internal/ingest"
	"github.com/booksage/booksage-api/internal/query"
)

// RegisterRoutes registers all API endpoints with a new ServeMux
func RegisterRoutes(ingestH *ingest.Handler, queryH *query.Handler) http.Handler {
	mux := http.NewServeMux()

	// REST API Endpoints
	mux.HandleFunc("POST /api/v1/query", queryH.HandleQuery)
	mux.HandleFunc("POST /api/v1/ingest", ingestH.HandleIngest)
	mux.HandleFunc("GET /api/v1/ingest/status", ingestH.HandleIngestStatusByHash)
	mux.HandleFunc("GET /api/v1/documents/{document_id}/status", ingestH.HandleDocumentStatus)
	mux.HandleFunc("HEAD /api/v1/documents/{document_id}", ingestH.HandleDocumentExist)

	// Health / Readiness probes
	mux.HandleFunc("GET /healthz", HandleHealthz)
	mux.HandleFunc("GET /readyz", HandleReadyz)

	// Apply middleware stack
	return Chain(mux,
		RecoveryMiddleware,
		LoggingMiddleware,
		RequestIDMiddleware,
	)
}

func HandleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func HandleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}
