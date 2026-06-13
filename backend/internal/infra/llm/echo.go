// Package llm provides agent.LLMClient implementations. The echo client is a
// dependency-free stub that keeps the skeleton runnable end-to-end; swap in a
// real provider (e.g. Anthropic Claude) by adding a sibling implementation and
// wiring it in cmd/server.
package llm

import (
	"context"

	"github.com/vngcloud/agentt/internal/domain/agent"
)

// Echo is a stub LLM that echoes the last user message back. It exists so the
// full request path (HTTP -> usecase -> memory) works without an API key.
type Echo struct{}

// NewEcho returns an Echo client.
func NewEcho() *Echo { return &Echo{} }

var _ agent.LLMClient = (*Echo)(nil)

// Complete returns a canned reply derived from the last user message.
func (e *Echo) Complete(_ context.Context, messages []agent.Message) (agent.Message, error) {
	last := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == agent.RoleUser {
			last = messages[i].Content
			break
		}
	}
	return agent.Message{
		Role:    agent.RoleAssistant,
		Content: "[echo] " + last,
	}, nil
}
