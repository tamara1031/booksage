from abc import ABC, abstractmethod

from booksage.domain.models import QueryContext
from booksage.retrieval.models import RetrievedNode


class IRetrievalEngine(ABC):
    @abstractmethod
    def retrieve(self, query_context: QueryContext) -> list[RetrievedNode]:
        """Execute retrieval and return scored nodes."""
        pass
