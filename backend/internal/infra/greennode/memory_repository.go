// Package greennode adapts the GreenNode AgentBase SDK to the domain ports.
// This is the only place in the app that knows about the vendor SDK; everything
// upstream depends on internal/domain interfaces. Replacing AgentBase means
// replacing this file.
package greennode

import (
	"context"

	"github.com/vngcloud/agentt/internal/domain/agent"
	"github.com/vngcloud/agentt/internal/domain/memory"
	gn "github.com/vngcloud/agentt/pkg/greennode/memory"
)

// MemoryRepository implements memory.Repository on top of the GreenNode Memory
// API. A single memory resource (MemoryID) backs the whole app; the domain
// user id maps to the AgentBase "actor".
type MemoryRepository struct {
	client    *gn.Client
	memoryID  string
	namespace string
}

// Config configures the repository adapter.
type Config struct {
	// MemoryID is the AgentBase memory resource id this app reads/writes.
	MemoryID string
	// Namespace scopes long-term record search/insert.
	Namespace string
	// ClientOptions are forwarded to the underlying SDK client.
	ClientOptions gn.Options
}

// NewMemoryRepository builds the adapter.
func NewMemoryRepository(cfg Config) *MemoryRepository {
	ns := cfg.Namespace
	if ns == "" {
		ns = "default"
	}
	return &MemoryRepository{
		client:    gn.New(cfg.ClientOptions),
		memoryID:  cfg.MemoryID,
		namespace: ns,
	}
}

var _ memory.Repository = (*MemoryRepository)(nil)

// Recall searches long-term memory records for facts relevant to query.
func (r *MemoryRepository) Recall(ctx context.Context, userID, query string, limit int) ([]memory.Fact, error) {
	resp, err := r.client.SearchRecords(ctx, r.memoryID, r.namespace, gn.SearchRequest{
		Query: query,
		Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	facts := make([]memory.Fact, 0, len(resp.ListData))
	for _, rec := range resp.ListData {
		f := memory.Fact{ID: rec.ID, Content: rec.Memory}
		if rec.Score != nil {
			f.Score = *rec.Score
		}
		facts = append(facts, f)
	}
	return facts, nil
}

// History returns recent session events as conversation turns (oldest-first).
func (r *MemoryRepository) History(ctx context.Context, userID, sessionID string, limit int) ([]memory.Turn, error) {
	resp, err := r.client.ListEvents(ctx, r.memoryID, userID, sessionID, 0, limit)
	if err != nil {
		return nil, err
	}
	turns := make([]memory.Turn, 0, len(resp.ListData))
	for _, ev := range resp.ListData {
		if ev.Payload == nil {
			continue
		}
		turns = append(turns, memory.Turn{
			Role:    agent.Role(ev.Payload.Role),
			Content: ev.Payload.Message,
		})
	}
	return turns, nil
}

// AppendTurn stores a conversation turn as a session event.
func (r *MemoryRepository) AppendTurn(ctx context.Context, userID, sessionID string, turn memory.Turn) error {
	return r.client.CreateEvent(ctx, r.memoryID, userID, sessionID, gn.CreateEventRequest{
		Payload: &gn.EventPayload{
			Type:    "conversational",
			Role:    string(turn.Role),
			Message: turn.Content,
		},
	})
}
