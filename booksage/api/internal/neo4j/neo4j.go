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

// InsertNodesAndEdges creates Chunk nodes in Neo4j for the given document.
// Each node is expected to be a map[string]any with "id", "text", and "type" keys.
func (c *Client) InsertNodesAndEdges(ctx context.Context, docID string, nodes []any) error {
	if len(nodes) == 0 {
		return nil
	}

	// Convert nodes to a format suitable for Cypher UNWIND
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

		nodeParams = append(nodeParams, map[string]any{
			"node_id":   nodeID,
			"doc_id":    docID,
			"text":      text,
			"node_type": nodeType,
		})
	}

	query := `
		UNWIND $nodes AS n
		MERGE (c:Chunk {node_id: n.node_id})
		SET c.doc_id = n.doc_id,
		    c.text = n.text,
		    c.node_type = n.node_type
	`

	_, err := neo4j.ExecuteQuery(ctx, c.driver, query,
		map[string]any{"nodes": nodeParams},
		neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(""),
	)
	if err != nil {
		return fmt.Errorf("neo4j insert failed for doc %s: %w", docID, err)
	}

	log.Printf("[Neo4j] Inserted %d nodes for doc %s", len(nodeParams), docID)
	return nil
}

// DeleteDocumentNodes deletes all nodes belonging to a document.
func (c *Client) DeleteDocumentNodes(ctx context.Context, docID string) error {
	query := `MATCH (c:Chunk {doc_id: $doc_id}) DETACH DELETE c`

	_, err := neo4j.ExecuteQuery(ctx, c.driver, query,
		map[string]any{"doc_id": docID},
		neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(""),
	)
	if err != nil {
		return fmt.Errorf("neo4j delete failed for doc %s: %w", docID, err)
	}

	log.Printf("[Neo4j] Deleted nodes for doc %s", docID)
	return nil
}

// DocumentExists checks if any Chunk nodes exist for the given document ID.
func (c *Client) DocumentExists(ctx context.Context, docID string) (bool, error) {
	query := `MATCH (c:Chunk {doc_id: $doc_id}) RETURN count(c) AS cnt LIMIT 1`

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

// Close closes the underlying Neo4j driver.
func (c *Client) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}
