import asyncio

from booksage.domain.models import Chunk, DocumentMetadata, QueryContext
from booksage.retrieval.fusion import FusionRetriever
from booksage.retrieval.models import RetrievedNode
from booksage.retrieval.ports import IRetrievalEngine


def create_mock_engine(name: str, mock_nodes: list[RetrievedNode]) -> IRetrievalEngine:
    class DynamicMockEngine(IRetrievalEngine):
        def retrieve(self, query_context: QueryContext) -> list[RetrievedNode]:
            return mock_nodes

    DynamicMockEngine.__name__ = name
    return DynamicMockEngine()


def test_fusion_operator_re_ranking():
    """Test the dynamic intent fusion logic leveraging the RouteOperator weights."""

    # Common mock chunk
    meta = DocumentMetadata(book_id="1", title="Test Book")
    c1 = Chunk(chunk_id="chunk-1", document_id="doc-1", content="summary text", metadata=meta)
    c2 = Chunk(chunk_id="chunk-2", document_id="doc-1", content="definition text", metadata=meta)

    # Create nodes with raw scores from different engines
    engine1 = create_mock_engine(
        "LightRAGEngine", [RetrievedNode(chunk=c1, score=0.8, engine_source="LightRAG")]
    )
    engine2 = create_mock_engine(
        "RAPTOREngine", [RetrievedNode(chunk=c2, score=0.9, engine_source="RAPTOR")]
    )

    retriever = FusionRetriever(engines=[engine1, engine2])

    context = QueryContext(original_query="Can you summarize this book?")

    nodes = asyncio.run(retriever.retrieve_concurrent(context))

    assert len(nodes) == 2
    # Verify the router detected SUMMARY intent
    assert context.intent.value == "summary"

    # Since RAPTOR weight is 0.70 and LightRAG is 0.20, chunk-2 (from RAPTOR) should rank highest.
    assert nodes[0].chunk.chunk_id == "chunk-2"
    assert nodes[1].chunk.chunk_id == "chunk-1"


def test_fusion_sync_wrapper():
    meta = DocumentMetadata(book_id="1", title="Test Book")
    c = Chunk(chunk_id="chunk-3", document_id="doc-1", content="general text", metadata=meta)
    engine = create_mock_engine(
        "ColBERTV2Engine", [RetrievedNode(chunk=c, score=0.5, engine_source="ColBERT")]
    )

    retriever = FusionRetriever(engines=[engine])
    # Use a query that doesn't hit any heuristics (e.g. no "what is", "compare", etc.)
    context = QueryContext(original_query="Please explain the background story.")
    nodes = retriever.retrieve(context)

    assert len(nodes) == 1
    assert context.intent.value == "general"
