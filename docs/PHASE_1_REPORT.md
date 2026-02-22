# Phase 1: Architecture Analysis & Improvement Plan

## 1. Discrepancies & Design Judgment

### A. Logic Leakage in Go Saga (`booksage/api/internal/usecase/ingest/saga.go`)
- **Observation:** The `RunIngestionSaga` method contains significant business logic (vector search, entity comparison, graph node construction) mixed with orchestration logic.
- **Principle Violated:** Single Responsibility Principle (SRP) & Clean Architecture. The Saga should coordinate tasks, not perform domain logic.
- **Design Decision:** The Go Orchestrator should remain the "conductor", but domain logic (like "Resolve Entities") must be encapsulated in dedicated services (`EntityResolutionService` or `GraphExtractor`).

### B. Documentation Redundancy (`docs/` vs `booksage/`)
- **Observation:**
    - `docs/class_design.md` duplicates information found in `booksage/ARCHITECTURE.md`.
    - `root/ARCHITECTURE.md` is a high-level summary that overlaps with `booksage/ARCHITECTURE.md`.
    - Sequence diagrams (`docs/sequence_*.md`) are disconnected from the main architecture document.
- **Principle Violated:** DRY (Don't Repeat Yourself). Duplicate documentation leads to inconsistency.
- **Design Decision:** `booksage/ARCHITECTURE.md` will become the Single Source of Truth (SSOT) for the BookSage subsystem. Root `docs/` content will be merged or deleted.

### C. Python Worker (`booksage/worker/server.py`)
- **Observation:** The implementation strictly adheres to the "Parsing Only" role, delegating to `DoclingParser`. This is correct.
- **Status:** Approved. Minor code style improvements (imports) will be addressed during refactoring if needed, but no major architectural change required.

## 2. Refactoring Plan (Phase 2)

### A. Go Refactoring: Extract `EntityLinker` Logic
- **Goal:** Simplify `RunIngestionSaga` to pure workflow coordination.
- **Action:**
    - Create `EntityResolutionService` (or method on `GraphExtractor`) to handle vector search and entity matching logic.
    - Move graph node construction logic (Entities, Relations, GT-Links) into a dedicated builder/factory.
    - Update `SagaOrchestrator` to inject and call these new components.

### B. Python Refactoring (Minor)
- **Goal:** Clean up imports and structure.
- **Action:** Move local imports to top-level in `server.py` to adhere to PEP 8.

## 3. Documentation Cleanup Plan (Phase 3)

### A. Consolidation & Deletion
- **Merge:** `docs/class_design.md` content into `booksage/ARCHITECTURE.md`.
- **Merge:** `docs/sequence_ingest.md` and `docs/sequence_query.md` into `booksage/ARCHITECTURE.md` (simplified).
- **Delete:** The entire `docs/` folder (except perhaps `ROADMAP.md` if relevant, but likely move to root).
- **Simplify:** `root/ARCHITECTURE.md` to be a lightweight index pointing to `booksage/ARCHITECTURE.md` and `bookscout/ARCHITECTURE.md`.

---
**Status:** Awaiting Approval to Proceed to Phase 2.
