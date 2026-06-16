// Package config loads application configuration from environment variables
// with sensible defaults. Twelve-factor style: config lives in the environment,
// not in code.
package config

import (
	"os"
	"strings"
)

// Config is the resolved application configuration.
type Config struct {
	// Port is the HTTP listen port.
	Port string
	// AllowedOrigins is the CORS allowlist for the frontend.
	AllowedOrigins []string

	// MemoryBackend selects the memory.Repository implementation:
	// "greennode" (AgentBase) or "memory" (in-process). Defaults to "memory"
	// unless GreenNode credentials are present.
	MemoryBackend string
	// GreenNodeMemoryID is the AgentBase memory resource id to use.
	GreenNodeMemoryID string
	// GreenNodeNamespace scopes long-term records.
	GreenNodeNamespace string

	// StaticDir, if non-empty, serves the built frontend from this directory
	// (SPA fallback to index.html). Set STATIC_DIR=/app/dist in production.
	StaticDir string
	// MockDir is the base directory for the mock data tools read from
	// (bi/, jira/, gitlab/, knowledge/). Swapped for real clients later.
	MockDir string
	// DigestDBPath is the SQLite file the digest store persists to.
	DigestDBPath string

	// LLMProvider selects the agent.LLMClient implementation: "echo" (stub,
	// default), "anthropic" (Claude), or "openai". Auto-selects based on which
	// API key is present when LLM_PROVIDER is unset.
	LLMProvider string
	// Anthropic configures the Claude provider (used when LLMProvider=anthropic).
	Anthropic AnthropicConfig
	// OpenAI configures the OpenAI provider (used when LLMProvider=openai).
	OpenAI OpenAIConfig
}

// AnthropicConfig holds Claude provider settings.
type AnthropicConfig struct {
	APIKey       string
	BaseURL      string // optional; overrides https://api.anthropic.com/
	Model        string
	MaxTokens    int64
	SystemPrompt string
}

// OpenAIConfig holds OpenAI provider settings.
type OpenAIConfig struct {
	APIKey       string
	BaseURL      string // optional; overrides https://api.openai.com/v1/ (useful for proxies or compatible APIs)
	Model        string
	MaxTokens    int64
	SystemPrompt string
}

// Load reads configuration from the environment.
func Load() Config {
	cfg := Config{
		Port:               getenv("PORT", "8080"),
		AllowedOrigins:     splitCSV(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		StaticDir:          getenv("STATIC_DIR", ""),
		MemoryBackend:      getenv("MEMORY_BACKEND", ""),
		GreenNodeMemoryID:  getenv("GREENNODE_MEMORY_ID", ""),
		GreenNodeNamespace: getenv("GREENNODE_MEMORY_NAMESPACE", "default"),
		MockDir:            getenv("MOCK_DIR", "./mock"),
		DigestDBPath:       getenv("DIGEST_DB_PATH", "./digests.db"),
	}

	// Default the backend: use GreenNode only when both credentials and a
	// memory id are configured, otherwise fall back to the in-process store.
	if cfg.MemoryBackend == "" {
		if cfg.GreenNodeMemoryID != "" && os.Getenv("GREENNODE_CLIENT_ID") != "" {
			cfg.MemoryBackend = "greennode"
		} else {
			cfg.MemoryBackend = "memory"
		}
	}

	cfg.Anthropic = AnthropicConfig{
		APIKey:       os.Getenv("ANTHROPIC_API_KEY"),
		BaseURL:      os.Getenv("ANTHROPIC_BASE_URL"),
		Model:        getenv("ANTHROPIC_MODEL", "claude-opus-4-8"),
		MaxTokens:    4096,
		SystemPrompt: getenv("ANTHROPIC_SYSTEM_PROMPT", "You are a helpful AI assistant."),
	}
	cfg.OpenAI = OpenAIConfig{
		APIKey:       os.Getenv("OPENAI_API_KEY"),
		BaseURL:      os.Getenv("OPENAI_BASE_URL"),
		Model:        getenv("OPENAI_MODEL", "gpt-4o"),
		MaxTokens:    4096,
		SystemPrompt: getenv("OPENAI_SYSTEM_PROMPT", "You are a helpful AI assistant."),
	}
	// Auto-select provider from which API key is present when unset.
	cfg.LLMProvider = getenv("LLM_PROVIDER", "")
	if cfg.LLMProvider == "" {
		switch {
		case cfg.Anthropic.APIKey != "":
			cfg.LLMProvider = "anthropic"
		case cfg.OpenAI.APIKey != "":
			cfg.LLMProvider = "openai"
		default:
			cfg.LLMProvider = "echo"
		}
	}
	return cfg
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
