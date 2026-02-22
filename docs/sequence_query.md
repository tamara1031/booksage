# Sequence Diagram: RAG Generation Flow

This document visualizes the advanced RAG (Retrieval-Augmented Generation) process, including Chain-of-Retrieval (CoR), Fusion Retrieval, and Self-RAG critique loops.

## Flow Overview

1.  **Decomposition (CoR)**: The query is analyzed and broken down into simpler sub-queries if complex.
2.  **Fusion Retrieval**: Each sub-query is executed against both Vector (Qdrant) and Graph (Neo4j) databases.
3.  **Retrieval Critique (Self-RAG)**: Retrieved chunks are evaluated for relevance. Irrelevant chunks are filtered out.
4.  **Generation**: The LLM generates an answer based on the filtered context.
5.  **Generation Critique (Self-RAG)**: The answer is checked for grounding/support. If unsupported, regeneration is triggered with strict constraints.
6.  **Streaming**: All steps (reasoning, sources, answers) are streamed to the client via Server-Sent Events (SSE).

## Sequence Diagram

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
