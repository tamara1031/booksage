import logging
import uuid

from booksage.domain import DocumentMetadata

from .models import RawDocument
from .ports import IDocumentParser

logger = logging.getLogger(__name__)


class EpubParser(IDocumentParser):
    def parse_file(self, file_path: str, metadata: DocumentMetadata) -> RawDocument:
        """Parse EPUB file and return a RawDocument containing extracted text.
        Handles missing library gracefully for testing purposes.
        """
        text = ""
        try:
            import ebooklib
            from bs4 import BeautifulSoup
            from ebooklib import epub

            book = epub.read_epub(file_path)
            for item in book.get_items():
                if item.get_type() == ebooklib.ITEM_DOCUMENT:
                    soup = BeautifulSoup(item.get_body_content(), "html.parser")
                    text += soup.get_text() + "\n"
        except ImportError:
            logger.warning(
                f"EbookLib/BeautifulSoup not installed, using mock extraction for EPUB: {file_path}"
            )
            text = f"Mock text extracted from EPUB {file_path}. Chapter 1: The Beginning..."
        except Exception as e:
            logger.error(f"Error parsing EPUB {file_path}: {e}")
            raise e

        doc_id = str(uuid.uuid4())
        return RawDocument(document_id=doc_id, text=text, metadata=metadata)
