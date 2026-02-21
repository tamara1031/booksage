from unittest.mock import MagicMock, patch

from booksage.domain.models import Chunk, DocumentMetadata, QueryContext
from booksage.generation.agent import AgenticGenerator
from booksage.retrieval.models import RetrievedNode


def test_self_rag_fallback_on_irrelevant_context():
    """Test SelfRAG fallback when retrieval is flagged as [Irrelevant]."""
    mock_retriever = MagicMock()
    # Mock some retrieved nodes
    meta = DocumentMetadata(book_id="1", title="Test Book")
    mock_node = RetrievedNode(
        chunk=Chunk(chunk_id="1", document_id="doc-1", content="some random text", metadata=meta),
        score=0.9,
        engine_source="Mock",
    )
    mock_retriever.retrieve.return_value = [mock_node]

    agent = AgenticGenerator(retriever=mock_retriever)

    # Patch the critique evaluate_retrieval to return False (Irrelevant)
    with patch.object(agent.critique, "evaluate_retrieval", return_value=False):
        context = QueryContext(original_query="What is the meaning of life?")
        answer = agent.generate_answer(context)

        # Verify fallback response is returned
        assert "could not find relevant information" in answer


def test_self_rag_fully_supported():
    """Test SelfRAG success path when generation is flagged as [Fully Supported]."""
    mock_retriever = MagicMock()
    meta = DocumentMetadata(book_id="1", title="Test Book")
    mock_node = RetrievedNode(
        chunk=Chunk(
            chunk_id="1", document_id="doc-1", content="meaning of life is 42", metadata=meta
        ),
        score=0.9,
        engine_source="Mock",
    )
    mock_retriever.retrieve.return_value = [mock_node]

    agent = AgenticGenerator(retriever=mock_retriever)

    # Patch critique methods to simulate a successful path
    with (
        patch.object(agent.critique, "evaluate_retrieval", return_value=True),
        patch.object(agent.critique, "evaluate_generation", return_value="[Fully Supported]"),
    ):
        context = QueryContext(original_query="What is the meaning of life?")
        answer = agent.generate_answer(context)

        # Verify the success token is appended
        assert "Verified by Self-RAG" in answer
