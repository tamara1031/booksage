import uuid

from booksage.domain import Chunk
from booksage.etl import RawDocument

from .ports import IChunker


class FreeChunker(IChunker):
    """Chunker implementing sliding window or basic semantic boundary chunking."""

    def __init__(self, chunk_size: int = 1000, chunk_overlap: int = 200):
        self.chunk_size = chunk_size
        self.chunk_overlap = chunk_overlap

    def create_chunks(self, document: RawDocument) -> list[Chunk]:
        chunks = []

        for element in document.elements:
            text = element.content
            if not text:
                continue

            start = 0
            while start < len(text):
                end = min(start + self.chunk_size, len(text))
                content = text[start:end]

                chunks.append(
                    Chunk(
                        chunk_id=str(uuid.uuid4()),
                        document_id=document.document_id,
                        content=content,
                        metadata=document.domain_metadata,
                    )
                )

                if end == len(text):
                    break

                start += self.chunk_size - self.chunk_overlap

        return chunks
