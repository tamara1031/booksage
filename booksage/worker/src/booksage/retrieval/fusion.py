import asyncio
import logging

from booksage.domain import QueryContext

from .models import RetrievedNode
from .ports import IRetrievalEngine
from .router import IntentClassifier, RouteOperator

logger = logging.getLogger(__name__)


class FusionRetriever(IRetrievalEngine):
    """
    Orchestrator that runs LightRAG, RAPTOR, and ColBERTv2 concurrently and ensembles results.
    """

    def __init__(self, engines: list[IRetrievalEngine]):
        self.engines = engines
        self.classifier = IntentClassifier()
        self.operator = RouteOperator()

    async def _retrieve_async(
        self, engine: IRetrievalEngine, context: QueryContext
    ) -> tuple[str, list[RetrievedNode]]:
        # Wrap sync retrieval in async for concurrency. Return engine class name with results.
        nodes = await asyncio.to_thread(engine.retrieve, context)
        return engine.__class__.__name__, nodes

    def _normalize_scores(self, nodes: list[RetrievedNode]) -> list[RetrievedNode]:
        """Min-Max normalize scores for a given engine's node list so they fall between 0 and 1."""
        if not nodes:
            return nodes
        scores = [n.score for n in nodes]
        min_s, max_s = min(scores), max(scores)
        if max_s == min_s:
            for n in nodes:
                n.score = 1.0  # Or 0.5, if all scores are identical
            return nodes

        for n in nodes:
            n.score = (n.score - min_s) / (max_s - min_s)
        return nodes

    async def retrieve_concurrent(self, query_context: QueryContext) -> list[RetrievedNode]:
        """Run all engines concurrently and apply dynamic intent fusion."""
        # 1. Classify Intent
        if not query_context.intent:
            query_context.intent = self.classifier.classify(query_context.original_query)

        # 2. Get Route Weights
        weights = self.operator.get_weights(query_context.intent)
        logger.info(f"Fusion Intent: {query_context.intent.value} | Weights: {weights}")

        # 3. Parallel Retrieval
        tasks = [self._retrieve_async(engine, query_context) for engine in self.engines]
        results = await asyncio.gather(*tasks)

        # 4. Normalize, Weight, and Flatten Results
        ensembled_nodes = []
        node_tracker = {}  # node_id -> RetrievedNode map for combining scores

        for engine_name, res_nodes in results:
            weight = weights.get(engine_name, 0.33)
            # Normalize scores per engine before applying global weights
            norm_nodes = self._normalize_scores(res_nodes)

            for node in norm_nodes:
                weighted_score = node.score * weight
                node_id = node.chunk.chunk_id  # Use chunk_id for tracking unique nodes
                if node_id in node_tracker:
                    node_tracker[node_id].score += weighted_score
                else:
                    node.score = weighted_score
                    node_tracker[node_id] = node

        ensembled_nodes = list(node_tracker.values())

        # 5. Sort by final fused score
        ensembled_nodes.sort(key=lambda n: n.score, reverse=True)
        return ensembled_nodes

    def retrieve(self, query_context: QueryContext) -> list[RetrievedNode]:
        """Synchronous wrapper for interface compliance if needed."""
        return asyncio.run(self.retrieve_concurrent(query_context))
