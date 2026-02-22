from abc import ABC, abstractmethod

from booksage.domain.models import DocumentMetadata
from booksage.domain.models import RawDocument


class IDocumentParser(ABC):
    @abstractmethod
    def parse_file(self, file_path: str, metadata: DocumentMetadata) -> RawDocument:
        """Parse file and return a RawDocument containing extracted text."""
        pass
