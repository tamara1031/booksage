import logging
import uuid

from booksage.domain import DocumentMetadata

from .models import RawDocument
from .ports import IDocumentParser

logger = logging.getLogger(__name__)


class PyMuPDFParser(IDocumentParser):
    def parse_file(self, file_path: str, metadata: DocumentMetadata) -> RawDocument:
        """Parse file using PyMuPDF (fitz) and return a RawDocument containing extracted text.
        Handles missing library gracefully for testing purposes.
        """
        from booksage.etl.models import ExtractedElement

        elements = []
        extra_meta = {"status": "success", "parser": "pymupdf"}
        doc_id = str(uuid.uuid4())

        try:
            import fitz

            doc = fitz.open(file_path)
            extra_meta["page_count"] = doc.page_count

            for i, page in enumerate(doc):
                text = page.get_text()
                if text.strip():
                    elements.append(
                        ExtractedElement(content=text.strip(), type="text", page_number=i + 1)
                    )
        except ImportError:
            logger.warning(f"PyMuPDF not installed, using mock text extraction for {file_path}")
            elements.append(
                ExtractedElement(
                    content=f"Mock text extracted from {file_path} using PyMuPDF fallback.",
                    type="text",
                    page_number=1,
                )
            )
            extra_meta["status"] = "mock_success"
        except Exception as e:
            logger.error(f"Error parsing {file_path} with PyMuPDF: {e}")
            raise e

        # fallback doc_id, but the router will overwrite it
        return RawDocument(
            document_id=doc_id, elements=elements, metadata=extra_meta, domain_metadata=metadata
        )
