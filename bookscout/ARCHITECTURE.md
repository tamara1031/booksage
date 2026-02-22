# BookScout Sub-project Architecture

BookScout is a dedicated collection worker job for the BookSage system.

---

## 1. Components

### BookScout Scraper (`bookscout/`)
**Role:** A dedicated, state-aware collection worker Job.
- **OPDS/Booklore Integration**: Pulls books from remote catalog feeds asynchronously based on schedules.
- **State-Aware Watermarking**: Tracking of seen entries via a persistent state engine (SQLite) to avoid redundant downloads.
- **API Ingestion**: Submits downloaded books directly to the BookSage REST API (`/api/v1/ingest`), ensuring high-reliability ingestion through the Saga pattern.

---

## 2. Technology Stack

- **Language:** Go 1.24+
- **Input:** OPDS Feeds, External Book Scrapers
- **Output:** BookSage Ingestion API
