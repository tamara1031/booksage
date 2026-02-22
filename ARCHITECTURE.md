# BookLore Architecture Index

This project is a polyglot monorepo containing multiple sub-projects. Please refer to the specific architecture documents for detailed design and implementation details.

## Sub-Projects

### 1. BookSage (RAG Engine)
**Path:** [`booksage/ARCHITECTURE.md`](booksage/ARCHITECTURE.md)
- **Role:** The core RAG engine for knowledge synthesis.
- **Components:** Go API Orchestrator (Intelligence), Python Worker (ETL).
- **Key Concepts:** Hybrid Retrieval, Skyline Ranking, Self-RAG, Saga Ingestion.

### 2. BookScout (Scraper)
**Path:** [`bookscout/ARCHITECTURE.md`](bookscout/ARCHITECTURE.md)
- **Role:** Autonomous book acquisition and cataloging.
- **Key Concepts:** Idempotent scraping, Watermarking, Async processing.

---

*Note: This root document serves only as a directory. The Single Source of Truth (SSOT) for each subsystem is located within its respective directory.*
