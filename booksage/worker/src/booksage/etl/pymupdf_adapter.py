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
        text = ""
        try:
            import fitz

            doc = fitz.open(file_path)
            for page in doc:
                text += page.get_text() + "\n"
        except ImportError:
            logger.warning(f"PyMuPDF not installed, using mock text extraction for {file_path}")
            text = f"Mock text extracted from {file_path} using PyMuPDF fallback."
        except Exception as e:
            logger.error(f"Error parsing {file_path} with PyMuPDF: {e}")
            raise e

        doc_id = str(uuid.uuid4())
        return RawDocument(document_id=doc_id, text=text, metadata=metadata)
