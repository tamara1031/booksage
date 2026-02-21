# BookSage System Design & Architecture

BookSage is an "Optimal Book-based RAG Generation Engine" that processes massive documents and complex logical hierarchies. The system adopts a **Hybrid Microservice Architecture** split between Go and Python to maximize both operational concurrency and machine learning performance.

---

## System Overview

The system is divided into two main sub-projects:

### [BookSage](booksage/ARCHITECTURE.md)
The core engine consisting of:
- **Go API Orchestrator**: The cognitive conductor and high-performance gateway.
- **Python ML Worker**: The heavy-lifting machine learning and ETL engine.
- [API Reference](booksage/API.md)

### [BookScout](bookscout/ARCHITECTURE.md)
The data collection component:
- **Scraper/Crawler**: Asynchronously pulls books from remote catalogs and submits them for ingestion.

---

## Core Philosophy

1. **Strict Separation of Concerns**: High-concurrency orchestration in Go, heavy model inference in Python.
2. **gRPC Interface**: Type-safe, high-performance communication between components.
3. **Intent-Driven Fusion**: Dynamic retrieval strategies based on user query intent.
4. **Agentic Self-Correction**: Self-RAG loops to minimize hallucinations and ensure factual accuracy.

---

## Infrastructure

- **Databases**: Qdrant (Vector) and Neo4j (Graph).
- **Deployment**: containerized using Docker and orchestrated via Docker Compose or Kubernetes.
