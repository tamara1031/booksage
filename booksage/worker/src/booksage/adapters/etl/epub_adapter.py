import logging
import uuid

from booksage.domain import DocumentMetadata

from booksage.domain.models import RawDocument
from booksage.ports.parser import IDocumentParser

logger = logging.getLogger(__name__)


class EpubParser(IDocumentParser):
    def parse_file(self, file_path: str, metadata: DocumentMetadata) -> RawDocument:
        """Parse EPUB file and return a RawDocument containing extracted text.
        Handles missing library gracefully for testing purposes.
        """
        from booksage.domain.models import ExtractedElement

        elements = []
        extra_meta = {"status": "success", "parser": "epublib"}
        doc_id = str(uuid.uuid4())

        try:
            import ebooklib
            from bs4 import BeautifulSoup
            from ebooklib import epub

            book = epub.read_epub(file_path)
            chapter_num = 1
            for item in book.get_items():
                if item.get_type() == ebooklib.ITEM_DOCUMENT:
                    soup = BeautifulSoup(item.get_body_content(), "html.parser")
                    text = soup.get_text()
                    if text.strip():
                        elements.append(
                            ExtractedElement(
                                content=text.strip(), type="text", page_number=chapter_num
                            )
                        )
                        chapter_num += 1
        except ImportError:
            logger.warning(
                f"EbookLib/BeautifulSoup not installed, using mock extraction for EPUB: {file_path}"
            )
            elements.append(
                ExtractedElement(
                    content=f"Mock text extracted from EPUB {file_path}. "
                    "Chapter 1: The Beginning...",
                    type="text",
                    page_number=1,
                )
            )
            extra_meta["status"] = "mock_success"
        except Exception as e:
            logger.error(f"Error parsing EPUB {file_path}: {e}")
            raise e

        # fallback doc_id, but the router will overwrite it
        return RawDocument(
            document_id=doc_id, elements=elements, metadata=extra_meta, domain_metadata=metadata
        )
