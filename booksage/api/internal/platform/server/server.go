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

	// Initialize Ollama Clients (LLM Only - Embedding is now handled by Infinity)
	localLLMClient := llm.NewLocalOllamaClient(s.cfg.Model.OllamaHost, s.cfg.Model.OllamaLLM)

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
	sagaOrchestrator := ingest.NewSagaOrchestrator(qdrantClient, neo4jClient, bunStore, bunStore, localLLMClient, tensorClient)

	// Ingestion Service (Updated to use TensorEngine)
	ingestService := ingest.NewIngestionService(sagaOrchestrator, tensorClient)

	// Fusion Retriever (Uses Infinity for Tensors)
	fusionRetriever := query.NewFusionRetriever(qdrantClient, neo4jClient, tensorClient, localLLMClient)

	// Agentic Generator
	generator := query.NewGenerator(localLLMClient, fusionRetriever)

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
