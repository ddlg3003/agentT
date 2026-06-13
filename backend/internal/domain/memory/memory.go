// Package memory defines the domain's memory abstractions. The Repository port
// describes what the agent needs from a memory backend in domain terms; the
// GreenNode AgentBase SDK is one implementation (see internal/infra/greennode),
// but the use case never sees the vendor. Swapping to a self-hosted store later
// means writing a new adapter, nothing more.
package memory

import (
	"context"

	"github.com/vngcloud/agentt/internal/domain/agent"
)

// Fact is a retrieved long-term memory record relevant to a query.
type Fact struct {
	ID      string
	Content string
	Score   float64
}

// Turn is a single conversational event persisted to memory.
type Turn struct {
	Role    agent.Role
	Content string
}

// Repository is the port for conversational + long-term memory.
type Repository interface {
	// Recall returns long-term facts relevant to query for the given user.
	Recall(ctx context.Context, userID, query string, limit int) ([]Fact, error)

	// History returns the recent conversation turns for a session, oldest-first.
	History(ctx context.Context, userID, sessionID string, limit int) ([]Turn, error)

	// AppendTurn persists a single conversation turn to a session.
	AppendTurn(ctx context.Context, userID, sessionID string, turn Turn) error
}
