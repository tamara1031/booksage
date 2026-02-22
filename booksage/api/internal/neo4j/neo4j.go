package neo4j

import (
	"context"
	"fmt"
	"log"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
)

// Client implements the ingest.Neo4jClient interface using the official Neo4j Go driver.
type Client struct {
	driver neo4j.Driver
}

// NewClient creates a new Neo4j client and verifies connectivity.
func NewClient(ctx context.Context, uri, user, password string) (*Client, error) {
	driver, err := neo4j.NewDriver(uri, neo4j.BasicAuth(user, password, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver for %s: %w", uri, err)
	}

	// Verify connectivity
	if err := driver.VerifyConnectivity(ctx); err != nil {
		if closeErr := driver.Close(ctx); closeErr != nil {
			log.Printf("[Neo4j] Warning: failed to close driver after connectivity check: %v", closeErr)
		}
		return nil, fmt.Errorf("failed to verify Neo4j connectivity at %s: %w", uri, err)
	}

	log.Printf("[Neo4j] Connected to %s as %s", uri, user)
	return &Client{driver: driver}, nil
}

// InsertNodesAndEdges creates a Document root node and Chunk child nodes in Neo4j.
// Each node is expected to be a map[string]any with "id", "text", "type", and "page_number" keys.
func (c *Client) InsertNodesAndEdges(ctx context.Context, docID string, nodes []any) error {
	if len(nodes) == 0 {
		return nil
	}

	// Step 1: Ensure Document root node exists
	docQuery := `MERGE (d:Document {doc_id: $doc_id})`
	_, err := neo4j.ExecuteQuery(ctx, c.driver, docQuery,
		map[string]any{"doc_id": docID},
		neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(""),
	)
	if err != nil {
		return fmt.Errorf("neo4j document node creation failed for doc %s: %w", docID, err)
	}

	// Step 2: Create Chunk nodes with HAS_CHUNK edges to Document
	var nodeParams []map[string]any
	for i, node := range nodes {
		m, ok := node.(map[string]any)
		if !ok {
			return fmt.Errorf("node %d: expected map[string]any, got %T", i, node)
		}

		nodeID, _ := m["id"].(string)
		if nodeID == "" {
			nodeID = fmt.Sprintf("%s-node-%d", docID, i)
		}

		text, _ := m["text"].(string)
		nodeType, _ := m["type"].(string)
		if nodeType == "" {
			nodeType = "Chunk"
		}

		pageNumber := 0
		switch pn := m["page_number"].(type) {
		case int:
			pageNumber = pn
		case int32:
			pageNumber = int(pn)
		case int64:
			pageNumber = int(pn)
		case float64:
			pageNumber = int(pn)
		}

		nodeParams = append(nodeParams, map[string]any{
			"node_id":     nodeID,
			"doc_id":      docID,
			"text":        text,
			"node_type":   nodeType,
			"page_number": pageNumber,
		})
	}

	chunkQuery := `
		UNWIND $nodes AS n
		MERGE (c:Chunk {node_id: n.node_id})
		SET c.doc_id = n.doc_id,
		    c.text = n.text,
		    c.node_type = n.node_type,
		    c.page_number = n.page_number
		WITH c, n
		MATCH (d:Document {doc_id: n.doc_id})
		MERGE (d)-[:HAS_CHUNK]->(c)
	`

	_, err = neo4j.ExecuteQuery(ctx, c.driver, chunkQuery,
		map[string]any{"nodes": nodeParams},
		neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(""),
	)
	if err != nil {
		return fmt.Errorf("neo4j chunk insertion failed for doc %s: %w", docID, err)
	}

	log.Printf("[Neo4j] Inserted Document + %d Chunk nodes for doc %s", len(nodeParams), docID)
	return nil
}

// DeleteDocumentNodes deletes all nodes belonging to a document.
func (c *Client) DeleteDocumentNodes(ctx context.Context, docID string) error {
	query := `
		MATCH (d:Document {doc_id: $doc_id})
		OPTIONAL MATCH (d)-[:HAS_CHUNK]->(c:Chunk)
		DETACH DELETE c, d
	`

	_, err := neo4j.ExecuteQuery(ctx, c.driver, query,
		map[string]any{"doc_id": docID},
		neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(""),
	)
	if err != nil {
		return fmt.Errorf("neo4j delete failed for doc %s: %w", docID, err)
	}

	log.Printf("[Neo4j] Deleted Document + Chunk nodes for doc %s", docID)
	return nil
}

// DocumentExists checks if any Chunk nodes exist for the given document ID.
func (c *Client) DocumentExists(ctx context.Context, docID string) (bool, error) {
	query := `MATCH (d:Document {doc_id: $doc_id}) RETURN count(d) AS cnt LIMIT 1`

	result, err := neo4j.ExecuteQuery(ctx, c.driver, query,
		map[string]any{"doc_id": docID},
		neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(""),
	)
	if err != nil {
		return false, fmt.Errorf("neo4j existence check failed for doc %s: %w", docID, err)
	}

	if len(result.Records) == 0 {
		return false, nil
	}

	cnt, _, err := neo4j.GetRecordValue[int64](result.Records[0], "cnt")
	if err != nil {
		return false, fmt.Errorf("neo4j result parse failed: %w", err)
	}

	return cnt > 0, nil
}

// SearchChunks performs a text search on Chunk nodes using keyword matching.
// Returns up to `limit` results.
func (c *Client) SearchChunks(ctx context.Context, query string, limit int) ([]ChunkSearchResult, error) {
	cypher := `
		MATCH (c:Chunk)
		WHERE c.text CONTAINS $query
		RETURN c.node_id AS node_id, c.text AS text, c.doc_id AS doc_id, c.page_number AS page_number
		LIMIT $limit
	`

	result, err := neo4j.ExecuteQuery(ctx, c.driver, cypher,
		map[string]any{"query": query, "limit": limit},
		neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(""),
	)
	if err != nil {
		return nil, fmt.Errorf("neo4j search failed: %w", err)
	}

	var out []ChunkSearchResult
	for _, record := range result.Records {
		nodeID, _, _ := neo4j.GetRecordValue[string](record, "node_id")
		text, _, _ := neo4j.GetRecordValue[string](record, "text")
		docID, _, _ := neo4j.GetRecordValue[string](record, "doc_id")
		pageNumber, _, _ := neo4j.GetRecordValue[int64](record, "page_number")

		out = append(out, ChunkSearchResult{
			NodeID:     nodeID,
			Text:       text,
			DocID:      docID,
			PageNumber: int32(pageNumber),
			Score:      0.5, // Fixed score for text match (no ranking in CONTAINS)
		})
	}

	return out, nil
}

// ChunkSearchResult represents a search result from Neo4j.
type ChunkSearchResult struct {
	NodeID     string
	Text       string
	DocID      string
	PageNumber int32
	Score      float32
}

// Close closes the underlying Neo4j driver.
func (c *Client) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}
