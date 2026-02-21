import logging

logger = logging.getLogger(__name__)


class SelfRAGCritique:
    """Evaluates the retrieved nodes and the generated answer for usefulness and support."""

    def evaluate_retrieval(self, query: str, context_text: str) -> bool:
        """
        Determines if the retrieved context is relevant to the query to decide whether to continue.
        """
        logger.info("[Self-RAG] Critiquing retrieval relevance.")
        # In a real system, ask an LLM: "Is this context relevant?" [Relevant] or [Irrelevant]
        return "mock" in context_text.lower() or "context" in context_text.lower()

    def evaluate_generation(self, answer: str) -> str:
        """Determines if the answer is supported by the context or if it needs more info/rewrite."""
        logger.info("[Self-RAG] Critiquing generation for factual support.")
        # Returns critique token: [Fully Supported], [Partially Supported], or [No Support]
        if "mock" in answer.lower():
            return "[Fully Supported]"
        return "[No Support]"
