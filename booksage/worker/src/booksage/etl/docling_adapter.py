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
        from booksage.etl.models import ExtractedElement
        elements = []
        extra_meta = {"status": "success", "parser": "docling"}
        doc_id = str(uuid.uuid4())

        try:
            from docling.document_converter import DocumentConverter

            converter = DocumentConverter()
            result = converter.convert(file_path)
            text = result.document.export_to_markdown()
            if text.strip():
                elements.append(ExtractedElement(content=text.strip(), type="text", page_number=1))
        except ImportError:
            logger.warning(f"Docling not installed, using mock text extraction for {file_path}")
            elements.append(
                ExtractedElement(
                    content=f"Mock text extracted from {file_path} using Docling fallback. [Table] [Equation]",
                    type="text",
                    page_number=1,
                )
            )
            extra_meta["status"] = "mock_success"
        except Exception as e:
            logger.error(f"Error parsing {file_path} with Docling: {e}")
            raise e

        # fallback doc_id, but the router will overwrite it
        return RawDocument(
            document_id=doc_id, elements=elements, metadata=extra_meta, domain_metadata=metadata
        )
