# BookSage System Design & Architecture

BookSage is a RAG engine designed for high-precision knowledge synthesis from complex, long-context book documents. It synergizes **LightRAG** (incremental graph updates) and **Lite-BookRAG** (hierarchical structural awareness) into a clean, high-performance hybrid architecture.

---

## System Overview

The system is architected as a clean decoupling between cognitive orchestration and structural data processing:

### [BookSage Core](booksage/ARCHITECTURE.md)
- **Go API Orchestrator**: The "Cognitive Conductor." It manages LLM/Embedding inference (Ollama), Parallel Fusion Retrieval, and the **SQLite-based Ingestion Saga**.
- **Python ML Worker**: The "Structural ETL Engine." It specializes in high-precision layout analysis (Docling) and intelligent chunking.
- [API Reference](booksage/API.md)

### [BookScout Scraper](bookscout/ARCHITECTURE.md)
- **Data Acquisition**: Asynchronously pulls books from remote catalogs and submits them to the BookSage Saga API.
- **Persistent Watermarks**: Features state-aware tracking to ensure idempotent and reliable scraping.

---

## Core Philosophy

1. **Strict Separation of Concerns**: Go for Intelligence/Orchestration, Python for Layout/ETL.
2. **Dual-level Hybrid Retrieval**: Simultaneously extracts Low-level (Entities) and High-level (Themes) keywords for parallel search.
3. **Skyline Ranker**: A Pareto-optimal ranking engine that strictly prunes noise while preserving multi-engine results.
4. **Incremental Graph Updates**: Union-based graph evolution (Neo4j) that avoids global re-computation.
5. **Agentic Self-Correction**: Full **Self-RAG** implementation with context filtering and factual grounding critique.

---

## Infrastructure

- **Vector Engine**: **Qdrant** (Dense vectors, RAPTOR trees, ColBERT tensors).
- **Graph Engine**: **Neo4j** (Hierarchical Document Trees, Entities, and GT-Links).
- **State Store**: **SQLite** (Saga progress, Scraper watermarks).
- **Inference**: **Ollama** (Local), **Gemini** (Cloud).
