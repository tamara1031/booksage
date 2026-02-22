# BookScout Architecture

BookScout is a lightweight, stateful collection worker designed to pull documents from remote sources (like OPDS feeds) and push them to the BookSage ingestion API.

## 1. System Overview (全体設計)

### Responsibilities
- **Extract (Fetch):** Periodically polls remote sources for new content.
- **Filter (State):** Uses a local state store to track processed items and prevent duplicates.
- **Load (Push):** Downloads content and uploads it to the BookSage API (`/api/v1/ingest`).

### Tech Stack
- **Language:** Go 1.25.7
- **Deployment:** Docker / Kubernetes CronJob
- **State:** Local JSON file (persistent volume)

### Boundary
BookScout operates as an external client to BookSage. It does not share databases or internal logic. Communication is strictly via the public REST API.

```mermaid
graph LR
    subgraph External["Remote Sources"]
        OPDS[OPDS Feed]
    end

    subgraph BookScout["BookScout Worker"]
        Worker[Worker Service]
        State[(State Store)]
    end

    subgraph BookSage["BookSage System"]
        API[Ingest API]
    end

    OPDS -->|Pull XML| Worker
    Worker <-->|Read/Write Watermark| State
    Worker -->|Push Multipart/Form| API
```

## 2. Class Design (構造の可視化)

The architecture follows the Hexagonal Architecture (Ports & Adapters) pattern.

```mermaid
classDiagram
    direction TB

    class WorkerService {
        +Run(ctx Context) error
    }

    class BookSource {
        <<interface>>
        +FetchNewBooks(ctx, timestamp) []BookMetadata
        +DownloadBookContent(ctx, book) ReadCloser
    }

    class BookDestination {
        <<interface>>
        +Send(ctx, book, content) error
    }

    class StateStore {
        <<interface>>
        +GetWatermark() int64
        +IsProcessed(id) bool
        +MarkProcessed(id) error
        +Save() error
    }

    class OPDSAdapter {
        -client HttpClient
        +FetchNewBooks()
        +DownloadBookContent()
    }

    class BookSageAPIAdapter {
        -client HttpClient
        -baseURL string
        +Send()
    }

    class FileStateStore {
        -filepath string
        -state StateData
        +Save()
    }

    WorkerService --> BookSource
    WorkerService --> BookDestination
    WorkerService --> StateStore

    OPDSAdapter ..|> BookSource
    BookSageAPIAdapter ..|> BookDestination
    FileStateStore ..|> StateStore
```

## 3. Ingest Sequence (ふるまいの可視化)

The ingestion process is designed to be idempotent and robust against partial failures.

```mermaid
sequenceDiagram
    participant C as CronJob/Main
    participant W as WorkerService
    participant S as StateStore
    participant O as OPDS Source
    participant B as BookSage API

    C->>W: Run(ctx)
    activate W

    W->>S: GetWatermark()
    S-->>W: lastTimestamp

    W->>O: FetchNewBooks(since=lastTimestamp)
    O-->>W: [Book1, Book2, ...]

    loop For Each Book (Concurrent)
        W->>S: IsProcessed(BookID)?
        alt Already Processed
            S-->>W: true (Skip)
        else New Book
            S-->>W: false

            W->>O: DownloadBookContent(Book)
            O-->>W: Stream<Content>

            W->>B: Send(BookMetadata + Content)
            B-->>W: 200 OK

            W->>S: MarkProcessed(BookID)
        end
    end

    W->>S: UpdateWatermark(newMaxTimestamp)
    W->>S: Save()

    deactivate W
```
