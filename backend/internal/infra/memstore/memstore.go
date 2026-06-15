// Package memstore is an in-process memory.Repository for local development and
// demos when GreenNode credentials are not configured. It keeps history in RAM
// and does no long-term recall.
package memstore

import (
	"context"
	"sync"

	"github.com/vngcloud/agentt/internal/domain/memory"
)

// Store is a thread-safe in-memory conversation store keyed by user+session.
type Store struct {
	mu      sync.Mutex
	history map[string][]memory.Turn
}

// New returns an empty Store.
func New() *Store {
	return &Store{history: map[string][]memory.Turn{}}
}

var _ memory.Repository = (*Store)(nil)

func key(userID, sessionID string) string { return userID + "\x00" + sessionID }

// Recall always returns no facts (no long-term memory in the local store).
func (s *Store) Recall(_ context.Context, _, _ string, _ int) ([]memory.Fact, error) {
	return nil, nil
}

// History returns up to limit most-recent turns starting from the last compact
// boundary (if any), oldest-first. Turns before the boundary are logically
// discarded — a compact boundary (Role==RoleCompact) is the summary of
// everything that came before it.
func (s *Store) History(_ context.Context, userID, sessionID string, limit int) ([]memory.Turn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	turns := s.history[key(userID, sessionID)]

	// Find the last compact boundary and start from there.
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Role == memory.RoleCompact {
			turns = turns[i:]
			break
		}
	}

	if limit > 0 && len(turns) > limit {
		turns = turns[len(turns)-limit:]
	}
	out := make([]memory.Turn, len(turns))
	copy(out, turns)
	return out, nil
}

// FullHistory returns all visible turns (compact markers stripped), oldest-first.
func (s *Store) FullHistory(_ context.Context, userID, sessionID string, limit int) ([]memory.Turn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := s.history[key(userID, sessionID)]
	out := make([]memory.Turn, 0, len(all))
	for _, t := range all {
		if t.Role != memory.RoleCompact {
			out = append(out, t)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

// AppendTurn appends a turn to the session history. To record a compact
// boundary, append a Turn with Role==RoleCompact; History() will start from
// the last such boundary, discarding older turns.
func (s *Store) AppendTurn(_ context.Context, userID, sessionID string, turn memory.Turn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(userID, sessionID)
	s.history[k] = append(s.history[k], turn)
	return nil
}
