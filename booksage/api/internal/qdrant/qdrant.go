package qdrant

import (
	"context"
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
	// The actual embedding dimension depends on the model used by the worker.
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

	log.Printf("[Qdrant] Created collection %q", c.collection)
	return nil
}

// InsertChunks upserts embedding chunks into the Qdrant collection.
// Each chunk is expected to be a map[string]any with "id", "text", and "vector" keys.
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

		points = append(points, &pb.PointStruct{
			Id:      pb.NewIDUUID(chunkID),
			Vectors: pb.NewVectors(vector...),
			Payload: pb.NewValueMap(map[string]any{
				"doc_id": docID,
				"text":   text,
			}),
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
