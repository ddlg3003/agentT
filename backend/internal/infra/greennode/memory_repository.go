// Package greennode adapts the GreenNode AgentBase SDK to the domain ports.
// This is the only place in the app that knows about the vendor SDK; everything
// upstream depends on internal/domain interfaces. Replacing AgentBase means
// replacing this file.
package greennode

import (
	"context"
	"strings"

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

// compactScanBuffer is the extra events fetched beyond limit to reliably find
// a compact boundary. At most compactThreshold (16) turns accumulate between
// compactions, so 32 is a safe headroom.
const compactScanBuffer = 32

// gnCompactPrefix is the sentinel prepended to the content of a compact
// boundary event stored in GreenNode. GreenNode only allows role values
// [user, assistant, system], so we encode compact boundaries as user messages
// and detect them on read by this prefix. The prefix uses ASCII control
// characters (SOH + ETX) that will never appear in LLM-generated text.
const gnCompactPrefix = "\x01compact\x03"

// History returns recent session events as conversation turns (oldest-first),
// starting from the last compact boundary (Role=="compact") if one exists.
// GreenNode is append-only: the compact boundary is a normal event with a
// sentinel role; History scans the fetch window and discards everything before
// the last such boundary.
func (r *MemoryRepository) History(ctx context.Context, userID, sessionID string, limit int) ([]memory.Turn, error) {
	resp, err := r.client.ListEvents(ctx, r.memoryID, userID, sessionID, 0, limit+compactScanBuffer)
	if err != nil {
		return nil, err
	}
	turns := make([]memory.Turn, 0, len(resp.ListData))
	for _, ev := range resp.ListData {
		if ev.Payload == nil {
			continue
		}
		role := agent.Role(ev.Payload.Role)
		content := ev.Payload.Message
		// Decode compact boundaries: stored as user messages with gnCompactPrefix.
		if strings.HasPrefix(content, gnCompactPrefix) {
			role = memory.RoleCompact
			content = content[len(gnCompactPrefix):]
		}
		turns = append(turns, memory.Turn{Role: role, Content: content})
	}

	// ListEvents returns newest-first; reverse to oldest-first before processing.
	for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
		turns[i], turns[j] = turns[j], turns[i]
	}

	// Find the last compact boundary and start from there.
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Role == memory.RoleCompact {
			turns = turns[i:]
			break
		}
	}

	if limit > 0 && len(turns) > limit {
		// Preserve the compact boundary at turns[0] (if present) and trim the
		// tail to at most (limit-1) entries after it. Without this guard, a
		// straight tail-trim silently drops the boundary and followup.go
		// replays all raw turns without the summary injection.
		if turns[0].Role == memory.RoleCompact {
			tail := turns[1:]
			if len(tail) > limit-1 {
				tail = tail[len(tail)-(limit-1):]
			}
			out := make([]memory.Turn, 0, 1+len(tail))
			out = append(out, turns[0])
			turns = append(out, tail...)
		} else {
			turns = turns[len(turns)-limit:]
		}
	}
	return turns, nil
}

// FullHistory returns all visible turns (compact markers stripped), oldest-first.
// Fetches a generous window so the full conversation history is available for display.
func (r *MemoryRepository) FullHistory(ctx context.Context, userID, sessionID string, limit int) ([]memory.Turn, error) {
	fetchLimit := 50 // GreenNode max page size is 100; keep headroom for future pagination
	resp, err := r.client.ListEvents(ctx, r.memoryID, userID, sessionID, 0, fetchLimit)
	if err != nil {
		return nil, err
	}
	turns := make([]memory.Turn, 0, len(resp.ListData))
	for _, ev := range resp.ListData {
		if ev.Payload == nil {
			continue
		}
		content := ev.Payload.Message
		// Skip compact boundary markers.
		if strings.HasPrefix(content, gnCompactPrefix) {
			continue
		}
		turns = append(turns, memory.Turn{
			Role:    agent.Role(ev.Payload.Role),
			Content: content,
		})
	}
	// ListEvents returns newest-first; reverse to oldest-first.
	for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
		turns[i], turns[j] = turns[j], turns[i]
	}
	if limit > 0 && len(turns) > limit {
		turns = turns[len(turns)-limit:]
	}
	return turns, nil
}

// AppendTurn stores a conversation turn as a session event. Compact boundaries
// (Role==RoleCompact) are encoded as role="user" messages with gnCompactPrefix
// prepended to the content, because GreenNode only accepts role values
// [user, assistant, system]. History() decodes them back on read.
func (r *MemoryRepository) AppendTurn(ctx context.Context, userID, sessionID string, turn memory.Turn) error {
	role := string(turn.Role)
	content := turn.Content
	if turn.Role == memory.RoleCompact {
		role = "user"
		content = gnCompactPrefix + content
	}
	return r.client.CreateEvent(ctx, r.memoryID, userID, sessionID, gn.CreateEventRequest{
		Payload: &gn.EventPayload{
			Type:    "conversational",
			Role:    role,
			Message: content,
		},
	})
}
