from booksage.domain import QueryContext
from booksage.retrieval import ColBERTV2Engine, FusionRetriever, LightRAGEngine, RAPTOREngine


def test_fusion_retrieval():
    engines = [LightRAGEngine(), RAPTOREngine(), ColBERTV2Engine()]
    retriever = FusionRetriever(engines=engines)

    context = QueryContext(original_query="What is the architecture of BookSage?")
    nodes = retriever.retrieve(context)

    # We have 3 engines, each returning 1 mock node
    assert len(nodes) == 3
    # Check if they are sorted by score descending
    assert nodes[0].score >= nodes[1].score
    assert nodes[1].score >= nodes[2].score
    assert nodes[0].score > 0  # ensure it's not zero
