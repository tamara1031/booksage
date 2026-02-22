# BookSage Sub-project Architecture

BookSage is the core engine of the system, split into a Go API Orchestrator and a Python ML Worker.

---

## 1. Components

### Go API Orchestrator (`api/`)
**Role:** The high-performance gateway and cognitive conductor.
- **REST API Server**: Serves public HTTP endpoints (e.g., `/api/v1/query`, `/api/v1/ingest`). Includes middleware stack (Request ID, Logging, Recovery).
- **Agentic Loop (CoR & Self-RAG)**: Orchestrates reasoning. It decomposes user queries into sub-queries and critiques retrieved data for hallucinations.
- **Fusion Retrieval Orchestrator**: Uses lightweight `goroutines` to query multiple databases (Neo4j, Qdrant) concurrently. Intent-driven dynamic fusion with weighted RRF.
- **LLM Router**: Intelligently dispatches tasks. Heavy reasoning goes to Cloud LLMs (Gemini), while lightweight tasks are sent to the local Ollama model.
- **Production Middleware**: Request ID propagation, structured access logging, panic recovery, and Circuit Breaker (Closed/Open/HalfOpen) for external service calls.
- **Health Probes**: `/healthz` (liveness) and `/readyz` (readiness) endpoints for Kubernetes.
- **Graceful Shutdown**: SIGTERM/SIGINT signal handling with connection draining.

### Python ML Worker (`worker/`)
**Role:** The heavy-lifting Machine Learning and ETL engine.
- **ETL (Document Parsing)**: High-speed parsing of PDFs and EPUBs (using Docling/PyMuPDF). It analyzes layouts, extracts tables, and forms the "Two-Level Indexing" structure.
- **Tensor/Embedding Calculations**: Generates dense and sparse (ColBERTv2) vector embeddings for the chunks parsed during ETL.
- **Async & Multi-Processing**: Built on `asyncio` to handle high-throughput I/O. GPU-bound tasks are safely managed to prevent CUDA context corruption.
- **gRPC Client Streaming**: Receives file chunks from the Go API to handle large documents efficiently.

---

## 2. Core Mechanisms

### A. Two-Level Indexing (ETL Phase)
Context is preserved by extracting global Document Metadata and attaching it to every finely divided `Chunk`. This enables strict pre-filtering during retrieval.

### B. Multi-Engine Fusion Retrieval (Retrieval Phase)
The Go Orchestrator executes three engines asynchronously in parallel:
1. **LightRAG (Neo4j Graph)**: Excels at multi-hop reasoning and entity relationships.
2. **RAPTOR (Qdrant Tree)**: Excels at macro summarization across chapters.
3. **ColBERTv2 (Qdrant Tensor)**: Excels at exact, microscopic token matching.

The results are dynamically prioritized using **intent-driven dynamic fusion (Operator pattern)**.

### C. Agentic Generation (Generation Phase)
Wraps generation in an autonomous evaluation loop (Self-RAG):
1. **Chain-of-Retrieval (CoR)**: Decomposes complex questions.
2. **Retrieval Critique**: Validates context relevance.
3. **Generation Critique**: Verifies factual support in the text.

---

## 3. Technology Stack

- **Languages:** Go 1.25+ (API), Python 3.12+ (Worker)
- **Communication:** gRPC / Protocol Buffers (`proto/booksage/v1/`)
- **Databases:** Qdrant (Vector/Tensor), Neo4j (Graph)
- **Deployment:** Docker Compose, MicroK8s
