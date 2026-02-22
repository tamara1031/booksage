# BookSage API Reference

This document outlines the API definitions for the BookSage system. BookSage utilizes a hybrid approach: a public-facing REST API served by the Go Orchestrator, and an internal gRPC API for high-performance communication with the Python ML Worker.

---

### 1. Public REST API (Go Orchestrator)

The Go Orchestrator (`api/`) serves as the primary entry point for user interactions. It exposes a RESTful HTTP API.

*(Note: The REST API is currently in the design phase and represents the planned endpoints for the frontend application. The complete specification is also provided as an OpenAPI 3.0 schema in the repository.)*

**Standardized Error Responses:**
For all REST API endpoints, 4xx and 5xx errors adopt the standardized RFC 7807 Problem Details format. A typical error response includes a `correlation_id` for backend tracing:
```json
{
  "type": "https://booksage.example.com/errors/invalid-request",
  "title": "Invalid Request Parameters",
  "status": 400,
  "detail": "The 'query' field cannot be empty.",
  "correlation_id": "req-uuid-string"
}
```

### 1.1 `POST /api/v1/query`
**Description:** Submits a query to the BookSage Agentic Generation Engine for an answer based on ingested books. The system uses **Dual-level Retrieval** (Entities + Themes) and **Skyline Ranker** (Pareto-optimal fusion) to gather high-precision context, encapsulated in a **Self-RAG** evaluation loop.

**Request Body:**
```json
{
  "query": "Explain the history of quantum mechanics based on the physics textbook.",
  "session_id": "optional-uuid-for-chat-history",
  "filters": {
    "book_ids": ["book-12345"]
  }
}
```

**Response (200 OK - `text/event-stream`):**
Streams Server-Sent Events (SSE) detailing the reasoning trace, sources, and final answer.

**Event Format (JSON payloads per event):**
```json
{
  "type": "reasoning", // "Analyzing complexity", "Extracting Keywords", "Evaluating Relevance", "Critiquing Support"
  "content": "Analyzing query complexity..."
}
{
  "type": "source",    // Retrieval source metadata (Graph, Vector, or RAPTOR)
  "content": "[RAPTOR] (score: 0.95) Chapter 1 Summary..."
}
{
  "type": "answer",    // Final generated content
  "content": "The history of quantum..."
}
```

### 1.2 `POST /api/v1/ingest`
**Description:** Uploads a document (PDF, EPUB) for ETL and embedding. 

**Request (Multipart/Form-Data):**
- `file`: The binary file to upload.
- `metadata`: JSON string containing title, author, etc.

**Response (202 Accepted):**
```json
{
  "saga_id": 42,
  "status": "pending",
  "hash": "sha256-hex-string"
}
```

### 1.3 `GET /api/v1/ingest/status?hash={sha256}`
**Description:** Checks the ingestion status of a previously uploaded document using its SHA-256 hash. This endpoint queries the **SQLite Saga Store** for idempotency and step-by-step progress.

**Response (200 OK):**
```json
{
  "document_id": "doc-uuid",
  "status": "completed", // pending, processing, completed, failed
  "current_step": "persistence", // parsing, embedding, persistence
  "updated_at": 1708600000
}
```

### 1.3 `GET /api/v1/documents/{document_id}/status`
**Description:** Checks the ingestion status of a previously uploaded document.

**Response (200 OK):**
```json
{
  "document_id": "doc-uuid-string",
  "status": "completed", 
  "extracted_metadata": {
    "title": "Quantum Physics Book",
    "pages": 312
  }
}
```

### 1.4 `GET /api/v1/documents`
**Description:** Retrieves a paginated list of uploaded documents.

**Query Parameters:**
- `cursor` (optional): The string cursor for the next page of results.
- `limit` (optional): Max items per page (default: 20).

**Response (200 OK):**
```json
{
  "documents": [
    {
      "document_id": "doc-1",
      "status": "completed",
      "title": "Quantum Physics Book"
    }
  ],
  "next_cursor": "encoded-cursor-string-for-next-page",
  "has_more": true
}
```

---

## 2. Internal gRPC API (Worker <-> API)

The internal gRPC API defines the strict boundaries between the Go Orchestrator and the Python ML Worker. It operates over Protocol Buffers (`booksage/proto/booksage/v1/booksage.proto`).

**Observability & Tracing requirement:**
All gRPC requests MUST propagate a `correlation_id` via gRPC metadata to ensure end-to-end distributed tracing across the Go-Python boundary.

### 2.1 `DocumentParserService`
Handles the Heavy ETL workloads, extracting markdown texts and metadata from complex binaries.

**RPC:** `Parse(stream ParseRequest) returns (ParseResponse)`
- **Client Streaming:** The Go API breaks raw documents into chunks and streams them to the Worker to circumvent gRPC's 4MB limit constraint.
- **ParseRequest Payload:**
  - `metadata`: Sent initially.
  - `chunk_data`: Sequential file bytes.
- **ParseResponse:** Returns unstructured metadata and a list of `RawDocument` messages (chunked structural elements translated to standard Markdown, optionally with page numbers).

### 2.2 `EmbeddingService`
Handles GPU/CPU heavy tensor calculations for embeddings.

**RPC:** `GenerateEmbeddings(EmbeddingRequest) returns (EmbeddingResponse)`
- **EmbeddingRequest:** Accepts an array of string `texts` to optimize for GPU batching. Specifies `embedding_type` (e.g. `dense`, `qdrant`) and `task_type`.
- **EmbeddingResponse:** Returns the `total_tokens` processed and a list of `EmbeddingResult` messages.
- **Vector Representations (OneOf):**
  - `DenseVector`: Float array (e.g. 768 or 1536 dim).
  - `SparseVector`: Index + Value array (e.g. SPLADE).
  - `TensorVector`: Flattened multi-dimensional array with shapes (e.g. ColBERT token embeddings).
