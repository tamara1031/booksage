# BookSage Sub-project Architecture

BookSage is a RAG engine designed for high-precision knowledge synthesis from complex, long-context book documents. It operates on a sophisticated hybrid architecture that synergizes the strengths of **LightRAG** (incremental graph updates) and **Lite-BookRAG** (hierarchical structural awareness).

---

## 1. System Overview & Component Roles

The system is architected as a clean decoupling between cognitive orchestration and structural data processing.

### Go API Orchestrator (`api/`)
**Role:** The "Cognitive Conductor" and primary inference engine.
- **REST & SSE Server**: High-concurrency gateway facilitating real-time Agentic reasoning traces via Server-Sent Events.
- **Unified Inference Management**: 
    - **LLM/Embedding Orchestration**: Go directly calls local/cloud LLMs (via Ollama/Gemini) for all cognitive tasks, including embedding generation and entity extraction.
    - **Dual-Model Routing**: Intelligently switches between specialized local embedding models (e.g., `nomic-embed-text`) and reasoning models.
- **SOTA Retrieval & Ranking**:
    - **Dual-level Retrieval**: A LightRAG-inspired methodology that extracts both **Low-level (Entities)** and **High-level (Themes)** keywords in a single pass to drive parallel multi-engine searches across Vector DB (**Qdrant**) and Graph DB (**Neo4j**).
    - **Skyline Ranker**: A BookRAG-based Pareto-optimal ranking engine that merges disparate search results, prioritizing non-dominated chunks based on semantic relevance and structural importance.
- **Reliable Ingestion Saga**: Implements a **Saga Pattern** orchestrated via an internal **SQLite** engine to manage idempotent document processing, hash-derived deduplication, and state recovery.

### Python ML Worker (`worker/`)
**Role:** The "Structural ETL Engine."
- **Layout-Aware Parsing**: Specializes in high-precision layout analysis using **Docling**. It decomposes complex binaries (PDF/EPUB) into structural elements (headings, tables, lists) while preserving logical hierarchies.
- **Intelligent Chunking**: Maps the physical document layout to logical data units, passing hierarchical metadata to Go for **RAPTOR** recursive summarization and tree construction.
- **Offloaded Tensor Operations**: Optionally handles heavy tensor-interaction tasks (e.g., **ColBERTv2** late interaction) to maintain Orchestrator responsiveness.

---

## 2. Core Philosophy & Mechanisms

### A. Strict Separation of Concerns
We enforce the principle that the ML Worker is for **Data Extraction**, while the Go Orchestrator is for **Intelligence**. 
- Heavy model inference is centralized in Go.
- Python is strictly offloaded to CPU/GPU-intensive layout analysis and chunking.

### B. Synergy of BookRAG & LightRAG
1. **Hierarchical Knowledge Graph**: Neo4j stores not just entities, but a **Hierarchical Document Tree**. These are interlinked via **GT-Links** (Graph-Tree Links), allowing the system to traverse from abstract themes to specific mentions seamlessly.
2. **Incremental Graph Updates**: New knowledge is integrated via **Union-based incremental updates** (LightRAG style), allowing for seamless library expansion without the overhead of global re-indexing or community re-computation.
3. **Pareto-Optimal Fusion**: The Skyline Ranker merges vector similarity from **Qdrant** and graph centrality from **Neo4j**, ensuring that retrieved context is both microscopic (exact match) and macroscopic (themed context), strictly pruning noise.

### C. Agentic Evaluation (Self-RAG)
The generation phase is wrapped in an autonomous verification loop:
1. **Context Filtering**: Evaluates retrieved chunks for "relevance" before generation.
2. **Support Level Critique**: Validates if the answer is **Fully Supported**, **Partially Supported**, or has **No Support** in the retrieved context.
3. **Healing Mechanism**: Triggers re-generation or broader retrieval if support is inadequate.

---

## 3. Infrastructure & Tech Stack

- **Databases**:
    - **Qdrant** (Vector Store for Dense/ColBERTv2 embeddings and RAPTOR summaries)
    - **Neo4j** (Graph Store for Entities, Relations, and Hierarchical Trees)
    - **SQLite** (Relational Store for Saga state management and idempotency)
- **Models**: 
    - **Local**: Ollama (Embeddings/Reasoning)
    - **Cloud**: Gemini (Advanced reasoning & Agentic loops)
- **Deployment**: Containerized via Docker / Kubernetes

---

## 4. Class Design

The following diagram illustrates the core components of the BookSage Ingestion Pipeline and Query Engine.

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

        class EntityResolver {
            +ResolveEntity(ctx, ent)
        }

        class GraphBuilder {
            +BuildGraphElements(docID, entities, relations, treeNodes)
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
    SagaOrchestrator --> EntityResolver
    SagaOrchestrator --> GraphBuilder

    Generator --> FusionRetriever
    Generator --> SelfRAGCritique
    Generator --> LLMRouter

    FusionRetriever --> VectorRepository
    FusionRetriever --> GraphRepository

    DocumentParser o-- IDocumentParser
    DoclingParser ..|> IDocumentParser
    EpubParser ..|> IDocumentParser
```

---

## 5. Sequence Diagrams

### 5.1 Ingestion Flow
This flow details the interaction between the Go API Orchestrator (Saga) and the Python Parser Worker, highlighting the refactored strict separation of concerns where business logic (resolution/building) is delegated.

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant Server as API Server (Go)
    participant Saga as Saga Orchestrator (Go)
    participant Worker as Parser Worker (Python gRPC)
    participant Resolver as EntityResolver
    participant Builder as GraphBuilder
    participant VectorDB as Qdrant
    participant GraphDB as Neo4j

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

            loop Entity Resolution
                Saga->>Resolver: ResolveEntity(ent)
                Resolver-->>Saga: Match ID / False
            end

            Saga->>Builder: BuildGraphElements(nodes, relations)
            Builder-->>Saga: Neo4j Payload

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

### 5.2 RAG Generation Flow
This flow visualizes the advanced Chain-of-Retrieval (CoR), Fusion Retrieval, and Self-RAG critique loops.

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant Server as API Server (SSE)
    participant Gen as Generator (Agentic RAG)
    participant Ret as FusionRetriever
    participant Crit as SelfRAGCritique
    participant LLM as LLM Router (Gemini/Ollama)
    participant DB as Vector/Graph DBs

    User->>Server: POST /api/v1/query (JSON)
    activate Server
    Server->>Gen: GenerateAnswer(ctx, query, stream)
    activate Gen

    %% Step 1: CoR Decomposition
    Gen->>Server: Stream Event (Reasoning: "Analyzing...")
    Gen->>LLM: Generate(Prompt: Decompose Query)
    LLM-->>Gen: Sub-Queries [Q1, Q2...]

    loop For Each Sub-Query
        Gen->>Server: Stream Event (Reasoning: "Searching Qx")

        %% Step 2: Fusion Retrieval
        Gen->>Ret: Retrieve(Qx)
        activate Ret
        Ret->>DB: Vector Search (Dense)
        Ret->>DB: Graph Traversal (Sparse)
        Ret-->>Gen: Candidates [C1, C2...]
        deactivate Ret

        %% Step 3: Retrieval Critique
        loop For Each Candidate
            Gen->>Crit: EvaluateRetrieval(Qx, C_content)
            activate Crit
            Crit-->>Gen: Relevant (Bool)
            deactivate Crit

            alt Is Relevant
                Gen->>Server: Stream Event (Source: C_metadata)
                Gen->>Gen: Add to Context
            else Irrelevant
                Gen->>Server: Stream Event (Reasoning: "Filtered...")
            end
        end
    end

    %% Step 4: Generation
    Gen->>Server: Stream Event (Reasoning: "Generating...")
    Gen->>LLM: Generate(Prompt: Context + Query)
    LLM-->>Gen: Draft Answer

    %% Step 5: Generation Critique
    Gen->>Crit: EvaluateGeneration(Answer, Context)
    activate Crit
    Crit-->>Gen: Support Level (Fully/Partially/None)
    deactivate Crit

    alt No Support
        Gen->>Server: Stream Event (Reasoning: "Regenerating...")
        Gen->>LLM: Generate(Prompt: Strict Context Constraint)
        LLM-->>Gen: Final Answer
    end

    Gen->>Server: Stream Event (Answer: Final Content)
    deactivate Gen
    Server-->>User: Close Stream
    deactivate Server
```
