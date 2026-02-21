from booksage.domain import QueryContext
from booksage.generation import AgenticGenerator
from booksage.retrieval import FusionRetriever, LightRAGEngine, RAPTOREngine


def test_generation_agent():
    engines = [LightRAGEngine(), RAPTOREngine()]  # Test with 2 engines
    retriever = FusionRetriever(engines=engines)
    agent = AgenticGenerator(retriever=retriever)

    context = QueryContext(original_query="Explain the concept of RAG.")
    answer = agent.generate_answer(context)

    # Check if answer was generated and critique token was appended
    assert "Based on the mock context" in answer
    assert "Verified by Self-RAG" in answer
    # Check decomposition occurred
    assert len(context.sub_queries) == 2
