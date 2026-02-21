# BookSage Development Guide

This guide provides instructions for setting up the local development environment for the BookSage Hybrid Architecture project. The project is split into a Go-based Orchestrator API and a Python-based ML Worker.

## Prerequisites
- Go 1.24+
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
Ensure you add your actual API keys (e.g., `BS_GEMINI_API_KEY`) to the `.env` file. Both Go and Python services, as well as **Docker Compose**, will read from this file.

*Note: If you want to run the system entirely offline without Gemini, set `BS_USE_LOCAL_ONLY_LLM=true` in your `.env`.*

## 4. Infrastructure Setup (Docker Compose)

The full RAG system requires a Vector Database (Qdrant) and a Graph Database (Neo4j), alongside our microservices. We provide a consolidated Docker Compose configuration in the **repository root**.

```bash
# Start the entire infrastructure from the root directory
make up
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
```bash
cd booksage/worker
uv run ruff check src/ tests/ --fix
uv run ruff format src/ tests/
```

### Go Formatting (`go fmt`)
Use standard Go tools in the `api` and `bookscout` directories.
```bash
# For BookSage API
cd booksage/api
go fmt ./...
go test ./...

# For BookScout
cd ../../bookscout
go fmt ./...
go test ./...
```

## 6. Architectural Overview

For details on the system's hybrid architecture and Separation of Concerns, please refer to [ARCHITECTURE.md](ARCHITECTURE.md).
