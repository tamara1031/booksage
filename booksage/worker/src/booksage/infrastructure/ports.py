from abc import ABC, abstractmethod

from booksage.domain.models import Chunk


class IVectorStore(ABC):
    @abstractmethod
    def add_chunks(self, chunks: list[Chunk]) -> None:
        """Store chunks (metadata and vectors) into the database."""
        pass

    @abstractmethod
    def search(self, query: str, filters: dict, top_k: int) -> list[Chunk]:
        """Search chunks using query and metadata filters."""
        pass
