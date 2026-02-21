from abc import ABC, abstractmethod

from booksage.domain.models import Chunk
from booksage.etl.models import RawDocument


class IChunker(ABC):
    @abstractmethod
    def create_chunks(self, document: RawDocument) -> list[Chunk]:
        """Split a RawDocument into a list of Chunks."""
        pass
