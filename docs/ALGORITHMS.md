# BookSage Inference Algorithm Visualization

This document explains the inference algorithm flow in "BookSage," an Agentic RAG system.
The system is architected around the **Go API Orchestrator**. All LLM inference (generation, embedding) is managed by the Go service. The Python Worker is specialized purely for structural document analysis (ETL) and does not perform inference.

## 1. Ingestion Pipeline Algorithms

This flow covers the process from document upload to storage in the Vector Store (Qdrant) and Graph Database (Neo4j).
It builds a multi-dimensional index by combining hierarchical summary generation (RAPTOR) and entity extraction (GraphRAG).

```mermaid
sequenceDiagram
    participant Client
    participant Go as Go Orchestrator
    participant Py as Python Worker
    participant LLM as Ollama (Local)
    participant Vec as Qdrant (Vector)
    participant Graph as Neo4j (Graph)

    Client->>Go: Upload Document (PDF/Markdown/etc.)
    activate Go

    %% Analysis Phase
    Note right of Go: [Analysis] Structural Data Extraction
    Go->>Py: Parse Request (Docling)
    activate Py
    Py-->>Go: Hierarchical Tree & Chunks
    deactivate Py

    %% Summary Inference Phase (RAPTOR)
    rect rgb(240, 248, 255)
        Note right of Go: [Summary Inference - RAPTOR]<br/>Recursive Summarization
        loop From leaf chunks to root sections
            Go->>LLM: Summarize Request
            activate LLM
            LLM-->>Go: Summary Text
            deactivate LLM
        end
    end

    %% Extraction Inference Phase (GraphRAG)
    rect rgb(255, 240, 245)
        Note right of Go: [Extraction Inference - GraphRAG]<br/>Entities & Relationships
        Go->>LLM: Extract Entities & Relations
        activate LLM
        LLM-->>Go: Entities JSON
        deactivate LLM
    end

    %% Vectorization Phase
    rect rgb(240, 255, 240)
        Note right of Go: [Vector Inference]<br/>Embedding Generation
        Go->>LLM: Generate Embeddings (Chunks & Entities)
        activate LLM
        LLM-->>Go: Vectors
        deactivate LLM
    end

    %% Persistence Phase
    Note right of Go: [Linking & Persistence]
    Go->>Vec: Entity Linking via Cosine Similarity
    Go->>Vec: Vector Upsert
    Go->>Graph: Incremental Persistence (GT-Link: Tree & Entity Binding)

    Go-->>Client: Ingestion Complete
    deactivate Go
```

### Algorithm Details

1.  **[Analysis] Structural Parsing via Python Worker**
    -   The Go Orchestrator sends document data to the Python Worker.
    -   The Python Worker uses specialized libraries like `Docling` to parse the document, extracting the chapter/section hierarchy (Tree) and micro-text chunks.
    -   *Constraint*: The Python Worker does not invoke LLMs.

2.  **[Summary Inference - RAPTOR] Recursive Summarization**
    -   The Go Orchestrator receives extracted chunks and invokes Ollama.
    -   Based on the **RAPTOR (Recursive Abstractive Processing for Tree-Organized Retrieval)** algorithm, it summarizes groups of child chunks to generate parent nodes (Section Summaries). This is repeated recursively up to the root, capturing global context.

3.  **[Extraction Inference - GraphRAG] Entity & Relationship Extraction**
    -   The Go Orchestrator invokes Ollama for each chunk to extract critical "Entities" (People, Places, Concepts) and their "Relationships."
    -   This constructs a knowledge graph representing semantic connections that traditional keyword search might miss.

4.  **[Vector Inference] Embedding Generation**
    -   The Go Orchestrator calls Ollama (Embedding models) to generate vector representations for text chunks, summaries, and extracted entities.

5.  **[Linking & Persistence] Qdrant & Neo4j Storage**
    -   **Qdrant**: Stores vectors. It uses cosine similarity to perform entity linking (deduplication) before upserting.
    -   **Neo4j**: Stores the document hierarchy (Tree nodes) and extracted entities. The **GT-Link (Graph-Tree Link)** algorithm interlinks the structural tree and the semantic graph, enabling cross-modal retrieval.

---

## 2. Query & Generation Pipeline Algorithms

This flow covers the process from receiving a user query to generating an optimal response.
It features adaptive routing, multi-engine retrieval, and a self-evaluation loop.

```mermaid
sequenceDiagram
    participant Client
    participant Go as Go Orchestrator
    participant LLM as Ollama (Local)
    participant Vec as Qdrant (Vector)
    participant Graph as Neo4j (Graph)

    Client->>Go: User Search Query
    activate Go

    %% Routing
    rect rgb(255, 250, 205)
        Note right of Go: [Routing Inference]<br/>Adaptive Routing
        Go->>LLM: Query Complexity Analysis
        activate LLM
        LLM-->>Go: Simple / Complex / Agentic
        deactivate LLM
    end

    %% Keyword Extraction
    rect rgb(230, 230, 250)
        Note right of Go: [Keyword Extraction - LightRAG]<br/>Dual-level Retrieval
        Go->>LLM: Extract Low/High-level Keys
        activate LLM
        LLM-->>Go: Specific Entities & Broad Themes
        deactivate LLM
    end

    %% Parallel Search
    par Parallel Search
        Go->>Vec: Dense Retrieval (Vector Search)
        and
        Go->>Graph: Graph Traversal (Cypher Query)
    end
    Vec-->>Go: Similar Chunks
    Graph-->>Go: Related Entities & Relations

    %% Ranking
    Note right of Go: [Algorithmic Evaluation - Skyline Ranker]<br/>Pareto Optimality (Vector vs Graph)
    Go->>Go: Context Selection & Noise Pruning

    %% Generation and Self-Correction
    rect rgb(255, 228, 225)
        loop Self-RAG Loop
            Note right of Go: [Generation & Critique - Self-RAG]
            Go->>LLM: Generate Answer (with Context)
            activate LLM
            LLM-->>Go: Answer Draft
            deactivate LLM

            Go->>LLM: Self-Critique (Faithfulness & Relevance)
            activate LLM
            LLM-->>Go: Evaluation Score (Pass/Fail)
            deactivate LLM

            alt Failed Evaluation
                Go->>Go: Refine Query/Context & Retry
            else Passed Evaluation
                Go-->>Client: Final Answer
            end
        end
    end
    deactivate Go
```

### Algorithm Details

1.  **[Routing Inference] Adaptive Routing**
    -   Upon receiving a query, the Go Orchestrator invokes Ollama to analyze its nature.
    -   It classifies the query as a simple fact-check, a complex multi-part reasoning task, or an "Agentic" task requiring multiple steps, dynamically switching the downstream pipeline.

2.  **[Keyword Extraction - LightRAG] Dual-level Retrieval**
    -   The Go Orchestrator calls Ollama to extract search keys.
    -   Following the **LightRAG** philosophy, it extracts "Low-level keys" (specific entity names) and "High-level keys" (abstract themes/concepts) simultaneously to capture both microscopic details and macroscopic context.

3.  **[Parallel Search] Vector & Graph Parallel Search**
    -   Using Go's `Goroutines`, the system performs parallel searches across Qdrant (dense vector search) and Neo4j (graph traversal).
    -   This ensures context is gathered from both semantic similarity and structural relationship vantage points.

4.  **[Algorithmic Evaluation] Skyline Ranker**
    -   The system filters the vast amount of candidate context using internal logic.
    -   The **Skyline Ranker** algorithm evaluates candidates across two axes: "Vector Similarity Score" and "Graph Centrality Score." It selects the **Pareto-optimal** set (where no candidate is strictly outperformed by another in both dimensions), strictly pruning noise to optimize the LLM's context window.

5.  **[Generation & Critique] Self-RAG Self-Correction Loop**
    -   The selected context is sent to Ollama to generate a response.
    -   The generated answer is immediately critiqued using Ollama for **Faithfulness** (is it grounded?) and **Relevance** (does it answer the query?).
    -   If the criteria are not met, the Go Orchestrator refined the search query or re-selects context and retries the generation loop.
