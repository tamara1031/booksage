# Class Design: Document Ingestion Pipeline

This document visualizes the structural relationship between the Go API Orchestrator (Core Logic) and the Python Worker (ETL Engine) involved in the document ingestion process.

## Component Overview

- **Go API Orchestrator**: Manages the ingestion lifecycle using the Saga pattern. It orchestrates vector embedding, RAPTOR tree construction, and GraphRAG entity extraction.
- **Python Worker**: Acts as a specialized ETL engine. It uses a Strategy pattern to select the appropriate parser (e.g., Docling for PDF/Docx, standard libraries for Epub) based on file type.

## Class Diagram

```mermaid
classDiagram
    namespace Go_API_Orchestrator {
        class Server {
            +handleIngest(w: http.ResponseWriter, r: *http.Request)
            +handleQuery(w: http.ResponseWriter, r: *http.Request)
        }

        class SagaOrchestrator {
            +StartOrResumeIngestion(ctx, doc)
            +RunIngestionSaga(ctx, saga, chunks, graphNodes)
            +GetDocumentStatus(ctx, hash)
        }

        class DocumentRepository {
            <<interface>>
            +CreateDocument(ctx, doc)
            +GetDocumentByHash(ctx, hash)
        }

        class SagaRepository {
            <<interface>>
            +CreateSaga(ctx, saga)
            +UpdateSagaStatus(ctx, id, version, status)
        }

        class VectorRepository {
            <<interface>>
            +InsertChunks(ctx, collection, chunks)
            +Search(ctx, vector, k)
        }

        class GraphRepository {
            <<interface>>
            +InsertNodesAndEdges(ctx, graphID, nodes, edges)
        }

        class RaptorBuilder {
            +BuildTree(ctx, docID, chunks)
        }

        class GraphExtractor {
            +ExtractEntitiesAndRelations(ctx, text)
        }
    }

    namespace Python_Worker {
        class DocumentParser {
            -parsers: dict[str, IDocumentParser]
            +parse(file_path, file_type, document_id)
        }

        class IDocumentParser {
            <<interface>>
            +parse_file(file_path, metadata)
        }

        class DoclingParser {
            +parse_file(file_path, metadata)
        }

        class EpubParser {
            +parse_file(file_path, metadata)
        }
    }

    Server --> SagaOrchestrator : uses
    SagaOrchestrator --> DocumentRepository : uses
    SagaOrchestrator --> SagaRepository : uses
    SagaOrchestrator --> VectorRepository : uses
    SagaOrchestrator --> GraphRepository : uses
    SagaOrchestrator --> RaptorBuilder : uses
    SagaOrchestrator --> GraphExtractor : uses

    DocumentParser o-- IDocumentParser : strategy
    DoclingParser ..|> IDocumentParser : implements
    EpubParser ..|> IDocumentParser : implements
```
