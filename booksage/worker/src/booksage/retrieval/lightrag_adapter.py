import logging
import uuid

from booksage.domain import Chunk, DocumentMetadata, QueryContext

from .models import RetrievedNode
from .ports import IRetrievalEngine

logger = logging.getLogger(__name__)


class LightRAGEngine(IRetrievalEngine):
    """Engine responsible for multi-hop graph-based reasoning and ToC context."""

    def retrieve(self, query_context: QueryContext) -> list[RetrievedNode]:
        logger.info(f"[LightRAG] Retrieving for {query_context.original_query}")
        # Mock retrieval
        chunk = Chunk(
            chunk_id=str(uuid.uuid4()),
            document_id="mock-doc-1",
            content=f"LightRAG graph context for: {query_context.original_query}",
            metadata=DocumentMetadata(
                book_id="mock-book", title="Mock Graph Book", toc_path="/ch1"
            ),
        )
        return [RetrievedNode(chunk=chunk, score=0.85, engine_source="LightRAG")]
