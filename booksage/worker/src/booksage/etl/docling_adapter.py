import logging
import uuid

from booksage.domain import DocumentMetadata

from .models import RawDocument
from .ports import IDocumentParser

logger = logging.getLogger(__name__)


class DoclingParser(IDocumentParser):
    def parse_file(self, file_path: str, metadata: DocumentMetadata) -> RawDocument:
        """Parse file using Docling to retain structural layout (H1 -> H2 -> Tree)."""
        from booksage.etl.models import ExtractedElement

        elements = []
        extra_meta = {"status": "success", "parser": "docling"}
        doc_id = str(uuid.uuid4())

        try:
            from docling.document_converter import DocumentConverter

            converter = DocumentConverter()
            result = converter.convert(file_path)
            doc = result.document

            # Iterate through document elements to capture structure
            for element, _level in doc.iterate_items():
                content = doc.export_to_markdown(item=element).strip()
                if not content:
                    continue

                e_type = "text"
                e_level = 0

                # Check for headings
                from docling_core.types.doc.document import ListItem, SectionHeaderItem, TableItem

                if isinstance(element, SectionHeaderItem):
                    e_type = "heading"
                    e_level = getattr(element, "level", 1)  # Default to 1 if level not present
                elif isinstance(element, TableItem):
                    e_type = "table"
                elif isinstance(element, ListItem):
                    e_type = "list"

                # Extract page number if available (from the first label if multiple)
                page_number = 1
                if element.prov and len(element.prov) > 0:
                    page_number = element.prov[0].page_no

                elements.append(
                    ExtractedElement(
                        content=content,
                        type=e_type,
                        page_number=page_number,
                        level=e_level,
                        metadata={"orig_type": element.__class__.__name__},
                    )
                )

        except ImportError:
            logger.warning(
                f"Docling not installed, using mock structural extraction for {file_path}"
            )
            elements.extend(
                [
                    ExtractedElement(content="# Mock Root", type="heading", level=1, page_number=1),
                    ExtractedElement(content="## Mock Sub", type="heading", level=2, page_number=1),
                    ExtractedElement(
                        content="Mock paragraph context.", type="text", level=0, page_number=1
                    ),
                ]
            )
            extra_meta["status"] = "mock_success"
        except Exception as e:
            logger.error(f"Error parsing {file_path} with Docling: {e}")
            raise e

        return RawDocument(
            document_id=doc_id, elements=elements, metadata=extra_meta, domain_metadata=metadata
        )
