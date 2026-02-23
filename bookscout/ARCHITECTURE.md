# BookScout Architecture

BookScout is a lightweight, stateful collection worker designed to pull documents from remote sources (like OPDS feeds) and push them to the BookSage ingestion API.

## 1. System Overview

### Responsibilities
- **Extract (Fetch):** Periodically polls remote sources for new content.
- **Filter (State):** Uses a local SQLite state store to track processed items and prevent duplicates.
- **Load (Push):** Downloads content, calculates SHA-256 hash, and uploads it to the BookSage API (`/api/v1/ingest`).
- **Sync (Status):** Polls BookSage API for the status of asynchronous ingestion tasks.

### Tech Stack
- **Language:** Go 1.25.7
- **Deployment:** Docker / Kubernetes CronJob
- **State:** SQLite 3 (managed by Bun ORM)
- **Config:** Viper (Environment variables & Defaults)
- **CLI:** Cobra

### Boundary
BookScout operates as an external client to BookSage. It does not share databases or internal logic. Communication is strictly via the public REST API.

```mermaid
graph LR
    subgraph External["Remote Sources"]
        OPDS[OPDS Feed]
    end

    subgraph BookScout["BookScout Worker"]
        Worker[Worker Service]
        State[(SQLite Store)]
    end

    subgraph BookSage["BookSage System"]
        API[Ingest API]
    end

    OPDS -->|Pull XML| Worker
    Worker <-->|Read/Write Watermark & Status| State
    Worker -->|Push Multipart + SHA256| API
    Worker <--|Poll Status by Hash| API
```

## 2. Architecture Layers (DDD)

BookScout follows a layered architecture to maintain clear boundaries and testability.

### 2.1 Domain Layer (`internal/scout/domain`)
The heart of the application, containing business logic and standard interfaces. It has zero external dependencies.
- **`models.go`**: Core entities like `TrackedDocument` and value objects like `Book`.
- **`interfaces.go`**: Defines the "contracts" for external communication (`Scraper`, `Ingestor`, `StateRepository`).

### 2.2 Application Layer (`internal/scout/app`)
Coordinates the flow of data between the domain and infrastructure.
- **`worker.go`**: The primary orchestrator (`ScoutWorker`).
- **`sync.go`**: Handles the Status Sync use case.
- **`batch.go`**: Handles the Scrape & Ingest use case.

### 2.3 Infrastructure Layer (`internal/scout/infra`)
Concrete implementations of domain interfaces.
- **`sqlite/`**: Persistent storage using Bun ORM.
- **`booksage/`**: Adapter for the BookSage REST API.
- **`opds/`**: Implementation of the OPDS catalog scraper.

## 3. Class Design

```mermaid
classDiagram
    direction TB

    class ScoutWorker {
        +Run(ctx Context) error
    }

    class Scraper {
        <<interface>>
        +Scrape(ctx, since) []Book
        +DownloadContent(ctx, book) ReadCloser
    }

    class Ingestor {
        <<interface>>
        +Ingest(ctx, book, content) (string hash, error)
        +GetStatusByHash(ctx, hash) (status, error)
    }

    class StateRepository {
        <<interface>>
        +GetWatermark() int64
        +IsProcessed(id) bool
        +UpdateWatermark(timestamp) error
        +GetProcessingDocuments() []TrackedDoc
        +UpdateStatusByHash(hash, status, err) error
        +RecordIngestion(id, hash) error
    }

    class OPDSScraper {
        -client HttpClient
        +Scrape()
        +DownloadContent()
    }

    class APIIngester {
        -client HttpClient
        +Ingest()
        +GetStatusByHash()
    }

    class SQLiteRepository {
        -db *bun.DB
        +RecordIngestion()
        +GetProcessingDocuments()
    }

    ScoutWorker --> Scraper
    ScoutWorker --> Ingestor
    ScoutWorker --> StateRepository

    OPDSScraper ..|> Scraper
    APIIngester ..|> Ingestor
    SQLiteRepository ..|> StateRepository
```

## 3. Ingest Sequence

The ingestion process is split into two phases to handle asynchronous API processing reliably.

### Phase 1: Status Sync
```mermaid
sequenceDiagram
    participant W as ScoutWorker
    participant S as StateRepository
    participant B as BookSage API

    W->>S: GetProcessingDocuments()
    S-->>W: [HashA, HashB, ...]
    loop For Each Hash
        W->>B: GetStatusByHash(Hash)
        B-->>W: status: "completed"
        W->>S: UpdateStatusByHash(Hash, COMPLETED)
    end
```

### Phase 2: Scrape & Ingest
```mermaid
sequenceDiagram
    participant W as ScoutWorker
    participant S as StateRepository
    participant O as OPDS Source
    participant B as BookSage API

    W->>S: GetWatermark()
    S-->>W: lastTimestamp

    W->>O: Scrape(since=lastTimestamp)
    O-->>W: [Book1, Book2, ...]

    loop For Each Book (Concurrent)
        W->>S: IsProcessed(BookID)?
        alt New Book
            S-->>W: false
            W->>O: DownloadContent(Book)
            O-->>W: Stream<Content>
            W->>B: Ingest(Metadata + Content)
            Note right of W: Calculates SHA-256 locally
            B-->>W: 202 Accepted
            W->>S: RecordIngestion(BookID, Hash, PROCESSING)
        end
    end

    W->>S: UpdateWatermark(newMaxTimestamp)
```
