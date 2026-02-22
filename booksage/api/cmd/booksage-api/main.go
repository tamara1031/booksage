package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/booksage/booksage-api/internal/agent"
	"github.com/booksage/booksage-api/internal/config"
	"github.com/booksage/booksage-api/internal/database/bunstore"
	"github.com/booksage/booksage-api/internal/embedding"
	"github.com/booksage/booksage-api/internal/ingest"
	"github.com/booksage/booksage-api/internal/llm"
	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
	"github.com/booksage/booksage-api/internal/server"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	log.Println("Starting BookSage API Orchestrator...")

	// Load Configuration
	cfg := config.Load()

	// Connect to the Python ML Worker
	log.Printf("Connecting to ML Worker at %s...", cfg.WorkerAddr)
	conn, err := grpc.NewClient(cfg.WorkerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to worker: %v", err)
	}
	defer func() { _ = conn.Close() }()
	log.Printf("Connected successfully.")

	// ==========================================
	// Initialize Dependencies (Dependency Injection)
	// ==========================================

	ctx := context.Background()

	var geminiClient llm.LLMClient
	if !cfg.UseLocalOnlyLLM {
		if cfg.GeminiAPIKey == "" {
			log.Fatalf("[Error] BS_GEMINI_API_KEY is not set and BS_USE_LOCAL_ONLY_LLM is false. Cannot start Orchestrator.")
		}
		var err error
		geminiClient, err = llm.NewGeminiClient(ctx, cfg.GeminiAPIKey)
		if err != nil {
			log.Fatalf("[Error] Failed to initialize Gemini: %v", err)
		}
	}

	// Initialize Ollama Client
	localClient := llm.NewLocalOllamaClient(cfg.OllamaHost, cfg.OllamaModel)

	// Override Gemini with Local Client if requested
	if cfg.UseLocalOnlyLLM {
		log.Println("[System] 🏠 BS_USE_LOCAL_ONLY_LLM is true. Overriding Gemini with Local Ollama.")
		geminiClient = localClient
	}

	// Initialize the LLM Router
	llmRouter := llm.NewRouter(localClient, geminiClient)
	log.Printf("[System] 🛤️  LLM Router initialized (Cloud: %s | Local: %s)",
		geminiClient.Name(), localClient.Name())

	// Inject the Router into the Agentic Generator
	generator := agent.NewGenerator(llmRouter)

	// Initialize gRPC clients
	parserClient := pb.NewDocumentParserServiceClient(conn)
	embedClient := pb.NewEmbeddingServiceClient(conn)

	// Wrap embedClient in a Batcher (max 100 texts per gRPC batch)
	embedBatcher := embedding.NewBatcher(embedClient, 100)

	// Initialize Database Clients and Saga Orchestrator
	sqldb, err := sql.Open(sqliteshim.ShimName, "booksage.db")
	if err != nil {
		log.Fatalf("[Error] Failed to open sqlite: %v", err)
	}

	bunStore, err := bunstore.NewBunStore(sqldb, sqlitedialect.New())
	if err != nil {
		log.Fatalf("[Error] Failed to initialize Database: %v", err)
	}

	qdrantMock := ingest.NewMockQdrantClient()
	neo4jMock := ingest.NewMockNeo4jClient()
	sagaOrchestrator := ingest.NewOrchestrator(qdrantMock, neo4jMock, bunStore, bunStore)

	// ==========================================
	// Initialize and Start HTTP Server
	// ==========================================

	apiServer := server.NewServer(generator, embedBatcher, parserClient, sagaOrchestrator)
	mux := apiServer.RegisterRoutes()

	// Start server on port 8080
	log.Println("[System] 🌐 Starting REST API Server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("[Error] HTTP server failed: %v", err)
	}
}
