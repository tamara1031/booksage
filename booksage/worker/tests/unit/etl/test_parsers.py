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
        assert doc.domain_metadata.book_id == "1"
        assert len(doc.elements) > 0
        assert "Mock text" in doc.elements[0].content


def test_docling_hierarchical():
    with patch("docling.document_converter.DocumentConverter") as mock_converter_cls:
        mock_result = MagicMock()
        mock_doc = MagicMock()

        # Mock elements
        mock_h1 = MagicMock()
        mock_h1.__class__.__name__ = "SectionHeaderItem"
        mock_h1.level = 1
        mock_h1.prov = [MagicMock(page_no=1)]

        mock_p = MagicMock()
        mock_p.__class__.__name__ = "ParagraphItem"
        mock_p.prov = [MagicMock(page_no=1)]

        # mock_doc.iterate_items returns (element, level)
        mock_doc.iterate_items.return_value = [(mock_h1, 0), (mock_p, 0)]
        mock_doc.export_to_markdown.side_effect = ["# Title", "Body text"]

        mock_result.document = mock_doc
        mock_converter_cls.return_value.convert.return_value = mock_result

        parser = DoclingParser()
        meta = DocumentMetadata(book_id="2", title="Test Book")

        # We need to mock instance checks because SectionHeaderItem etc
        # are imported inside the method
        with patch("booksage.etl.docling_adapter.isinstance") as mock_isinstance:

            def side_effect(obj, cls):
                if "SectionHeaderItem" in str(cls) and obj == mock_h1:
                    return True
                return False

            mock_isinstance.side_effect = side_effect

            doc = parser.parse_file("dummy.pdf", meta)

        assert len(doc.elements) == 2
        assert doc.elements[0].type == "heading"
        assert doc.elements[0].level == 1
        assert doc.elements[0].content == "# Title"
        assert doc.elements[1].type == "text"
        assert doc.elements[1].level == 0
        assert doc.elements[1].content == "Body text"
