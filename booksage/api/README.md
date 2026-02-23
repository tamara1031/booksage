# BookSage Orchestrator (Go API)

The high-performance gateway and orchestration engine for the BookSage RAG system.

## Configuration

The API is configured via environment variables (see the root [README.md](../README.md) for the full list):

- `SAGE_WORKER_ADDR`: ML Worker gRPC address.
- `SAGE_USE_LOCAL_ONLY_LLM`: Set to `true` to use only local models (Ollama).
- `SAGE_GEMINI_API_KEY`: Required if using Google Gemini.

## Development

### Prerequisites
- Go 1.25+

### Commands
- `make test-api-small`: Run unit tests.
- `make test-api-medium`: Run SUT (System Under Test) integration tests.
- `go mod tidy`: Update dependencies.

## Architecture
See the [ARCHITECTURE.md](../ARCHITECTURE.md) for a deep dive into the Agentic RAG and Saga implementation.
