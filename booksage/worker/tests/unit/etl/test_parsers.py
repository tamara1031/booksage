from unittest.mock import MagicMock, patch

from booksage.domain import DocumentMetadata
from booksage.etl import DoclingParser, PyMuPDFParser


def test_pymupdf_fallback():
    with patch("fitz.open") as mock_open:
        mock_doc = MagicMock()
        mock_page = MagicMock()
        mock_page.get_text.return_value = "Mock text"
        mock_doc.__iter__.return_value = [mock_page]
        mock_open.return_value = mock_doc

        parser = PyMuPDFParser()
        meta = DocumentMetadata(book_id="1", title="Test Book")
        doc = parser.parse_file("dummy.pdf", meta)
        assert doc.metadata.book_id == "1"
        assert "Mock text" in doc.text


def test_docling_fallback():
    with patch("docling.document_converter.DocumentConverter") as mock_converter_cls:
        mock_result = MagicMock()
        mock_result.document.export_to_markdown.return_value = "Mock text [Table] [Equation]"
        mock_converter_cls.return_value.convert.return_value = mock_result

        parser = DoclingParser()
        meta = DocumentMetadata(book_id="2", title="Test Book")
        doc = parser.parse_file("dummy.pdf", meta)
        assert doc.metadata.book_id == "2"
        assert "Mock text" in doc.text
