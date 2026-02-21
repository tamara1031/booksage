from booksage.chunking import ChonkersChunker, FreeChunker
from booksage.domain import DocumentMetadata
from booksage.etl.models import ExtractedElement, RawDocument


def test_free_chunker():
    chunker = FreeChunker(chunk_size=10, chunk_overlap=2)
    meta = DocumentMetadata(book_id="1", title="Test Book")
    doc = RawDocument(
        document_id="123",
        elements=[
            ExtractedElement(
                content="This is a test document content for chunking.", type="text", page_number=1
            )
        ],
        domain_metadata=meta,
    )
    chunks = chunker.create_chunks(doc)
    assert len(chunks) > 0
    assert chunks[0].document_id == doc.document_id


def test_chonkers_chunker():
    chunker = ChonkersChunker(target_chunk_size=20)
    meta = DocumentMetadata(book_id="2", title="Test Docling")
    doc = RawDocument(
        document_id="456",
        elements=[
            ExtractedElement(
                content="This is another test document content.", type="text", page_number=1
            )
        ],
        domain_metadata=meta,
    )
    chunks = chunker.create_chunks(doc)
    assert len(chunks) > 0
    assert chunks[0].index_locality_hash is not None
