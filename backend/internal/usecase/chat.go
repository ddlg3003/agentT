// Package usecase contains application business logic. It orchestrates domain
// ports (LLMClient, memory.Repository) and depends on no framework, transport
// or vendor — those are injected by the composition root (cmd/server).
package usecase

import (
	"context"
	"log/slog"
	"strings"

	"github.com/vngcloud/agentt/internal/domain/agent"
	"github.com/vngcloud/agentt/internal/domain/memory"
)

const (
	historyLimit = 20
	recallLimit  = 5
)

// ChatService implements the core "talk to the agent" use case: recall relevant
// memory, build the prompt from history, call the LLM, and persist the turn.
type ChatService struct {
	llm    agent.LLMClient
	memory memory.Repository
	log    *slog.Logger
}

// NewChatService wires a ChatService from its ports.
func NewChatService(llm agent.LLMClient, mem memory.Repository) *ChatService {
	return &ChatService{llm: llm, memory: mem, log: slog.Default()}
}

// ChatInput is the request to the chat use case.
type ChatInput struct {
	UserID    string
	SessionID string
	Message   string
}

// ChatOutput is the agent's reply.
type ChatOutput struct {
	Reply string `json:"reply"`
}

// Chat runs one conversational turn.
func (s *ChatService) Chat(ctx context.Context, in ChatInput) (ChatOutput, error) {
	messages := make([]agent.Message, 0, historyLimit+2)

	log := s.log.With("userID", in.UserID, "sessionID", in.SessionID)

	// Ground the agent with long-term facts when memory is available.
	if facts, err := s.memory.Recall(ctx, in.UserID, in.Message, recallLimit); err != nil {
		log.ErrorContext(ctx, "memory recall failed", "error", err)
	} else if len(facts) > 0 {
		messages = append(messages, agent.Message{
			Role:    agent.RoleSystem,
			Content: "Relevant memory about the user:\n" + formatFacts(facts),
		})
	}

	// Replay recent conversation history.
	if history, err := s.memory.History(ctx, in.UserID, in.SessionID, historyLimit); err != nil {
		log.ErrorContext(ctx, "memory history failed", "error", err)
	} else {
		for _, t := range history {
			messages = append(messages, agent.Message{Role: t.Role, Content: t.Content})
		}
	}

	userMsg := agent.Message{Role: agent.RoleUser, Content: in.Message}
	messages = append(messages, userMsg)

	reply, err := s.llm.Complete(ctx, messages)
	if err != nil {
		log.ErrorContext(ctx, "llm complete failed", "error", err)
		return ChatOutput{}, err
	}

	// Persist the turn (best-effort: memory failures shouldn't drop the reply).
	if err := s.memory.AppendTurn(ctx, in.UserID, in.SessionID, memory.Turn{Role: agent.RoleUser, Content: in.Message}); err != nil {
		log.ErrorContext(ctx, "failed to persist user turn", "error", err)
	}
	if err := s.memory.AppendTurn(ctx, in.UserID, in.SessionID, memory.Turn{Role: agent.RoleAssistant, Content: reply.Content}); err != nil {
		log.ErrorContext(ctx, "failed to persist assistant turn", "error", err)
	}

	return ChatOutput{Reply: reply.Content}, nil
}

func formatFacts(facts []memory.Fact) string {
	var b strings.Builder
	for _, f := range facts {
		b.WriteString("- ")
		b.WriteString(f.Content)
		b.WriteByte('\n')
	}
	return b.String()
}
