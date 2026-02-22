from typing import Any

from pydantic import BaseModel, Field


class DocumentMetadata(BaseModel):
    book_id: str
    title: str
    author: str | None = None
    toc_path: str | None = None
    extra_attributes: dict[str, Any] = Field(default_factory=dict)
