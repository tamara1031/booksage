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
To reduce hallucinations (plausible lies) and derive advanced reasoning and accurate answers comparable to human experts, we have integrated the latest retrieval and generation methods (Fusion Retrieval, Self-RAG, CoR).

## 🏗️ Architecture Highlights

### 1. Hybrid Architecture (Go + Python)
The system leverages a modern microservice-style monorepo structure containing two main subprojects:

**BookSage Core (`booksage/`)**
* **Go Orchestrator (`booksage/api/`)**: Handles concurrent user requests, complex workflow orchestration, and high-performance parallel database queries (Fusion Retrieval) safely using lightweight Goroutines. Includes the **LLM Router** for intelligent task dispatching.
* **Python ML Worker (`booksage/worker/`)**: Dedicated to heavy Machine Learning workflows, including document parsing (ETL via Docling) and dense/tensor embedding operations (PyTorch), orchestrated via gRPC.
* **gRPC Boundaries**: The two worlds communicate over strictly defined Protocol Buffers (`booksage/proto/booksage/v1/`). We use **gRPC Client Streaming** instead of sending raw bytes all at once to circumvent the 4MB message limit and avoid memory spikes for massive PDF transfers (as we do not assume a shared storage volume between containers). Additionally, we heavily utilize **gRPC Server Streaming** for low-latency Agentic LLM responses to minimize Time-To-First-Token (TTFT).

**BookScout Scraper (`bookscout/`)**
* A dedicated collection worker (intended for k8s CronJob) that pulls from remote OPDS feeds and pushes documents into the main BookSage API.

### 2. Local & Cloud LLM Hybrid Routing
* **Intelligent Dispatching**: Uses an internal Go Router to switch between Local models and Cloud APIs (Gemini 1.5 Pro) for 2M context window reasoning and complex agentic loops. Local models are explicitly categorized by role: **Ollama** runs the generative LLM workload, while **ColBERT/embedding models** (hosted in the worker) handle the dense/sparse indexing and ranking.
* **Cost & Performance Efficiency**: Offloads cognitive-heavy tasks to the cloud, while retaining fast, free embedding operations locally.

### 3. Multi-Engine Fusion Retrieval
To breakthrough the limitations of a single vector search, we execute and ensemble (fusion) the following engines in parallel via Go:
* **Graph DB (Neo4j)**: Reasoning based on table of contents trees and entity graph relationships.
* **Vector DB (Qdrant)**: RAPTOR tree representations and ColBERTv2 strict token matching via Late Interaction.

### 4. Agentic Generation (CoR & Self-RAG)
* **Chain-of-Retrieval (CoR)**: Autonomously decomposes complex questions into sub-queries.
* **Self-RAG Critique**: Evaluates and corrects the "relevance" of retrieved information and "factual support" of generated answers at runtime.

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