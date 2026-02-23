# BookSage Orchestrator (Go API)

The high-performance gateway and orchestration engine for the BookSage RAG system.

### Configuration

The API is configured via environment variables, categorized by domain (see the root [README.md](../README.md) for the full list):

- `SAGE_CLIENT_WORKER_ADDR`: ML Worker gRPC address.
- `SAGE_MODEL_LOCAL_ONLY`: Use only local models (Ollama).
- `SAGE_MODEL_GEMINI_KEY`: API key for Google Gemini.
- `SAGE_DB_QDRANT_HOST`: Qdrant Vector DB host.
- `SAGE_TIMEOUT_DEFAULT`: Default request timeout.

## Development

### Prerequisites
- Go 1.25+

### Commands
- `make test-api-small`: Run unit tests.
- `make test-api-medium`: Run SUT (System Under Test) integration tests.
- `go mod tidy`: Update dependencies.

## Architecture
See the [ARCHITECTURE.md](../ARCHITECTURE.md) for a deep dive into the Agentic RAG and Saga implementation.
