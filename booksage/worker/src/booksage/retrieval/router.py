from booksage.domain.models import QueryIntent


class IntentClassifier:
    """Mock LLM-based intent classifier to categorize user queries."""

    def classify(self, query: str) -> QueryIntent:
        """Classify query using basic heuristics (mocking LLM)."""
        query_lower = query.lower()

        if any(word in query_lower for word in ["summary", "summarize", "overview", "about"]):
            return QueryIntent.SUMMARY
        elif any(word in query_lower for word in ["definition", "define", "what is", "meaning"]):
            return QueryIntent.DEFINITION
        elif any(
            word in query_lower for word in ["relationship", "connect", "between", "how does"]
        ):
            return QueryIntent.RELATIONSHIP
        elif any(word in query_lower for word in ["compare", "difference", "vs", "versus"]):
            return QueryIntent.COMPARISON

        return QueryIntent.GENERAL


class RouteOperator:
    """Retrieval Engine Weight Operator based on Intent."""

    # Weights for (LightRAG, RAPTOR, ColBERTV2)
    WEIGHT_MAP = {
        QueryIntent.SUMMARY: {
            "LightRAGEngine": 0.20,
            "RAPTOREngine": 0.70,
            "ColBERTV2Engine": 0.10,
        },
        QueryIntent.DEFINITION: {
            "LightRAGEngine": 0.20,
            "RAPTOREngine": 0.10,
            "ColBERTV2Engine": 0.70,
        },
        QueryIntent.RELATIONSHIP: {
            "LightRAGEngine": 0.70,
            "RAPTOREngine": 0.10,
            "ColBERTV2Engine": 0.20,
        },
        QueryIntent.COMPARISON: {
            "LightRAGEngine": 0.40,
            "RAPTOREngine": 0.40,
            "ColBERTV2Engine": 0.20,
        },
        QueryIntent.GENERAL: {
            "LightRAGEngine": 0.34,
            "RAPTOREngine": 0.33,
            "ColBERTV2Engine": 0.33,
        },
    }

    def get_weights(self, intent: QueryIntent) -> dict[str, float]:
        """Return engine weights for a given intent."""
        return self.WEIGHT_MAP.get(intent, self.WEIGHT_MAP[QueryIntent.GENERAL])
