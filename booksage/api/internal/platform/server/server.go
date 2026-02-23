package server

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/booksage/booksage-api/internal/api"
	"github.com/booksage/booksage-api/internal/conf"
	"github.com/booksage/booksage-api/internal/ingest"
	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
	"github.com/booksage/booksage-api/internal/platform/database/bunstore"
	"github.com/booksage/booksage-api/internal/platform/infinity"
	"github.com/booksage/booksage-api/internal/platform/llm"
	neo4jpkg "github.com/booksage/booksage-api/internal/platform/neo4j"
	qdrantpkg "github.com/booksage/booksage-api/internal/platform/qdrant"
	"github.com/booksage/booksage-api/internal/ports"
	"github.com/booksage/booksage-api/internal/query"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Server struct {
	cfg        *conf.Config
	httpServer *http.Server
	dbConn     *sql.DB
}

func New(cfg *conf.Config) *Server {
	return &Server{
		cfg: cfg,
	}
}

func (s *Server) Run() error {
	ctx := context.Background()

	// Connect to the Python ML Worker
	log.Printf("Connecting to ML Worker at %s...", s.cfg.Client.WorkerAddr)
	conn, err := grpc.NewClient(s.cfg.Client.WorkerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	log.Printf("Connected successfully.")

	// ==========================================
	// Initialize Dependencies (Dependency Injection)
	// ==========================================

	var geminiClient ports.LLMClient
	if !s.cfg.Model.LocalOnly {
		if s.cfg.Model.GeminiKey == "" {
			log.Fatalf("[Error] SAGE_MODEL_GEMINI_KEY is not set and SAGE_MODEL_LOCAL_ONLY is false. Cannot start Orchestrator.")
		}
		var err error
		geminiClient, err = llm.NewGeminiClient(ctx, s.cfg.Model.GeminiKey)
		if err != nil {
			return err
		}
	}

	// Initialize Ollama Clients (LLM Only - Embedding is now handled by Infinity)
	localLLMClient := llm.NewLocalOllamaClient(s.cfg.Model.OllamaHost, s.cfg.Model.OllamaLLM)

	// Override Gemini with Local Client if requested
	if s.cfg.Model.LocalOnly {
		log.Println("[System] 🏠 SAGE_MODEL_LOCAL_ONLY is true. Overriding Gemini with Local Ollama.")
		geminiClient = localLLMClient
	}

	// Initialize the LLM Router (Ollama for simple tasks, Gemini/Ollama for complex)
	// Passing localLLMClient for both Embedding slots is fine as placeholder, but router logic should prioritize Infinity where applicable?
	// Actually, Router still handles "simple keyword extraction" via LLM.
	// Embedding task routing in Router might be obsolete if we inject TensorEngine directly.
	// We'll keep it for now but note that EmbeddingClient usage is deprecated in favor of TensorEngine.
	llmRouter := llm.NewRouter(localLLMClient, localLLMClient, geminiClient)
	log.Printf("[System] 🛤️  LLM Router initialized (Cloud: %s | Local LLM: %s)",
		geminiClient.Name(), localLLMClient.Name())

	// Initialize Infinity Tensor Engine
	tensorClient := infinity.NewClient(s.cfg.Model.InfinityURL)
	log.Printf("[System] ♾️  Infinity Tensor Engine initialized at %s", s.cfg.Model.InfinityURL)

	// Pull configured Ollama models at startup
	log.Printf("[System] 📥 Ensuring local LLM model '%s' is available...", s.cfg.Model.OllamaLLM)
	if err := localLLMClient.PullModel(ctx, s.cfg.Model.OllamaLLM); err != nil {
		log.Printf("[Warning] 📥 Failed to pull LLM model '%s': %v", s.cfg.Model.OllamaLLM, err)
	}

	// Initialize gRPC clients
	parserClient := pb.NewDocumentParserServiceClient(conn)

	// Initialize Database Clients and Saga Orchestrator
	s.dbConn, err = sql.Open(sqliteshim.ShimName, "booksage.db")
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := s.dbConn.Close(); closeErr != nil {
			log.Printf("[Warning] Failed to close database: %v", closeErr)
		}
	}()

	bunStore, err := bunstore.NewBunStore(s.dbConn, sqlitedialect.New())
	if err != nil {
		return err
	}

	qdrantClient, err := qdrantpkg.NewClient(s.cfg.DB.QdrantHost, s.cfg.DB.QdrantPort, s.cfg.DB.QdrantCollection)
	if err != nil {
		return err
	}
	defer func() { _ = qdrantClient.Close() }()

	neo4jClient, err := neo4jpkg.NewClient(ctx, s.cfg.DB.Neo4jURI, s.cfg.DB.Neo4jUser, s.cfg.DB.Neo4jPassword)
	if err != nil {
		return err
	}
	defer func() { _ = neo4jClient.Close(ctx) }()

	// --- Domain Services ---

	// Saga Orchestrator (Updated to use TensorEngine)
	sagaOrchestrator := ingest.NewSagaOrchestrator(qdrantClient, neo4jClient, bunStore, bunStore, llmRouter, tensorClient)

	// Ingestion Service (Updated to use TensorEngine)
	ingestService := ingest.NewIngestionService(sagaOrchestrator, tensorClient)

	// Fusion Retriever (Uses Infinity for Tensors)
	fusionRetriever := query.NewFusionRetriever(qdrantClient, neo4jClient, tensorClient, llmRouter)

	// Agentic Generator
	generator := query.NewGenerator(llmRouter, fusionRetriever)

	// --- Handlers ---

	ingestHandler := ingest.NewHandler(sagaOrchestrator, ingestService, parserClient)
	queryHandler := query.NewHandler(generator)

	// ==========================================
	// Initialize and Start HTTP Server
	// ==========================================

	handler := api.RegisterRoutes(ingestHandler, queryHandler)

	s.httpServer = &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// Listen for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Println("[System] 🌐 Starting REST API Server on :8080")
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Error] HTTP server failed: %v", err)
		}
	}()

	<-stop
	log.Println("[System] 🛑 Shutdown signal received. Draining connections...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Error] HTTP shutdown error: %v", err)
	}

	log.Println("[System] ✅ Server stopped gracefully.")
	return nil
}
