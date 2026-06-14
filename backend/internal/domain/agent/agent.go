// Package agent holds the core domain entities and ports for the AI agent.
// It depends on nothing outside the standard library — vendors and frameworks
// are kept at the edges (infra/adapter), so the business rules stay portable.
package agent

import "context"

// Role identifies who produced a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Message is a single turn in a conversation. For plain chat only Role and
// Content are set. In an agentic (tool-using) loop an assistant message may also
// carry ToolCalls, and a follow-up user message carries the matching ToolResults
// — this is how the think → act → observe loop threads tool I/O back to the model.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`

	// ToolCalls is set on assistant messages that ask to invoke one or more
	// tools. Empty for plain text replies — its emptiness is the loop's
	// content-derived stop signal (never trust the provider's stop_reason).
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolResults is set on user messages that return tool outputs to the model,
	// one per preceding ToolCall (matched by CallID).
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}

// LLMClient is the port for a large language model. Infra provides the concrete
// implementation (a stub today, a real provider later) — the use case only
// knows this interface.
type LLMClient interface {
	// Complete produces the assistant's next message given the conversation so
	// far. messages is ordered oldest-first and may include a system message.
	Complete(ctx context.Context, messages []Message) (Message, error)
}

// ToolCaller is the port for an LLM that supports tool use. The agent loop
// depends on this richer capability; plain chat depends only on LLMClient.
// Providers that cannot call tools (e.g. the echo stub) may still satisfy this
// by always returning a text-only message, which makes the loop stop on turn 0.
type ToolCaller interface {
	LLMClient
	// CompleteWithTools is like Complete but advertises the given tools to the
	// model. The returned message either carries ToolCalls (the model wants to
	// act) or is text-only (the model is done).
	CompleteWithTools(ctx context.Context, messages []Message, tools []ToolDef) (Message, error)
}
