# Phase 2: Go API Refactoring Plan

This document outlines the changes required to implement **Adaptive RAG** (Query Routing) and **Reflexion** (Self-Correction Loop) in the BookSage Go API Orchestrator (`booksage/api/`).

## 1. Component Refactoring

### A. Adaptive Router (`internal/usecase/query/adaptive_router.go`)
**Objective**: Implement Query Intent Classification.
- **Change**: Add `ClassifyIntent(ctx, query) (Intent, error)`.
- **Logic**: Use LLM to classify user query as `Simple` (Greeting/Fact) or `Complex` (Reasoning/Deep Dive).
- **Usage**: Before retrieval, route `Simple` queries to a lightweight generation path (or skip heavy retrieval) and `Complex` queries to the full Agentic RAG pipeline.

### B. Self-RAG Critique (`internal/usecase/query/self_rag.go`)
**Objective**: Implement Missing Context Detection.
- **Change**: Add `EvaluateMissingContext(ctx, answer, context) (status, missingInfo string)`.
- **Logic**: Use LLM to determine if the generated answer is `Sufficient` based on the context, or if there is `Missing Context`.
- **Output**: If missing, return a description of *what* is missing to guide the next retrieval step.

### C. Generator (`internal/usecase/query/generator.go`)
**Objective**: Implement Reflexion Loop & State Management.
- **Change**: Refactor `GenerateAnswer` to include a `for` loop with a `Max Iterations` guardrail (e.g., 3).
- **Logic**:
    1.  **Step 0 (Routing)**: Call `AdaptiveRouter.ClassifyIntent`.
    2.  **Loop Start**: Initialize loop (max 3 iterations).
    3.  **Step 1 (Extraction)**: Use `DualKeyExtractor`. If it's a retry (iteration > 0), append the `missingInfo` from the previous critique to the query prompt.
    4.  **Step 2 (Retrieval)**: Call `FusionRetriever.Retrieve`.
    5.  **Step 3 (Generation)**: Generate draft answer.
    6.  **Step 4 (Critique)**: Call `SelfRAGCritique.EvaluateMissingContext`.
        -   **If Sufficient**: Break loop, stream answer.
        -   **If Missing Context**:
            -   Stream event: "Reasoning: Missing info [details]... re-retrieving..."
            -   Append `missingInfo` to `queryHistory`.
            -   **Continue Loop** (Backtrack to Step 1).
    7.  **Loop End**: If max iterations reached, return best effort answer.

## 2. Implementation Steps

1.  **Update `AdaptiveRouter`**: Add `ClassifyIntent` method and `Intent` types.
2.  **Update `SelfRAGCritique`**: Add `EvaluateMissingContext` method.
3.  **Refactor `Generator`**:
    -   Introduce the `for` loop structure.
    -   Implement the state management for `missingInfo`.
    -   Add strict error handling and stream events for each step.
4.  **Verify**: Ensure unit tests cover the new logic (mocking LLM responses for "Missing Context").

## 3. Verification Plan

-   **Unit Tests**: Update `generator_test.go` to simulate a "Missing Context" scenario and verify the retry loop triggers.
-   **Integration**: Manually test with a complex query to observe the "Re-retrieving" reasoning event in the stream.
