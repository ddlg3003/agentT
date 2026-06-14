// Package app is the shared composition root: it constructs the concrete
// dependencies (LLM, memory, digest store, tools) from config and wires them
// into the use cases. Both the HTTP server (cmd/server) and the CLI runner
// (cmd/runner) build their world through Build, so wiring lives in exactly one
// place.
package app

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/vngcloud/agentt/internal/config"
	"github.com/vngcloud/agentt/internal/domain/agent"
	"github.com/vngcloud/agentt/internal/domain/memory"
	"github.com/vngcloud/agentt/internal/infra/digeststore"
	gnadapter "github.com/vngcloud/agentt/internal/infra/greennode"
	"github.com/vngcloud/agentt/internal/infra/llm"
	"github.com/vngcloud/agentt/internal/infra/memstore"
	"github.com/vngcloud/agentt/internal/infra/tools"
	"github.com/vngcloud/agentt/internal/usecase"
	gnmemory "github.com/vngcloud/agentt/pkg/greennode/memory"
)

// App holds the wired use cases and the resources that must be released on exit.
type App struct {
	Config config.Config
	Chat   *usecase.ChatService
	Digest *usecase.DigestService

	closers []func() error
}

// Build constructs all dependencies from config and returns a ready App. The
// caller must call Close when done to release the digest store.
func Build(cfg config.Config, logger *slog.Logger) (*App, error) {
	a := &App{Config: cfg}

	// Memory backend (conversation + long-term facts; also backs follow-up Q&A).
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

	// LLM provider. The digest loops need tool calling (agent.ToolCaller); all
	// providers implement it (echo degenerately).
	var llmClient agent.ToolCaller
	switch cfg.LLMProvider {
	case "anthropic":
		llmClient = llm.NewAnthropic(llm.AnthropicOptions{
			APIKey: cfg.Anthropic.APIKey, BaseURL: cfg.Anthropic.BaseURL,
			Model: cfg.Anthropic.Model, MaxTokens: cfg.Anthropic.MaxTokens,
			SystemPrompt: cfg.Anthropic.SystemPrompt,
		})
		logger.Info("llm provider: anthropic", "model", cfg.Anthropic.Model)
	case "openai":
		llmClient = llm.NewOpenAI(llm.OpenAIOptions{
			APIKey: cfg.OpenAI.APIKey, BaseURL: cfg.OpenAI.BaseURL,
			Model: cfg.OpenAI.Model, MaxTokens: cfg.OpenAI.MaxTokens,
			SystemPrompt: cfg.OpenAI.SystemPrompt,
		})
		logger.Info("llm provider: openai", "model", cfg.OpenAI.Model)
	default:
		llmClient = llm.NewEcho()
		logger.Info("llm provider: echo (stub) — agent loops cannot call tools; set an API key for real digests")
	}

	// Digest store (SQLite).
	store, err := digeststore.Open(cfg.DigestDBPath)
	if err != nil {
		return nil, fmt.Errorf("open digest store: %w", err)
	}
	a.closers = append(a.closers, store.Close)
	logger.Info("digest store: sqlite", "path", cfg.DigestDBPath)

	// Read-only data tools (shared across all loops).
	readTools := []agent.Tool{
		tools.NewKnowledge(cfg.MockDir),
		tools.NewBI(cfg.MockDir),
		tools.NewJira(cfg.MockDir),
		tools.NewGitlab(cfg.MockDir),
	}
	// Write-tool factory: builds update_digest bound to a digest + actor. Only
	// the follow-up loop is given this; daily/monthly stay read-only.
	newWriteTool := func(date, userID string) agent.Tool {
		return tools.NewUpdateDigest(store, date, userID, time.Now)
	}

	a.Chat = usecase.NewChatService(llmClient, repo)
	a.Digest = usecase.NewDigestService(llmClient, store, repo, readTools, newWriteTool)
	return a, nil
}

// Close releases resources (digest store handle) in reverse order.
func (a *App) Close() error {
	var firstErr error
	for i := len(a.closers) - 1; i >= 0; i-- {
		if err := a.closers[i](); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
