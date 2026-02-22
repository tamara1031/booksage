# BookSage

![Status](https://img.shields.io/badge/Status-WIP-red)
![Architecture](https://img.shields.io/badge/Architecture-Go%2BPython_Hybrid-blue)
![Go](https://img.shields.io/badge/Go-1.25+-00ADD8)
![Python](https://img.shields.io/badge/Python-3.12-blue)

**BookSage** is an "Optimal Book-based RAG Generation Engine" that fully utilizes Large Language Models (LLMs) and advanced Retrieval-Augmented Generation (RAG) technologies. It specializes in processing book information that has long contexts and complex logical hierarchies.

## 🧠 Concept & Philosophy

### From Storage to Sage
This project aims to treat book libraries not just as "passive storage that needs to be actively accessed," but as an autonomous **Knowledge Base (a "Sage")** that organizes information and responds to interactions.

### Hallucination Reduction
To reduce hallucinations and derive expert-level reasoning, we have integrated SOTA hybrid architectures: **Lite-BookRAG** for structural hierarchy and **LightRAG** for incremental graph-based retrieval. Key techniques include Dual-level Retrieval, Self-RAG, and Pareto-optimal ranking.

## 🏗️ Architecture Highlights

### 1. Hybrid Architecture (Go + Python)
The system leverages a modern microservice-style monorepo structure containing two main subprojects:

**BookSage Core (`booksage/`)**
* **Go Orchestrator (`booksage/api/`)**: The cognitive logic engine. Managed by Go for superior concurrency. It handles **Embedding generation (Ollama)**, **Entity/Structure Extraction**, and high-performance parallel queries via Goroutines. It also manages **SQLite-based Saga status** for reliable ingestion.
* **Python ML Worker (`booksage/worker/`)**: Specializes in layout analysis and structured ETL using **Docling**. It provides the internal gRPC backend for high-precision parsing, chunking, and specialized tensor calculations (e.g., ColBERTv2 interaction).
* **gRPC Boundaries**: Communication occurs over strict Protocol Buffers. We use **gRPC Client Streaming** for efficient document ingestion (streaming chunks) and **Server Streaming** for real-time Agentic generation traces.

**BookScout Scraper (`bookscout/`)**
* A dedicated collection worker (intended for k8s CronJob) that pulls from remote OPDS feeds and pushes documents into the main BookSage API.

### 2. Local & Cloud LLM Hybrid Routing
* **Intelligent Dispatching**: Uses a weight-based internal Go Router to switch between Local models (Ollama for logic/embeddings) and Cloud APIs (Gemini 1.5 Pro).
* **Local Intelligence**: Directly generates dense vectors and knowledge graph entities using local **Ollama** clients, keeping low-latency operations safe and private.

### 3. SOTA Hybrid Retrieval & Ingestion
* **Dual-level Retrieval**: Extracts **Low-level (Entities)** and **High-level (Themes)** keywords in a single pass to drive multi-engine parallel search across Vector (Qdrant) and Graph (Neo4j) stores.
* **Skyline Ranker**: A Pareto-optimal ranker that merges results from disparate engines, pruning noise while preserving structural and semantic relevance.
* **Incremental Graph Updates**: Leverages LightRAG principles to update Neo4j incrementally (Union-based), linking entities to document trees via **GT-Links** without global re-computation.
* **Reliable Saga Management**: An internal **SQLite** engine manages ingestion state as a Finite State Machine (FSM), ensuring hash-based deduplication and idempotent processing.

### 4. Agentic Self-RAG Loop
* **Retrieval & Generation Critique**: Autonomously evaluates context relevance and generation grounding (Support Level), performing correction or re-generation when necessary.

## 📂 Project Structure

```text
booksage/
├── booksage/        # BookSage Core System
│   ├── api/         # Go Orchestrator (concurrent logic, LLM router, fusion retrieval)
│   ├── worker/      # Python ML Worker (ETL, chunking, embeddings)
│   ├── proto/       # Shared Protocol Buffers (booksage/v1/booksage.proto)
│   └── tests/       # Large E2E tests
├── bookscout/       # OPDS/Booklore Worker Job
├── docs/            # Detailed documentation
├── Makefile         # Root task runner (proto generation, build, run)
├── docker-compose.yml # Infrastructure (docker-compose)
└── .env.example     # Environment template
```
For deep architectural details, please refer to [`ARCHITECTURE.md`](ARCHITECTURE.md).

## ⚡ Quick Start

### Development Environment Setup (Docker)

BookSage runs as a dual-service application. The easiest way to run the complete environment is via Docker Compose:

```bash
# Clone the repository
git clone https://github.com/tamara1031/booksage.git
cd booksage

# Build and start the Go API, Python Worker, and Datastores
make up
```

The unified API will be available at `http://localhost:8080`.
The system is configured to route heavy reasoning to Gemini API and lightweight embeddings to your local Ollama instance (configured in `.env` with `SAGE_` prefix, see `.env.example`).

> **Note on Local-Only Mode:**
> You can now run BookSage entirely locally without Gemini by setting `SAGE_USE_LOCAL_ONLY_LLM=true` in your environment. This will route all requests, including agentic reasoning, to the local Ollama container.

## 📖 Documentation

* [System Design & Architecture](ARCHITECTURE.md)
* [API Reference](API.md)
* [Development Guide](DEVELOPMENT.md)