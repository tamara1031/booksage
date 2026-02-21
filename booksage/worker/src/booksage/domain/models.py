from enum import StrEnum
from typing import Any

from pydantic import BaseModel, Field


class QueryIntent(StrEnum):
    SUMMARY = "summary"
    DEFINITION = "definition"
    RELATIONSHIP = "relationship"
    COMPARISON = "comparison"
    GENERAL = "general"


class DocumentMetadata(BaseModel):
    book_id: str
    title: str
    author: str | None = None
    toc_path: str | None = None
    extra_attributes: dict[str, Any] = Field(default_factory=dict)


class Chunk(BaseModel):
    """Output of the Chunking layer, ready for indexing."""

    chunk_id: str
    document_id: str
    content: str
    metadata: DocumentMetadata
    index_locality_hash: str | None = None
    embedding: list[float] | None = None


class QueryContext(BaseModel):
    """Context for generation and retrieval layers."""

    original_query: str
    sub_queries: list[str] = Field(default_factory=list)
    metadata_filters: dict[str, Any] = Field(default_factory=dict)
    intent: QueryIntent | None = None


class GraphNodeType(StrEnum):
    CHUNK = "chunk"
    ENTITY = "entity"
    SUMMARY = "summary"
    FIGURE = "figure"


class GraphNode(BaseModel):
    node_id: str
    node_type: GraphNodeType
    content: str
    metadata: dict[str, Any] = Field(default_factory=dict)


class GraphEdgeType(StrEnum):
    BELONGS_TO_SUMMARY = "belongs_to_summary"
    REFERENCES_FIGURE = "references_figure"
    RELATES_TO = "relates_to"


class GraphEdge(BaseModel):
    source_id: str
    target_id: str
    edge_type: GraphEdgeType
    weight: float = 1.0
    description: str | None = None
