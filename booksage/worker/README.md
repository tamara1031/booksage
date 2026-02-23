# BookSage Python Worker

Dedicated ML/ETL backend for the BookSage system.

## Configuration

The worker is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `SAGE_WORKER_LISTEN_ADDR` | gRPC server listen address | `[::]:50051` |
| `SAGE_WORKER_MAX_WORKERS` | Max CPU processes for layout parsing | CPU count |

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
- **Layout Parsing**: Uses [Docling](https://github.com/DS4SD/docling) for PDF/EPUB structure extraction.
- **Concurrency**: Combines `asyncio` for gRPC/IO and `ProcessPoolExecutor` for CPU-heavy tasks.
