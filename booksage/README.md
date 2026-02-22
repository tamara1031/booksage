# BookSage Core Services

This directory contains the core services for the BookSage system: the **Go API Orchestrator** and the **Python ML Worker**. They communicate via gRPC defined in the `proto/` directory.

---

## 🐹 Go API Orchestrator (`api/`)

The high-performance gateway and orchestration engine for the BookSage RAG system. Built with Go for superior concurrency and low-latency I/O.

### Key Responsibilities

- **API Gateway**: Serving high-performance REST and SSE endpoints for real-time reasoning traces.
- **SOTA Agentic Loop (Dual-level Retrieval)**: Implements LightRAG-inspired keyword extraction — identifying **Low-level (Entities)** and **High-level (Themes)** in a single pass for parallel search.
- **Advanced Fusion (Skyline Ranker)**: Merges Vector and Graph search results using **Pareto-optimal** ranking (BookRAG) to eliminate noise and ensure structural relevance.
- **Local Intelligence (Ollama)**: Directly manages **Embedding Generation** and **Entity Extraction** via local Ollama APIs, offloading ML logic from the Python worker.
- **Reliable Ingestion Saga (SQLite)**: Orchestrates asynchronous pipelines using the **Saga Pattern** with an internal **SQLite** store for hash-based deduplication and state persistence.
- **Resilience**: Integrated Circuit Breakers, graceful shutdown, and Kubernetes-native health/readiness probes.
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

- **Structured ETL (Docling)**: High-precision layout analysis of PDFs and EPUBs, extracting tables, headings, and logical hierarchies.
- **Intelligent Chunking**: Layout-aware chunking that preserves context and maps structural metadata for Go-side **RAPTOR** summarization.
- **Hybrid Tensor Generation**: Offloads GPU-intensive tensor tasks (e.g., **ColBERTv2**) when required, keeping the core orchestrator lightweight.
- **gRPC Server**: High-throughput streaming backend for document ingestion.

### Setup & Development

- **Prerequisites**: Python 3.12+ and `uv` package manager.
- **Installation**: Run `uv sync` from the `worker/` directory.
- **Testing**: Use `make test-worker` from the monorepo root.

### Architecture Notes

- **gRPC Client Streaming**: To avoid memory spikes from receiving massive PDF binaries over gRPC all at once, the Go Orchestrator streams the file in chunks over a persistent gRPC connection.
- **Async I/O & CPU Offloading**: Built on the asynchronous **`grpc.aio`** API and `asyncio` for high-throughput I/O handling. To ensure the gRPC server remains responsive, all heavy CPU-bound operations (like Docling ETL) are strictly offloaded via `asyncio.get_running_loop().run_in_executor` to a `ProcessPoolExecutor`.
- **⚠️ GPU Execution Warning**: For GPU-bound operations (like PyTorch Embedding), do **not** use the default `ProcessPoolExecutor`. Spawning new processes can corrupt the CUDA context and cause the application to freeze. GPU tasks must be managed safely using a `ThreadPoolExecutor` (which releases the GIL for native extensions) or carefully isolated process pools.
- **Dependency Injection (DI)**: The gRPC Servicer receives its dependencies (Parsers and Executors) via its constructor for clean separation and testability.
- **Incremental Graph Update**: Leveraging LightRAG principles, the system performs **Union-based incremental updates** to Neo4j, avoiding global re-computation and linking entities to trees via **GT-Links**.
