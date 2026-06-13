// Command server is the composition root: it loads config, constructs the
// concrete dependencies (LLM, memory repository), wires them into the use case
// and HTTP delivery, and runs the server with graceful shutdown. This is the
// only place where the wiring of interfaces to implementations happens.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/vngcloud/agentt/internal/config"
	httpadapter "github.com/vngcloud/agentt/internal/adapter/http"
	"github.com/vngcloud/agentt/internal/domain/agent"
	"github.com/vngcloud/agentt/internal/domain/memory"
	gnadapter "github.com/vngcloud/agentt/internal/infra/greennode"
	"github.com/vngcloud/agentt/internal/infra/llm"
	"github.com/vngcloud/agentt/internal/infra/memstore"
	"github.com/vngcloud/agentt/internal/usecase"
	gnmemory "github.com/vngcloud/agentt/pkg/greennode/memory"
)

func main() {
	_ = godotenv.Load() // load .env if present; no-op in production when file is absent

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	// Select the memory backend. The use case is agnostic to which one.
	var repo memory.Repository
	switch cfg.MemoryBackend {
	case "greennode":
		repo = gnadapter.NewMemoryRepository(gnadapter.Config{
			MemoryID:      cfg.GreenNodeMemoryID,
			Namespace:     cfg.GreenNodeNamespace,
			ClientOptions: gnmemory.Options{},
		})
		logger.Info("memory backend: greennode", "memoryID", cfg.GreenNodeMemoryID)
	default:
		repo = memstore.New()
		logger.Info("memory backend: in-process (memstore)")
	}

	// Select the LLM provider. The use case only sees the agent.LLMClient port.
	var llmClient agent.LLMClient
	switch cfg.LLMProvider {
	case "anthropic":
		llmClient = llm.NewAnthropic(llm.AnthropicOptions{
			APIKey:       cfg.Anthropic.APIKey,
			BaseURL:      cfg.Anthropic.BaseURL,
			Model:        cfg.Anthropic.Model,
			MaxTokens:    cfg.Anthropic.MaxTokens,
			SystemPrompt: cfg.Anthropic.SystemPrompt,
		})
		logger.Info("llm provider: anthropic", "model", cfg.Anthropic.Model, "baseURL", cfg.Anthropic.BaseURL)
	case "openai":
		llmClient = llm.NewOpenAI(llm.OpenAIOptions{
			APIKey:       cfg.OpenAI.APIKey,
			BaseURL:      cfg.OpenAI.BaseURL,
			Model:        cfg.OpenAI.Model,
			MaxTokens:    cfg.OpenAI.MaxTokens,
			SystemPrompt: cfg.OpenAI.SystemPrompt,
		})
		logger.Info("llm provider: openai", "model", cfg.OpenAI.Model, "baseURL", cfg.OpenAI.BaseURL)
	default:
		llmClient = llm.NewEcho()
		logger.Info("llm provider: echo (stub)")
	}

	chat := usecase.NewChatService(llmClient, repo)

	router := httpadapter.NewRouter(httpadapter.RouterDeps{
		Chat:           chat,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Run the server until a termination signal arrives.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Error("server failed", "error", err)
		os.Exit(1)
	case sig := <-stop:
		logger.Info("shutting down", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
