package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vngcloud/agentt/internal/domain/agent"
	"github.com/vngcloud/agentt/internal/domain/memory"
)

// compactThreshold and compactKeepRecent configure the in-loop context
// compaction for the follow-up Q&A session.
const (
	compactThreshold  = 16
	compactKeepRecent = 4
)

// FollowupInput is a single Q&A request scoped to one digest.
type FollowupInput struct {
	Date     string
	UserID   string
	Question string
}

// FollowupOutput is the agent's answer to a follow-up question.
type FollowupOutput struct {
	Answer string `json:"answer"`
}

// FollowupTurn is a single visible Q&A turn (compact boundary markers are
// stripped — they are internal bookkeeping, not conversation content).
type FollowupTurn struct {
	Role    string `json:"role"` // "user" | "assistant"
	Content string `json:"content"`
}

// GetFollowupHistory returns the persisted Q&A turns for a digest session,
// oldest-first, with compact boundary markers stripped out.
func (s *DigestService) GetFollowupHistory(ctx context.Context, date, userID string) ([]FollowupTurn, error) {
	sessionID := "digest:" + date
	raw, err := s.mem.FullHistory(ctx, userID, sessionID, historyLimit)
	if err != nil {
		return nil, err
	}
	turns := make([]FollowupTurn, 0, len(raw))
	for _, t := range raw {
		switch t.Role {
		case "user", "assistant":
			turns = append(turns, FollowupTurn{Role: string(t.Role), Content: t.Content})
		// skip compact boundary markers
		}
	}
	return turns, nil
}

// AskFollowup runs the follow-up Q&A loop for a digest. The loop is grounded
// with the full digest JSON as truth, can re-query the read-only tools if it
// needs data not in the digest, and is additionally given the update_digest
// write tool so a PO can correct the digest in conversation. Q&A turns are
// persisted via the memory repository, scoped to this digest (sessionID
// "digest:<date>"), so the conversation survives across requests.
//
// Context compaction is handled by the loop (compactConfig). History() always
// starts from the last compact boundary; when the loop compacts, the summary
// is persisted via AppendTurn(RoleCompact) before the new Q&A turns so it
// becomes the new boundary on the next load.
func (s *DigestService) AskFollowup(ctx context.Context, in FollowupInput) (FollowupOutput, error) {
	log := s.log.With("loop", "followup", "date", in.Date, "userID", in.UserID)

	d, err := s.repo.Get(ctx, in.Date)
	if err != nil {
		log.ErrorContext(ctx, "load digest for followup", "error", err)
		return FollowupOutput{}, err
	}
	ground, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return FollowupOutput{}, fmt.Errorf("marshal digest ground truth: %w", err)
	}

	// Follow-up tool set = read-only tools + the per-request write tool, bound to
	// this digest and actor. Daily/monthly never get the write tool.
	tools := append([]agent.Tool{}, s.readTools...)
	tools = append(tools, s.newWriteTool(in.Date, in.UserID))

	sessionID := "digest:" + in.Date

	messages := []agent.Message{
		{Role: agent.RoleSystem, Content: followupPrompt + "\n" + string(ground)},
	}

	// History() returns turns from the last compact boundary onward. If the
	// first turn is a compact boundary, inject it as a synthetic exchange so
	// the model has prior context, then replay verbatim turns from history[1:].
	history, _ := s.mem.History(ctx, in.UserID, sessionID, historyLimit)
	replayFrom := 0
	if len(history) > 0 && history[0].Role == memory.RoleCompact {
		messages = append(messages,
			agent.Message{Role: agent.RoleUser, Content: "[Summary of earlier Q&A on this digest]\n" + history[0].Content},
			agent.Message{Role: agent.RoleAssistant, Content: "Understood, I have the context of our prior conversation."},
		)
		replayFrom = 1
	}
	for _, t := range history[replayFrom:] {
		messages = append(messages, agent.Message{Role: t.Role, Content: t.Content})
	}
	messages = append(messages, agent.Message{Role: agent.RoleUser, Content: in.Question})

	loop := newAgentLoop(s.llm, tools, s.maxDailyTurns, log)
	loop.compact = &compactConfig{threshold: compactThreshold, keepRecent: compactKeepRecent}

	res, err := loop.run(ctx, messages)

	// Persist the compact boundary even when the loop returns an error (e.g.
	// ErrMaxTurnsExceeded). Compaction may have fired before the turn limit was
	// hit; if we skip this, the next request reloads full pre-compact history,
	// triggers compaction again, and hits max turns again — permanently degraded.
	if res.CompactSummary != "" {
		if perr := s.mem.AppendTurn(ctx, in.UserID, sessionID, memory.Turn{
			Role:    memory.RoleCompact,
			Content: res.CompactSummary,
		}); perr != nil {
			log.ErrorContext(ctx, "persist compact boundary", "error", perr)
		} else {
			log.InfoContext(ctx, "compact boundary persisted", "summaryLen", len(res.CompactSummary))
		}
	}

	if err != nil {
		log.ErrorContext(ctx, "followup loop failed", "error", err)
		return FollowupOutput{}, err
	}
	answer := res.Final.Content

	if err := s.mem.AppendTurn(ctx, in.UserID, sessionID, memory.Turn{Role: agent.RoleUser, Content: in.Question}); err != nil {
		log.ErrorContext(ctx, "persist followup question", "error", err)
	}
	if err := s.mem.AppendTurn(ctx, in.UserID, sessionID, memory.Turn{Role: agent.RoleAssistant, Content: answer}); err != nil {
		log.ErrorContext(ctx, "persist followup answer", "error", err)
	}

	log.InfoContext(ctx, "followup answered", "turns", res.Turns, "compacted", res.CompactSummary != "")
	return FollowupOutput{Answer: answer}, nil
}
