# Sequence Diagram: Document Ingestion Flow

This document visualizes the end-to-end flow of document ingestion in BookSage. It details the interaction between the user, the Go API Orchestrator, the Python Parser Worker, and the databases (Qdrant & Neo4j).

## Flow Overview

1.  **Upload & Validation**: The user uploads a document. The server calculates a hash and initializes a Saga record.
2.  **Streaming Parse**: The file is streamed to the Python Worker via gRPC for high-performance parsing (e.g., using Docling).
3.  **Async Processing**: The user receives an immediate `202 Accepted` response while the server processes the document in the background.
4.  **Saga Execution**: The `SagaOrchestrator` manages a multi-step transaction:
    -   **Vector Insertion**: Chunks are embedded and stored in Qdrant.
    -   **Graph Construction**: A RAPTOR tree is built, and entities are extracted for GraphRAG. These are stored in Neo4j.
    -   **Compensation**: If the Graph step fails, the Vector step is rolled back to ensure consistency.

## Sequence Diagram

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant Server as API Server (Go)
    participant Saga as Saga Orchestrator (Go)
    participant Worker as Parser Worker (Python gRPC)
    participant VectorDB as Qdrant (Vector DB)
    participant GraphDB as Neo4j (Graph DB)

    User->>Server: POST /api/v1/ingest (File)
    activate Server
    Server->>Server: Calculate SHA-256 Hash
    Server->>Saga: StartOrResumeIngestion(doc)
    activate Saga
    Saga->>Saga: Create/Check Saga Record (Pending)
    Saga-->>Server: Saga ID
    deactivate Saga

    Server->>Worker: Stream ParseRequest (Metadata)
    activate Worker
    Server->>Worker: Stream ParseRequest (Chunks)
    Worker->>Worker: DocumentParser.parse()
    Worker-->>Server: Stream ParseResponse (Structured Chunks)
    deactivate Worker

    Server-->>User: 202 Accepted (Saga ID)

    note right of Server: Async Processing Starts
    par Async Ingestion
        Server->>Server: Generate Embeddings (Batcher)
        Server->>Saga: RunIngestionSaga(saga, chunks, nodes)
        activate Saga

        rect rgb(240, 248, 255)
            note over Saga, VectorDB: Step 1: Vector Insertion
            Saga->>VectorDB: InsertChunks()
            alt Insertion Failed
                Saga->>Saga: Update Status (Failed)
                Saga-->>Server: Error
            else Success
                Saga->>Saga: Update Step Status (Completed)
            end
        end

        rect rgb(255, 250, 240)
            note over Saga, GraphDB: Step 2: Indexing (RAPTOR & GraphRAG)
            Saga->>Saga: RaptorBuilder.BuildTree()
            Saga->>Saga: GraphExtractor.ExtractEntities()
            Saga->>GraphDB: InsertNodesAndEdges()

            alt Graph Insertion Failed
                Saga->>VectorDB: DeleteDocument (Compensating Transaction)
                Saga->>Saga: Update Status (Failed)
                Saga-->>Server: Error
            else Success
                Saga->>Saga: Update Status (Completed)
            end
        end
        deactivate Saga
    end
    deactivate Server
```
