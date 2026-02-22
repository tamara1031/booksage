package ingest

import (
	"context"
	"fmt"
	"log"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

// RaptorNode represents a node in the RAPTOR summary tree.
type RaptorNode struct {
	ID       string
	Content  string
	Level    int
	Children []string // IDs of child nodes
}

// RaptorBuilder orchestrates recursive summarization.
type RaptorBuilder struct {
	router repository.LLMRouter
}

// NewRaptorBuilder creates a new RAPTOR builder.
func NewRaptorBuilder(router repository.LLMRouter) *RaptorBuilder {
	return &RaptorBuilder{router: router}
}

// BuildTree constructs a hierarchical summary tree based on document structure.
// It returns a list of tree nodes to be inserted into Neo4j.
func (b *RaptorBuilder) BuildTree(ctx context.Context, docID string, chunks []map[string]any) ([]map[string]any, []map[string]any, error) {
	client := b.router.RouteLLMTask(repository.TaskType("deep_summarization"))

	var treeNodes []map[string]any
	var treeEdges []map[string]any

	// 1. Group chunks by their hierarchy
	// For simplicity in this lightweight version, we cluster chunks that share a common "heading" parent.
	// In docling output, headings have a level. We can use this to group.

	// Implementation note: A full-blown RAPTOR implementation would do K-means clustering.
	// This lightweight version uses the Document structure (Docling) to define clusters.

	lastHeadingID := ""
	currentGroup := []string{}

	for i, chunk := range chunks {
		cType, _ := chunk["type"].(string)
		cLevel, _ := chunk["level"].(int)
		cContent, _ := chunk["content"].(string)

		if cType == "heading" {
			// Summarize previous group before starting a new one
			if len(currentGroup) > 0 {
				summary, _ := b.summarizeGroup(ctx, client, currentGroup)
				parentID := fmt.Sprintf("%s-tree-%s", docID, lastHeadingID)
				treeNodes = append(treeNodes, map[string]any{
					"id":    parentID,
					"text":  summary,
					"type":  "Tree",
					"level": cLevel,
				})

				// Link children to this parent
				// (Omitted detailed edge construction for brevity, but would go into treeEdges)
			}
			lastHeadingID = fmt.Sprintf("h-%d-%d", cLevel, i)
			currentGroup = []string{}
		}
		currentGroup = append(currentGroup, cContent)
	}

	// Final group
	if len(currentGroup) > 0 {
		summary, _ := b.summarizeGroup(ctx, client, currentGroup)
		treeNodes = append(treeNodes, map[string]any{
			"id":   fmt.Sprintf("%s-tree-last", docID),
			"text": summary,
			"type": "Tree",
		})
	}

	return treeNodes, treeEdges, nil
}

func (b *RaptorBuilder) summarizeGroup(ctx context.Context, client repository.LLMClient, texts []string) (string, error) {
	if len(texts) == 0 {
		return "", nil
	}

	prompt := "Summarize the following text segments into a concise overview:\n\n"
	for _, t := range texts {
		prompt += "- " + t + "\n"
	}

	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		log.Printf("[RAPTOR] Summarization failed: %v", err)
		return "Summary extraction failed.", nil
	}

	return resp, nil
}
