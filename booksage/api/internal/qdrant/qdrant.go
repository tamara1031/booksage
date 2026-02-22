package qdrant

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"

	pb "github.com/qdrant/go-client/qdrant"
)

// Client implements the ingest.QdrantClient interface using the official Qdrant Go SDK.
type Client struct {
	client     *pb.Client
	collection string
}

// NewClient creates a new Qdrant client and ensures the target collection exists.
func NewClient(host string, port int, collection string) (*Client, error) {
	client, err := pb.NewClient(&pb.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant at %s:%d: %w", host, port, err)
	}

	c := &Client{
		client:     client,
		collection: collection,
	}

	// Ensure collection exists (create if not)
	if err := c.ensureCollection(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure collection %q: %w", collection, err)
	}

	log.Printf("[Qdrant] Connected to %s:%d, collection=%s", host, port, collection)
	return c, nil
}

// ensureCollection creates the collection if it does not already exist.
func (c *Client) ensureCollection(ctx context.Context) error {
	exists, err := c.client.CollectionExists(ctx, c.collection)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Create collection with a reasonable default vector size.
	// 768 is a common dimension for many embedding models (e.g. all-MiniLM-L6-v2).
	err = c.client.CreateCollection(ctx, &pb.CreateCollection{
		CollectionName: c.collection,
		VectorsConfig: pb.NewVectorsConfig(&pb.VectorParams{
			Size:     768,
			Distance: pb.Distance_Cosine,
		}),
	})
	if err != nil {
		return err
	}

	// Create payload index on doc_id for efficient filtering
	if indexErr := c.createPayloadIndex(ctx); indexErr != nil {
		log.Printf("[Qdrant] Warning: failed to create payload index: %v", indexErr)
	}

	log.Printf("[Qdrant] Created collection %q with payload index", c.collection)
	return nil
}

// createPayloadIndex creates keyword indexes on frequently filtered payload fields.
func (c *Client) createPayloadIndex(ctx context.Context) error {
	_, err := c.client.CreateFieldIndex(ctx, &pb.CreateFieldIndexCollection{
		CollectionName: c.collection,
		FieldName:      "doc_id",
		FieldType:      pb.PtrOf(pb.FieldType_FieldTypeKeyword),
	})
	return err
}

// InsertChunks upserts embedding chunks into the Qdrant collection.
// Each chunk is expected to be a map[string]any with "id", "text", "vector",
// and optionally "page_number" and "type" keys.
func (c *Client) InsertChunks(ctx context.Context, docID string, chunks []any) error {
	if len(chunks) == 0 {
		return nil
	}

	var points []*pb.PointStruct
	for i, chunk := range chunks {
		m, ok := chunk.(map[string]any)
		if !ok {
			return fmt.Errorf("chunk %d: expected map[string]any, got %T", i, chunk)
		}

		chunkID, _ := m["id"].(string)
		if chunkID == "" {
			chunkID = fmt.Sprintf("%s-chunk-%d", docID, i)
		}

		text, _ := m["text"].(string)

		vectorRaw, ok := m["vector"]
		if !ok {
			return fmt.Errorf("chunk %d: missing 'vector' key", i)
		}

		vector, err := toFloat32Slice(vectorRaw)
		if err != nil {
			return fmt.Errorf("chunk %d: %w", i, err)
		}

		// Build payload
		payload := map[string]any{
			"doc_id": docID,
			"text":   text,
		}
		if pageNum, ok := m["page_number"]; ok {
			payload["page_number"] = pageNum
		}
		if chunkType, ok := m["type"]; ok {
			payload["type"] = chunkType
		}

		// Generate a deterministic numeric ID from the chunk string ID
		pointID := deterministicID(chunkID)

		points = append(points, &pb.PointStruct{
			Id:      pb.NewIDNum(pointID),
			Vectors: pb.NewVectors(vector...),
			Payload: pb.NewValueMap(payload),
		})
	}

	_, err := c.client.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: c.collection,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("qdrant upsert failed: %w", err)
	}

	log.Printf("[Qdrant] Upserted %d points for doc %s", len(points), docID)
	return nil
}

// DeleteDocument deletes all points belonging to a document by filtering on doc_id payload.
func (c *Client) DeleteDocument(ctx context.Context, docID string) error {
	_, err := c.client.Delete(ctx, &pb.DeletePoints{
		CollectionName: c.collection,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Filter{
				Filter: &pb.Filter{
					Must: []*pb.Condition{
						pb.NewMatch("doc_id", docID),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("qdrant delete failed for doc %s: %w", docID, err)
	}

	log.Printf("[Qdrant] Deleted points for doc %s", docID)
	return nil
}

// Search performs a dense vector similarity search in the collection.
// Returns up to `limit` results with their text payloads and scores.
func (c *Client) Search(ctx context.Context, queryVector []float32, limit uint64) ([]SearchResult, error) {
	results, err := c.client.Query(ctx, &pb.QueryPoints{
		CollectionName: c.collection,
		Query:          pb.NewQuery(queryVector...),
		Limit:          pb.PtrOf(limit),
		WithPayload:    pb.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant search failed: %w", err)
	}

	var out []SearchResult
	for _, point := range results {
		text := ""
		docID := ""
		pageNum := int32(0)

		if val, ok := point.Payload["text"]; ok {
			text = val.GetStringValue()
		}
		if val, ok := point.Payload["doc_id"]; ok {
			docID = val.GetStringValue()
		}
		if val, ok := point.Payload["page_number"]; ok {
			pageNum = int32(val.GetIntegerValue())
		}

		out = append(out, SearchResult{
			ID:         fmt.Sprintf("%d", point.Id.GetNum()),
			Text:       text,
			DocID:      docID,
			PageNumber: pageNum,
			Score:      point.Score,
		})
	}

	return out, nil
}

// SearchResult represents a single search result from Qdrant.
type SearchResult struct {
	ID         string
	Text       string
	DocID      string
	PageNumber int32
	Score      float32
}

// DocumentExists checks if any points exist for the given document ID.
func (c *Client) DocumentExists(ctx context.Context, docID string) (bool, error) {
	result, err := c.client.Scroll(ctx, &pb.ScrollPoints{
		CollectionName: c.collection,
		Filter: &pb.Filter{
			Must: []*pb.Condition{
				pb.NewMatch("doc_id", docID),
			},
		},
		Limit: pb.PtrOf(uint32(1)),
	})
	if err != nil {
		return false, fmt.Errorf("qdrant scroll failed for doc %s: %w", docID, err)
	}

	return len(result) > 0, nil
}

// Close closes the underlying Qdrant gRPC connection.
func (c *Client) Close() error {
	return c.client.Close()
}

// deterministicID generates a deterministic uint64 from a string key using SHA256.
func deterministicID(key string) uint64 {
	h := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint64(h[:8])
}

// toFloat32Slice converts various numeric slice types to []float32.
func toFloat32Slice(v any) ([]float32, error) {
	switch vt := v.(type) {
	case []float32:
		return vt, nil
	case []float64:
		out := make([]float32, len(vt))
		for i, f := range vt {
			out[i] = float32(f)
		}
		return out, nil
	case []any:
		out := make([]float32, len(vt))
		for i, elem := range vt {
			switch n := elem.(type) {
			case float32:
				out[i] = n
			case float64:
				out[i] = float32(n)
			default:
				return nil, fmt.Errorf("element %d: unsupported type %T", i, elem)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported vector type %T", v)
	}
}
