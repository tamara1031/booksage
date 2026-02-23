# BookSage Python Worker

Dedicated ML/ETL backend for the BookSage system.

### Configuration

The worker is configured via environment variables (prefixed with `SAGE_WORKER_`):

| Variable | Description | Default |
|----------|-------------|---------|
| `SAGE_WORKER_PORT` | gRPC listen address | `[::]:50051` |
| `SAGE_WORKER_MAX_CONCURRENCY` | Max worker processes | CPU count |

## Development

### Prerequisites
- Python 3.12+
- `uv` package manager

### Setup
```bash
uv sync
```

### Testing
```bash
# From the worker/ directory
uv run pytest tests/unit/        # Small (Unit)
uv run pytest tests/integration/ # Large (Integration)

# From the root directory
make test-worker-small
make test-worker-large
```

## Architecture

The worker implements a RAG pipeline across several specialized modules:

- **gRPC Service**: Entry point for the BookSage API. It coordinates the ingestion saga.
- **ETL Component**:
  - **Docling Integration**: Advanced PDF/EPUB structure extraction.
  - **Markdown Parsing**: Standardized intermediate format for all documents.
- **Embedding & Search**:
  - **Dense Embeddings**: High-dimensional vector representations.
  - **Knowledge Graph**: (Optional/Experimental) Graph-based relationship extraction.
- **Concurrency**: Combines `asyncio` for gRPC/IO and `ProcessPoolExecutor` for CPU-heavy mining/embedding tasks to ensure high throughput without blocking.

## Deployment

The worker is designed to run as a horizontal-scaled service in a Kubernetes cluster or via Docker Compose.

```bash
# Build Docker image
docker build -t booksage-worker -f booksage/worker/Dockerfile .
```
