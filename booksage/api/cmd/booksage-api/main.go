package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/booksage/booksage-api/internal/agent"
	"github.com/booksage/booksage-api/internal/config"
	"github.com/booksage/booksage-api/internal/database/bunstore"
	"github.com/booksage/booksage-api/internal/embedding"
	"github.com/booksage/booksage-api/internal/fusion"
	"github.com/booksage/booksage-api/internal/ingest"
	"github.com/booksage/booksage-api/internal/llm"
	neo4jpkg "github.com/booksage/booksage-api/internal/neo4j"
	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
	qdrantpkg "github.com/booksage/booksage-api/internal/qdrant"
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

	// NOTE: Generator will be initialized after DB clients so we can inject the retriever.
	// See below after Qdrant/Neo4j initialization.

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

	qdrantClient, err := qdrantpkg.NewClient(cfg.QdrantHost, cfg.QdrantPort, cfg.QdrantCollection)
	if err != nil {
		log.Fatalf("[Error] Failed to connect to Qdrant: %v", err)
	}
	defer func() { _ = qdrantClient.Close() }()

	neo4jClient, err := neo4jpkg.NewClient(ctx, cfg.Neo4jURI, cfg.Neo4jUser, cfg.Neo4jPassword)
	if err != nil {
		log.Fatalf("[Error] Failed to connect to Neo4j: %v", err)
	}
	defer func() { _ = neo4jClient.Close(ctx) }()

	sagaOrchestrator := ingest.NewOrchestrator(qdrantClient, neo4jClient, bunStore, bunStore)

	// Initialize the Fusion Retriever (Qdrant + Neo4j + Embedding)
	fusionRetriever := fusion.NewFusionRetriever(qdrantClient, neo4jClient, embedBatcher)

	// Inject the Router and Retriever into the Agentic Generator
	generator := agent.NewGenerator(llmRouter, fusionRetriever)

	// ==========================================
	// Initialize and Start HTTP Server
	// ==========================================

	apiServer := server.NewServer(generator, embedBatcher, parserClient, sagaOrchestrator)
	handler := apiServer.RegisterRoutes()

	// ==========================================
	// Graceful Shutdown
	// ==========================================

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// Listen for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Println("[System] 🌐 Starting REST API Server on :8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Error] HTTP server failed: %v", err)
		}
	}()

	<-stop
	log.Println("[System] 🛑 Shutdown signal received. Draining connections...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Error] HTTP shutdown error: %v", err)
	}

	log.Println("[System] ✅ Server stopped gracefully.")
}
