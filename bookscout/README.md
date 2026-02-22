# BookScout

BookScout is a dedicated collection worker (intended for Kubernetes CronJobs) that pulls from remote OPDS feeds and pushes documents into the main BookSage API.

## 🚀 Role
- **Data Acquisition**: Periodically scans OPDS catalogs for new books.
- **Persistent Watermarks**: Features state-aware tracking using a local state store to ensure idempotent and reliable scraping across restarts.
- **Integrated Ingestion**: Downloads book binaries and metadata, then forwards them to the BookSage REST API (`/api/v1/ingest`), triggering the **Saga-based** SOTA RAG pipeline.

## 🛠️ Setup & Configuration

BookScout is managed as part of the BookVerse monorepo. 

### Environment Variables
- `BS_API_BASE_URL`: The endpoint of the BookSage API (e.g., `http://api:8080/api/v1`).
- `BS_OPDS_BASE_URL`: The URL of the target OPDS feed.
- (See the root `.env.example` for a full list of configuration options.)

### Local Development
From the root of the repository:
```bash
# Run unit tests
make test-bookscout

# Format code
make fmt-bookscout
```

### Docker
BookScout can be built using its own Dockerfile:
```bash
docker build -f bookscout/Dockerfile -t bookscout .
```
It is also integrated into the root `docker-compose.yml`.
