from pydantic import BaseModel, Field

from booksage.domain.models import DocumentMetadata


class ExtractedElement(BaseModel):
    content: str
    type: str = "text"
    page_number: int = 1
    level: int = 0  # H1=1, H2=2, etc. 0 = regular text
    metadata: dict[str, str] = Field(default_factory=dict)


class RawDocument(BaseModel):
    """Output of the ETL layer containing extracted elements and metadata."""

    document_id: str
    elements: list[ExtractedElement] = Field(default_factory=list)
    metadata: dict = Field(default_factory=dict)
    domain_metadata: DocumentMetadata
