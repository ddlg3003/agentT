package agent

import (
	"context"
	"encoding/json"
)

// ToolDef describes a tool to the LLM: its name, a natural-language description
// of when to use it, and a JSON Schema for its input. This is the contract the
// model reasons over when deciding what to call.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolCall is the model's request to invoke a tool. ID is the provider-assigned
// identifier that must be echoed back on the matching ToolResult so the model
// can correlate the two across a turn.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult is the outcome of a single tool invocation, fed back to the model.
// On failure IsError is true and Content carries a human-readable reason — the
// loop never drops a failed call silently, it reports the failure to the model
// so it can adapt (see the "[DATA UNAVAILABLE]" convention in the use case).
type ToolResult struct {
	CallID  string `json:"call_id"`
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// Tool is the port for an executable capability the agent can invoke. Concrete
// tools live in infra (they read mock files today, real systems later); the loop
// only sees this interface and the ToolDef it advertises.
//
// Tools are read-only by default. A tool that mutates state (e.g. correcting a
// stored digest) is a deliberate, separately-wired exception — never given to a
// loop that is meant to stay read-only.
type Tool interface {
	// Definition returns the schema advertised to the model.
	Definition() ToolDef
	// Run executes the tool with the model-supplied input and returns its output
	// as a string (typically JSON or markdown) to feed back into context.
	Run(ctx context.Context, input json.RawMessage) (string, error)
}
