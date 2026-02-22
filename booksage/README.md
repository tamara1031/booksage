# BookSage Core Services

This directory contains the core services for the BookSage system: the **Go API Orchestrator** and the **Python ML Worker**. They communicate via gRPC defined in the `proto/` directory.

---

## 🐹 Go API Orchestrator (`api/`)

The high-performance gateway and orchestration engine for the BookSage RAG system. Built with Go for superior concurrency and low-latency I/O.

### Key Responsibilities

- **API Gateway**: Provides the primary interface (REST/gRPC) for user interactions.
- **Agentic Loop (CoR & Self-RAG)**: Orchestrates the reasoning chain, decomposing queries and critiquing results.
- **Fusion Retrieval**: Executes parallel queries across Neo4j (Graph) and Qdrant (Vector) using Go's `goroutines` and `errgroup`.
- **LLM Router**: Intelligently dispatches tasks between local models (Ollama) and Cloud APIs (Gemini 1.5 Pro).
- **gRPC Client**: Manages communication with the Python ML Worker via **gRPC Client Streaming**.

### Setup & Development

- **Prerequisites**: Go 1.25+
- **Installation**: Run `go mod download` from the `api/` directory.
- **Testing**: Use `make test-api` from the monorepo root.

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
