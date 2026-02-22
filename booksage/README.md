# BookSage Core Services

This directory contains the core services for the BookSage system: the **Go API Orchestrator** and the **Python ML Worker**. They communicate via gRPC defined in the `proto/` directory.

---

## 🐹 Go API Orchestrator (`api/`)

The high-performance gateway and orchestration engine for the BookSage RAG system. Built with Go for superior concurrency and low-latency I/O.

### Key Responsibilities

- **API Gateway**: Provides the primary interface (REST/SSE) for user interactions, with middleware stack (Request ID, Structured Logging, Panic Recovery).
- **SOTA Agentic Loop (CoR & Self-RAG)**: Orchestrates reasoning via Chain-of-Retrieval (CoR), critiques retrieval relevance, and validates factual grounding via Support Level evaluation.
- **Advanced Fusion Retrieval**: Executes parallel queries across Neo4j (Graph) and Qdrant (Vector) with **Skyline Ranker (Pareto-optimal)** and weighted RRF.
- **Dual-Model LLM Router**: Intelligently dispatches tasks between local models (separate LLM and Embedding clients) and Gemini.
- **Reliable Ingestion Saga**: Implements a SagaOrchestrator for idempotent, hash-based document processing across multiple databases.
- **Resilience**: Circuit Breaker pattern, graceful shutdown, and health/readiness probes.
- **gRPC Client**: Manages communication with the Python ML Worker via **gRPC Client Streaming**.

### Configuration

The Orchestrator is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `SAGE_GEMINI_API_KEY` | Google Gemini API Key | Required (if not local-only) |
| `SAGE_OLLAMA_HOST` | Ollama Server URL | `http://localhost:11434` |
| `SAGE_OLLAMA_LLM_MODEL` | Local LLM for light tasks (intent/keywords) | `llama3` |
| `SAGE_OLLAMA_EMBED_MODEL` | Dedicated model for high-quality embeddings | `nomic-embed-text` |
| `SAGE_USE_LOCAL_ONLY_LLM` | Force local-only execution | `false` |
| `SAGE_WORKER_ADDR` | ML Worker gRPC address | `localhost:50051` |

### Setup & Development

- **Prerequisites**: Go 1.25+
- **Installation**: Run `go mod download` from the `api/` directory.
- **Testing**: Use `make test-api-small` (unit) or `make test-api-medium` (SUT) from the root.

---

## 🐍 Python ML Worker (`worker/`)

The dedicated ML/ETL backend for the BookSage system. This worker is responsible for heavy CPU/GPU-bound tasks and serves the Go Orchestrator via gRPC.

### Key Responsibilities

- **ETL (Document Parsing)**: High-speed PDF/EPUB parsing using `Docling` and `PyMuPDF`.
- **Intelligent Chunking**: Implements layout-aware chunking and Two-Level Indexing (Document & Chunk layers).
- **Embedding Generation**: Processes dense and sparse (ColBERT) vector generations.
- **gRPC Server**: A robust server that accepts parsing and embedding requests from the Go gateway.

### Setup & Development

- **Prerequisites**: Python 3.12+ and `uv` package manager.
- **Installation**: Run `uv sync` from the `worker/` directory.
- **Testing**: Use `make test-worker` from the monorepo root.

### Architecture Notes

- **gRPC Client Streaming**: To avoid memory spikes from receiving massive PDF binaries over gRPC all at once, the Go Orchestrator streams the file in chunks over a persistent gRPC connection.
- **Async I/O & CPU Offloading**: Built on the asynchronous **`grpc.aio`** API and `asyncio` for high-throughput I/O handling. To ensure the gRPC server remains responsive, all heavy CPU-bound operations (like Docling ETL) are strictly offloaded via `asyncio.get_running_loop().run_in_executor` to a `ProcessPoolExecutor`.
- **⚠️ GPU Execution Warning**: For GPU-bound operations (like PyTorch Embedding), do **not** use the default `ProcessPoolExecutor`. Spawning new processes can corrupt the CUDA context and cause the application to freeze. GPU tasks must be managed safely using a `ThreadPoolExecutor` (which releases the GIL for native extensions) or carefully isolated process pools.
- **Dependency Injection (DI)**: The gRPC Servicer (`BookSageWorker`) receives its dependencies (Parsers, Embedders, and Executors) via its constructor. This allows for clean separation of concerns and easy mocking during unit testing.
