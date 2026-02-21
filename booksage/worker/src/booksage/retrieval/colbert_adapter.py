import logging
import uuid

from booksage.domain import Chunk, DocumentMetadata, QueryContext

from .models import RetrievedNode
from .ports import IRetrievalEngine

logger = logging.getLogger(__name__)


class ColBERTV2Engine(IRetrievalEngine):
    """Engine responsible for late-interaction rigorous token-level exact matching."""

    def retrieve(self, query_context: QueryContext) -> list[RetrievedNode]:
        logger.info(f"[ColBERTv2] Retrieving for {query_context.original_query}")
        # Mock retrieval
        chunk = Chunk(
            chunk_id=str(uuid.uuid4()),
            document_id="mock-doc-3",
            content=f"ColBERTv2 exact token match context for: {query_context.original_query}",
            metadata=DocumentMetadata(
                book_id="mock-book", title="Mock Exact Book", toc_path="/appendix"
            ),
        )
        return [RetrievedNode(chunk=chunk, score=0.95, engine_source="ColBERTv2")]
