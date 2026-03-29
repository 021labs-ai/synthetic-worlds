package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	httpserver "github.com/021labs-ai/synthetic-worlds/internal/adapters/http"
	"github.com/021labs-ai/synthetic-worlds/internal/adapters/http/handlers"
	"github.com/021labs-ai/synthetic-worlds/internal/adapters/http/middleware"
	"github.com/021labs-ai/synthetic-worlds/internal/adapters/llm"
	"github.com/021labs-ai/synthetic-worlds/internal/adapters/postgres"
	"github.com/021labs-ai/synthetic-worlds/internal/adapters/redis"
	"github.com/021labs-ai/synthetic-worlds/internal/config"
	"github.com/021labs-ai/synthetic-worlds/internal/services"
)

func main() {
	// Initialize logger
	log, _ := zap.NewProduction()
	if os.Getenv("ENV") == "development" || os.Getenv("ENV") == "" {
		log, _ = zap.NewDevelopment()
	}
	defer log.Sync()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config", zap.Error(err))
	}

	ctx := context.Background()

	// Initialize Postgres
	log.Info("connecting to PostgreSQL")
	pgClient, err := postgres.NewClient(ctx, cfg.DB.PostgresURL)
	if err != nil {
		log.Fatal("failed to connect to PostgreSQL", zap.Error(err))
	}
	defer pgClient.Close()

	// Initialize Redis
	log.Info("connecting to Redis")
	redisClient, err := redis.NewClient(ctx, cfg.Redis.URL)
	if err != nil {
		log.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	log.Info("all connections established")

	// Initialize repositories
	worldRepo := postgres.NewSyntheticWorldRepository(pgClient)
	callRepo := postgres.NewSyntheticCallRepository(pgClient)
	traceRepo := postgres.NewTraceRepository(pgClient)
	syntheticState := redis.NewSyntheticState(redisClient)

	// Initialize example provider (backed by imported traces)
	exampleProvider := postgres.NewPostgresExampleProvider(traceRepo)

	// Initialize LLM client (BYOK)
	llmClient := llm.NewClient(cfg.LLM.AnthropicKey, cfg.LLM.OpenAIKey, cfg.LLM.XAIKey)

	// Initialize services
	syntheticService := services.NewSyntheticService(
		worldRepo,
		callRepo,
		syntheticState,
		llmClient,
		exampleProvider,
		log,
		cfg.World.TTLSeconds,
	)
	traceImportService := services.NewTraceImportService(traceRepo, log)

	// Initialize handlers
	syntheticHandler := handlers.NewSyntheticHandler(syntheticService)
	traceImportHandler := handlers.NewTraceImportHandler(traceImportService)

	// Initialize auth provider
	var authProvider middleware.StaticAuthProvider
	authProvider = *middleware.NewStaticAuthProvider(cfg.Auth.APIKey)

	// Initialize HTTP server
	server := httpserver.NewServer(cfg.Server.Port, cfg.Server.ReadTimeout, cfg.Server.WriteTimeout)
	httpserver.RegisterRoutes(server.App(), syntheticHandler, traceImportHandler, &authProvider)

	// Start server
	go func() {
		log.Info("starting server", zap.Int("port", cfg.Server.Port))
		if err := server.Start(); err != nil {
			log.Error("server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down")
	if err := server.Shutdown(); err != nil {
		log.Error("server shutdown error", zap.Error(err))
	}
	log.Info("server stopped")
}
