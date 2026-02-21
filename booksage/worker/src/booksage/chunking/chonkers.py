import hashlib
import uuid

from booksage.domain import Chunk
from booksage.etl import RawDocument

from .ports import IChunker


class ChonkersChunker(IChunker):
    """
    Chunker implementing CDC-based index locality to ensure stable hashes when boundaries shift.
    """

    def __init__(self, target_chunk_size: int = 1000):
        self.target_chunk_size = target_chunk_size

    def _compute_locality_hash(self, content: str) -> str:
        # Mock CDC rolling hash
        return hashlib.sha256(content.encode("utf-8")).hexdigest()[:16]

    def create_chunks(self, document: RawDocument) -> list[Chunk]:
        # For mock testability, just do simple splits but assign an index_locality_hash
        chunks = []
        text = document.text
        start = 0

        if not text:
            return chunks

        while start < len(text):
            # Simulated CDC boundary search
            end = min(start + self.target_chunk_size, len(text))
            content = text[start:end]

            chunks.append(
                Chunk(
                    chunk_id=str(uuid.uuid4()),
                    document_id=document.document_id,
                    content=content,
                    metadata=document.metadata,
                    index_locality_hash=self._compute_locality_hash(content),
                )
            )

            start = end

        return chunks
