package ingest

import (
	"fmt"
)

// GraphBuilder constructs graph nodes and edges for Neo4j.
type GraphBuilder struct{}

// NewGraphBuilder creates a new GraphBuilder.
func NewGraphBuilder() *GraphBuilder {
	return &GraphBuilder{}
}

// BuildGraphElements converts entities and relations into Neo4j-compatible maps.
func (b *GraphBuilder) BuildGraphElements(docID string, entities []Entity, relations []Relation, treeNodes []map[string]any) ([]map[string]any, []map[string]any) {
	var nodes []map[string]any
	var edges []map[string]any

	// 1. Add Tree Nodes (RAPTOR)
	if len(treeNodes) > 0 {
		nodes = append(nodes, treeNodes...)
	}

	// 2. Add Relations
	for _, rel := range relations {
		edges = append(edges, map[string]any{
			"from": fmt.Sprintf("%s-ent-%s", docID, rel.Source),
			"to":   fmt.Sprintf("%s-ent-%s", docID, rel.Target),
			"type": "RELATED_TO",
			"desc": rel.Description,
		})
	}

	// 3. Add Entities and GT-Links
	for _, ent := range entities {
		entID := fmt.Sprintf("%s-ent-%s", docID, ent.Name)

		nodes = append(nodes, map[string]any{
			"id":   entID,
			"text": ent.Description,
			"type": "Entity",
			"name": ent.Name,
		})

		// GT-Link: Connect entity to the document root
		edges = append(edges, map[string]any{
			"from": entID,
			"to":   docID,
			"type": "GT_LINK",
		})
	}

	return nodes, edges
}
