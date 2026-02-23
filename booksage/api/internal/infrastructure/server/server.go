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

	"github.com/booksage/booksage-api/internal/config"
	"github.com/booksage/booksage-api/internal/database/bunstore"
	"github.com/booksage/booksage-api/internal/domain/repository"
	"github.com/booksage/booksage-api/internal/embedding"
	"github.com/booksage/booksage-api/internal/infrastructure/llm"
	neo4jpkg "github.com/booksage/booksage-api/internal/infrastructure/neo4j"
	qdrantpkg "github.com/booksage/booksage-api/internal/infrastructure/qdrant"
	httpserver "github.com/booksage/booksage-api/internal/interface/http"
	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
	"github.com/booksage/booksage-api/internal/usecase/ingest"
	"github.com/booksage/booksage-api/internal/usecase/query"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Server struct {
	cfg        *config.Config
	httpServer *http.Server
	dbConn     *sql.DB
}

func New(cfg *config.Config) *Server {
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

	var geminiClient repository.LLMClient
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

	// Initialize Ollama Clients
	localLLMClient := llm.NewLocalOllamaClient(s.cfg.Model.OllamaHost, s.cfg.Model.OllamaLLM)
	localEmbedClient := llm.NewLocalOllamaClient(s.cfg.Model.OllamaHost, s.cfg.Model.OllamaEmbed)

	// Override Gemini with Local Client if requested
	if s.cfg.Model.LocalOnly {
		log.Println("[System] 🏠 SAGE_MODEL_LOCAL_ONLY is true. Overriding Gemini with Local Ollama.")
		geminiClient = localLLMClient
	}

	// Initialize the LLM Router
	llmRouter := llm.NewRouter(localLLMClient, localEmbedClient, geminiClient)
	log.Printf("[System] 🛤️  LLM Router initialized (Cloud: %s | Local LLM: %s | Local Embed: %s)",
		geminiClient.Name(), localLLMClient.Name(), localEmbedClient.Name())

	// Pull configured Ollama models at startup
	log.Printf("[System] 📥 Ensuring local models '%s' and '%s' are available...", s.cfg.Model.OllamaLLM, s.cfg.Model.OllamaEmbed)
	if err := localLLMClient.PullModel(ctx, s.cfg.Model.OllamaLLM); err != nil {
		log.Printf("[Warning] 📥 Failed to pull LLM model '%s': %v", s.cfg.Model.OllamaLLM, err)
	}
	if err := localEmbedClient.PullModel(ctx, s.cfg.Model.OllamaEmbed); err != nil {
		log.Printf("[Warning] 📥 Failed to pull Embed model '%s': %v", s.cfg.Model.OllamaEmbed, err)
	}

	// Initialize gRPC clients
	parserClient := pb.NewDocumentParserServiceClient(conn)

	// Route Embedding Task (Ollama by default as per Router logic)
	embeddingClient := llmRouter.RouteEmbeddingTask(llm.TaskEmbedding)
	if embeddingClient == nil {
		log.Fatalf("[Error] Failed to route embedding task. Ensure a valid LLM client is configured.")
	}

	// Wrap embeddingClient in a Batcher (max 100 texts per batch)
	embedBatcher := embedding.NewBatcher(embeddingClient, 100)

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

	// Saga Orchestrator (DDD Usecase)
	sagaOrchestrator := ingest.NewSagaOrchestrator(qdrantClient, neo4jClient, bunStore, bunStore, llmRouter, embeddingClient)

	// Initialize the Fusion Retriever (DDD Usecase)
	fusionRetriever := query.NewFusionRetriever(qdrantClient, neo4jClient, embedBatcher, llmRouter)

	// Inject the Router and Retriever into the Agentic Generator (DDD Usecase)
	generator := query.NewGenerator(llmRouter, fusionRetriever)

	// ==========================================
	// Initialize and Start HTTP Server
	// ==========================================

	apiServer := httpserver.NewServer(generator, embedBatcher, parserClient, sagaOrchestrator)
	handler := apiServer.RegisterRoutes()

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
