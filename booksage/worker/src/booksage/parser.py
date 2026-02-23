import logging
import os
import uuid
from typing import Dict

from booksage.models import DocumentMetadata, ExtractedElement, RawDocument

logger = logging.getLogger(__name__)


class DoclingParser:
    def parse_file(self, file_path: str, metadata: DocumentMetadata) -> RawDocument:
        """Parse file using Docling to retain structural layout."""
        elements = []
        extra_meta = {"status": "success", "parser": "docling"}
        doc_id = str(uuid.uuid4())

        try:
            from docling.document_converter import DocumentConverter
            from docling_core.types.doc.document import (
                ListItem,
                SectionHeaderItem,
                TableItem,
            )

            converter = DocumentConverter()
            result = converter.convert(file_path)
            doc = result.document

            for element, _level in doc.iterate_items():
                content = doc.export_to_markdown(item=element).strip()
                if not content:
                    continue

                e_type = "text"
                e_level = 0

                if isinstance(element, SectionHeaderItem):
                    e_type = "heading"
                    e_level = getattr(element, "level", 1)
                elif isinstance(element, TableItem):
                    e_type = "table"
                elif isinstance(element, ListItem):
                    e_type = "list"

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
            logger.warning(f"Docling not installed, using mock for {file_path}")
            elements.extend(
                [
                    ExtractedElement(content="# Mock Root", type="heading", level=1, page_number=1),
                    ExtractedElement(content="Mock paragraph.", type="text", level=0, page_number=1),
                ]
            )
            extra_meta["status"] = "mock_success"

        return RawDocument(
            document_id=doc_id,
            elements=elements,
            metadata=extra_meta,
            domain_metadata=metadata,
        )


class EpubParser:
    def parse_file(self, file_path: str, metadata: DocumentMetadata) -> RawDocument:
        """Parse EPUB file."""
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
            logger.warning(f"EPUB libs not installed, using mock for {file_path}")
            elements.append(
                ExtractedElement(content="Mock EPUB text.", type="text", page_number=1)
            )
            extra_meta["status"] = "mock_success"

        return RawDocument(
            document_id=doc_id,
            elements=elements,
            metadata=extra_meta,
            domain_metadata=metadata,
        )


class PyMuPDFParser:
    def parse_file(self, file_path: str, metadata: DocumentMetadata) -> RawDocument:
        """Parse PDF using PyMuPDF."""
        import fitz
        elements = []
        extra_meta = {"status": "success", "parser": "pymupdf"}
        doc_id = str(uuid.uuid4())

        try:
            doc = fitz.open(file_path)
            extra_meta["page_count"] = doc.page_count
            for i, page in enumerate(doc):
                text = page.get_text()
                if text.strip():
                    elements.append(
                        ExtractedElement(content=text.strip(), type="text", page_number=i + 1)
                    )
        except Exception as e:
            logger.error(f"PyMuPDF error: {e}")
            raise e

        return RawDocument(
            document_id=doc_id,
            elements=elements,
            metadata=extra_meta,
            domain_metadata=metadata,
        )


class DocumentParser:
    """Coordinator that routes files to specific parser implementations."""

    def __init__(self):
        self.parsers = {
            ".pdf": DoclingParser(),
            ".docx": DoclingParser(),
            ".epub": EpubParser(),
        }

    def parse(self, file_path: str, file_type: str, document_id: str) -> Dict:
        """Main entry point for parsing a file."""
        _, ext = os.path.splitext(file_path)
        ext = ext.lower()

        parser = self.parsers.get(ext, DoclingParser())
        metadata = DocumentMetadata(
            book_id=document_id,
            title=os.path.basename(file_path),
            extra_attributes={"file_type": file_type},
        )

        logger.info(f"Parsing {file_path} with {parser.__class__.__name__}")
        raw_doc = parser.parse_file(file_path, metadata)

        return {
            "document_id": document_id,
            "extracted_metadata": raw_doc.metadata,
            "documents": [
                {
                    "content": el.content,
                    "type": el.type,
                    "page_number": el.page_number,
                    "level": el.level,
                    "extra_metadata": el.metadata,
                }
                for el in raw_doc.elements
            ],
        }
