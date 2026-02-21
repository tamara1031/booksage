from pydantic import BaseModel

from booksage.domain.models import DocumentMetadata


class RawDocument(BaseModel):
    """Output of the ETL layer containing extracted text and metadata."""

    document_id: str
    text: str
    metadata: DocumentMetadata
