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

// Message is a single turn in a conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// LLMClient is the port for a large language model. Infra provides the concrete
// implementation (a stub today, a real provider later) — the use case only
// knows this interface.
type LLMClient interface {
	// Complete produces the assistant's next message given the conversation so
	// far. messages is ordered oldest-first and may include a system message.
	Complete(ctx context.Context, messages []Message) (Message, error)
}
