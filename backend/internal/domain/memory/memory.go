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

// RoleCompact is a sentinel role used as a compact boundary marker. A Turn
// with this role is never a real conversation turn — it is a summary of all
// prior turns, written by Compact() and consumed by the follow-up loop to
// inject prior context without replaying every individual turn.
const RoleCompact agent.Role = "compact"

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
	// If the session has been compacted, the first turn will have Role==RoleCompact
	// and its Content is the summary; subsequent turns are the keepRecent verbatim
	// turns preserved at compaction time.
	History(ctx context.Context, userID, sessionID string, limit int) ([]Turn, error)

	// FullHistory returns all conversation turns for a session, oldest-first,
	// with compact boundary markers stripped. Unlike History(), it does not
	// truncate at a compact boundary — intended for display, not agent context.
	FullHistory(ctx context.Context, userID, sessionID string, limit int) ([]Turn, error)

	// AppendTurn persists a single conversation turn to a session. To record a
	// compact boundary, append a Turn with Role==RoleCompact; History() will
	// automatically scan for the last such boundary and discard turns before it.
	AppendTurn(ctx context.Context, userID, sessionID string, turn Turn) error
}
