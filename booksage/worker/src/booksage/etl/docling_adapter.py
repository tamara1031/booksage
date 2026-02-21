import logging
import uuid

from booksage.domain import DocumentMetadata

from .models import RawDocument
from .ports import IDocumentParser

logger = logging.getLogger(__name__)


class DoclingParser(IDocumentParser):
    def parse_file(self, file_path: str, metadata: DocumentMetadata) -> RawDocument:
        """Parse file using Docling to retain table/equation layout context.
        Handles missing library gracefully for testing purposes.
        """
        text = ""
        try:
            from docling.document_converter import DocumentConverter

            converter = DocumentConverter()
            result = converter.convert(file_path)
            text = result.document.export_to_markdown()
        except ImportError:
            logger.warning(f"Docling not installed, using mock text extraction for {file_path}")
            text = (
                f"Mock text extracted from {file_path} using Docling fallback. [Table] [Equation]"
            )
        except Exception as e:
            logger.error(f"Error parsing {file_path} with Docling: {e}")
            raise e

        doc_id = str(uuid.uuid4())
        return RawDocument(document_id=doc_id, text=text, metadata=metadata)
