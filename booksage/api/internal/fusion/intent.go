package fusion

import "strings"

// QueryIntent categorizes the cognitive nature of a user query.
type QueryIntent string

const (
	IntentSummary      QueryIntent = "summary"
	IntentDefinition   QueryIntent = "definition"
	IntentRelationship QueryIntent = "relationship"
	IntentComparison   QueryIntent = "comparison"
	IntentGeneral      QueryIntent = "general"
)

// IntentClassifier categorizes user queries using keyword heuristics.
// In production this could be replaced with an LLM-based classifier.
type IntentClassifier struct{}

// Classify determines the intent of a query using keyword matching.
func (c *IntentClassifier) Classify(query string) QueryIntent {
	q := strings.ToLower(query)

	switch {
	case containsAny(q, "summary", "summarize", "overview", "about"):
		return IntentSummary
	case containsAny(q, "definition", "define", "what is", "meaning"):
		return IntentDefinition
	case containsAny(q, "relationship", "connect", "between", "how does"):
		return IntentRelationship
	case containsAny(q, "compare", "difference", "vs", "versus"):
		return IntentComparison
	default:
		return IntentGeneral
	}
}

// containsAny checks if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// EngineWeights maps engine source names to their retrieval weights.
type EngineWeights map[string]float32

// RouteOperator provides intent-driven engine weights for fusion retrieval.
type RouteOperator struct {
	weights map[QueryIntent]EngineWeights
}

// NewRouteOperator creates a RouteOperator with default weight mappings.
func NewRouteOperator() *RouteOperator {
	return &RouteOperator{
		weights: map[QueryIntent]EngineWeights{
			IntentSummary: {
				"graph":  0.20,
				"tree":   0.70,
				"vector": 0.10,
			},
			IntentDefinition: {
				"graph":  0.20,
				"tree":   0.10,
				"vector": 0.70,
			},
			IntentRelationship: {
				"graph":  0.70,
				"tree":   0.10,
				"vector": 0.20,
			},
			IntentComparison: {
				"graph":  0.40,
				"tree":   0.40,
				"vector": 0.20,
			},
			IntentGeneral: {
				"graph":  0.34,
				"tree":   0.33,
				"vector": 0.33,
			},
		},
	}
}

// GetWeights returns the engine weights for a given intent.
func (r *RouteOperator) GetWeights(intent QueryIntent) EngineWeights {
	if w, ok := r.weights[intent]; ok {
		return w
	}
	return r.weights[IntentGeneral]
}
