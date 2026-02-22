# BookSage Development Guide

This guide provides instructions for setting up the local development environment for the BookSage Hybrid Architecture project. The project is split into a Go-based Orchestrator API and a Python-based ML Worker.

## Prerequisites
- Go 1.25+
- Python 3.12+
- [uv](https://docs.astral.sh/uv/) (Python package manager)
- Protobuf Compiler (`protoc`)
- Docker & Docker Compose
- Git

## 1. Initial Setup

The repository is structured as a monorepo. You need to set up both environments.

### Go API Orchestrator Setup
```bash
cd booksage/api
go mod download
go build ./cmd/booksage-api
```

### Python ML Worker Setup
We use `uv` for lightning-fast lockfile resolution and dependency management.
```bash
cd booksage/worker
uv sync --all-extras

# Optionally activate the virtual environment if you aren't using `uv run`
source .venv/bin/activate
```
**IDE Setup Note:** If you are using an IDE like VS Code, ensure you select `booksage/worker/.venv/bin/python` as your Python interpreter to enable correct module resolution and linting features.

## 2. Generating Protocol Buffers (gRPC)

When changes are made to `booksage/proto/booksage/v1/booksage.proto`, you must regenerate the stubs for both languages.

A centralized `Makefile` is provided in the root directory to handle this complex process.

```bash
# From the repository root
make proto-all
```

**Note:** Ensure you have the `protoc` compiler installed on your system.
For Go, you also need the gRPC plugins:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```
For Python, the required gRPC tools (`grpcio-tools`) are already included in the `uv sync` dependencies, so absolutely no further manual installation is needed.

## 3. Environment Variables

Copy the `.env.example` file to create your local `.env` file in the **repository root**:
```bash
cp .env.example .env
```
Ensure you add your actual API keys (e.g., `SAGE_GEMINI_API_KEY`) to the `.env` file. Both Go and Python services, as well as **Docker Compose**, will read from this file.

*Note: If you want to run the system entirely offline without Gemini, set `SAGE_USE_LOCAL_ONLY_LLM=true` in your `.env`.*

### Key Environment Variables

| Variable | Default | Description |
|---|---|---|
| `SAGE_WORKER_ADDR` | `worker:50051` | gRPC address of the Python ML Worker |
| `SAGE_GEMINI_API_KEY` | *(none)* | Google Gemini API key |
| `SAGE_OLLAMA_HOST` | `http://ollama:11434` | Ollama LLM host |
| `SAGE_OLLAMA_LLM_MODEL` | `llama3` | Local LLM for light tasks (intent/keywords) |
| `SAGE_OLLAMA_EMBED_MODEL` | `nomic-embed-text` | Dedicated model for high-quality embeddings |
| `SAGE_USE_LOCAL_ONLY_LLM` | `false` | Route all LLM tasks to local Ollama |
| `SAGE_QDRANT_HOST` | `qdrant` | Qdrant vector DB host |
| `SAGE_QDRANT_PORT` | `6334` | Qdrant gRPC port |
| `SAGE_NEO4J_URI` | `bolt://neo4j:7687` | Neo4j Bolt URI |

## 4. Infrastructure Setup (Docker Compose)

The full RAG system requires a Vector Database (Qdrant), a Graph Database (Neo4j), and a State Store (SQLite), alongside our microservices. 

```bash
# Start the entire infrastructure and build images
make up-build
```

To view the logs:
```bash
# Using Makefile targets
make logs

# Or using docker compose directly (it will automatically pick up .env from the root)
docker compose logs -f
```

## 5. Development Workflow

### Python Formatting & Linting (`ruff`)
We use `ruff` to ensure code quality in the `worker` directory.
## 5. Development Workflow

### Python Testing & Linting
```bash
cd booksage/worker
# Run small (unit/mock) tests
make test-worker-small

# Run medium (SUT) tests
make test-worker-medium
```

### Go Testing & Formatting
```bash
# For BookSage API
cd booksage/api
# Run small unit tests
make test-api-small

# Run medium integration tests
make test-api-medium
```

## 6. Architectural Overview

For details on the system's hybrid architecture and Separation of Concerns, please refer to [ARCHITECTURE.md](ARCHITECTURE.md).
