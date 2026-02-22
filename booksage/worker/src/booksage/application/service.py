import logging
import os
import tempfile
from collections.abc import AsyncIterable
from concurrent.futures import ProcessPoolExecutor

import grpc

from booksage.config import load
from booksage.domain.models import DocumentMetadata
from booksage.adapters.etl.docling_adapter import DoclingParser
from booksage.adapters.etl.epub_adapter import EpubParser
from booksage.ports.parser import IDocumentParser

class DocumentParser:
    def __init__(self):
        # Setup the router/registry of parsers based on extension
        # Docling is now the primary parser for structured RAG
        self.parsers: dict[str, IDocumentParser] = {
            ".pdf": DoclingParser(),
            ".docx": DoclingParser(),
            ".epub": EpubParser(),
        }

    def parse(self, file_path: str, file_type: str, document_id: str) -> dict:
        """
        Routes the file to the appropriate ETL parser based on file extension.
        """
        _, ext = os.path.splitext(file_path)
        ext = ext.lower()

        parser = self.parsers.get(ext)
        if not parser:
            logger = logging.getLogger(__name__)
            logger.warning(f"No specific parser found for extension {ext}, using Docling fallback.")
            parser = DoclingParser()

        # Build basic domain metadata object
        metadata = DocumentMetadata(
            book_id=document_id,
            title=os.path.basename(file_path),
            extra_attributes={"file_type": file_type},
        )

        logging.info(
            f"Starting actual ETL parsing for {file_path} using {parser.__class__.__name__}"
        )

        # Execute the parse
        raw_doc = parser.parse_file(file_path, metadata)

        # Convert the RawDocument Pydantic model into the dictionary format
        # expected by the gRPC handler
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
