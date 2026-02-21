import logging

from booksage.domain import QueryContext
from booksage.retrieval import IRetrievalEngine

from .ports import IGenerationAgent
from .self_rag import SelfRAGCritique

logger = logging.getLogger(__name__)


class AgenticGenerator(IGenerationAgent):
    """Chain-of-Retrieval (CoR) generator wrapped with Self-RAG critique loops."""

    def __init__(self, retriever: IRetrievalEngine, max_iterations: int = 3):
        self.retriever = retriever
        self.max_iterations = max_iterations
        self.critique = SelfRAGCritique()

    def _decompose_query(self, original_query: str) -> list[str]:
        """Chain-of-Retrieval Step 1: Decompose query into sub-queries."""
        logger.info(f"[CoR] Decomposing query: {original_query}")
        # Mock LLM decomposition
        return [f"Aspect 1 of {original_query}", f"Aspect 2 of {original_query}"]

    def generate_answer(self, query_context: QueryContext) -> str:
        """Main loop: Decompose, Retrieve, Critique, Generate, Verify."""
        query_context.sub_queries = self._decompose_query(query_context.original_query)

        all_retrieved_nodes = []

        # Iterative retrieval (CoR)
        for sub_query in query_context.sub_queries:
            sub_context = QueryContext(
                original_query=sub_query, metadata_filters=query_context.metadata_filters
            )
            nodes = self.retriever.retrieve(sub_context)
            all_retrieved_nodes.extend(nodes)

        context_text = "\\n".join([n.chunk.content for n in all_retrieved_nodes])

        # Self-RAG Retrieval Critique
        is_relevant = self.critique.evaluate_retrieval(query_context.original_query, context_text)
        if not is_relevant:
            # Fallback or alert if no relevant context found
            return "I could not find relevant information in the library to answer your query."

        # Generation Mock
        logger.info("[Agent] Generating draft answer...")
        draft_answer = (
            f"Based on the mock context gathered, here is the answer for "
            f"{query_context.original_query}."
        )

        # Self-RAG Generation Critique
        support_token = self.critique.evaluate_generation(draft_answer)
        if support_token == "[Fully Supported]":
            return draft_answer + " (Verified by Self-RAG)"
        else:
            return draft_answer + " (Caution: May contain hallucinations - Refine triggered)"
