# Class Design: BookSage Architecture

This document visualizes the core architecture of BookSage, covering both the **Ingestion Pipeline** (Data Processing) and the **Query Engine** (RAG Generation).

## Component Overview

### Ingestion Pipeline
- **Go Saga Orchestrator**: Manages the distributed transaction of document ingestion.
- **Python Worker**: Specialized ETL engine using a Strategy pattern for file parsing.

### Query Engine
- **Generator**: Implements Agentic RAG logic (CoR, Self-RAG).
- **FusionRetriever**: Orchestrates hybrid search across Vector and Graph databases.
- **LLMRouter**: Routes tasks to appropriate LLM backends (Local vs Cloud).

## Class Diagram

```mermaid
classDiagram
    namespace Go_API_Orchestrator {
        class Server {
            +handleIngest(w, r)
            +handleQuery(w, r)
        }

        class SagaOrchestrator {
            +StartOrResumeIngestion(ctx, doc)
            +RunIngestionSaga(ctx, saga, chunks)
        }

        class Generator {
            +GenerateAnswer(ctx, query, stream)
            -decomposeQuery(ctx, query)
        }

        class FusionRetriever {
            +Retrieve(ctx, query)
        }

        class SelfRAGCritique {
            +EvaluateRetrieval(ctx, query, doc)
            +EvaluateGeneration(ctx, answer, context)
        }

        class LLMRouter {
            <<interface>>
            +RouteLLMTask(taskType)
        }

        class VectorRepository {
            <<interface>>
            +InsertChunks(ctx, chunks)
            +Search(ctx, vector)
        }

        class GraphRepository {
            <<interface>>
            +InsertNodesAndEdges(ctx, nodes)
        }
    }

    namespace Python_Worker {
        class DocumentParser {
            +parse(file_path)
        }
        class IDocumentParser { <<interface>> }
        class DoclingParser { +parse_file() }
        class EpubParser { +parse_file() }
    }

    %% Relationships
    Server --> SagaOrchestrator : Ingest
    Server --> Generator : Query

    SagaOrchestrator --> VectorRepository
    SagaOrchestrator --> GraphRepository

    Generator --> FusionRetriever
    Generator --> SelfRAGCritique
    Generator --> LLMRouter

    FusionRetriever --> VectorRepository
    FusionRetriever --> GraphRepository

    DocumentParser o-- IDocumentParser
    DoclingParser ..|> IDocumentParser
    EpubParser ..|> IDocumentParser
```
