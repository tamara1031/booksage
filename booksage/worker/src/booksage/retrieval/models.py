from pydantic import BaseModel

from booksage.domain.models import Chunk


class RetrievedNode(BaseModel):
    """Node returned from a Retrieval engine."""

    chunk: Chunk
    score: float
    engine_source: str
