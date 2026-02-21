from abc import ABC, abstractmethod

from booksage.domain.models import QueryContext


class IGenerationAgent(ABC):
    @abstractmethod
    def generate_answer(self, query_context: QueryContext) -> str:
        """Generate final answer using retrieval layers and autonomous critique loops."""
        pass
