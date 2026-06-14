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

var (
	_ agent.LLMClient  = (*Echo)(nil)
	_ agent.ToolCaller = (*Echo)(nil)
)

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

// CompleteWithTools satisfies agent.ToolCaller so the agent loop is runnable
// without an API key. The echo stub never calls tools — it returns a text-only
// message, which makes the loop stop immediately on turn 0. It cannot produce a
// real digest (that needs a tool-using model), but it keeps the wiring testable.
func (e *Echo) CompleteWithTools(ctx context.Context, messages []agent.Message, _ []agent.ToolDef) (agent.Message, error) {
	return e.Complete(ctx, messages)
}
