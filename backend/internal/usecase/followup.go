package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vngcloud/agentt/internal/domain/agent"
	"github.com/vngcloud/agentt/internal/domain/memory"
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

// AskFollowup runs the follow-up Q&A loop for a digest. The loop is grounded
// with the full digest JSON as truth, can re-query the read-only tools if it
// needs data not in the digest, and is additionally given the update_digest
// write tool so a PO can correct the digest in conversation. Q&A turns are
// persisted via the memory repository, scoped to this digest (sessionID
// "digest:<date>"), so the conversation survives across requests.
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
	// Replay prior Q&A on this digest so the conversation has continuity.
	if history, err := s.mem.History(ctx, in.UserID, sessionID, historyLimit); err != nil {
		log.ErrorContext(ctx, "followup history failed", "error", err)
	} else {
		for _, t := range history {
			messages = append(messages, agent.Message{Role: t.Role, Content: t.Content})
		}
	}
	messages = append(messages, agent.Message{Role: agent.RoleUser, Content: in.Question})

	loop := newAgentLoop(s.llm, tools, s.maxDailyTurns, log)
	res, err := loop.run(ctx, messages)
	if err != nil {
		log.ErrorContext(ctx, "followup loop failed", "error", err)
		return FollowupOutput{}, err
	}
	answer := res.Final.Content

	// Persist the turn (best-effort: memory failure shouldn't drop the answer).
	if err := s.mem.AppendTurn(ctx, in.UserID, sessionID, memory.Turn{Role: agent.RoleUser, Content: in.Question}); err != nil {
		log.ErrorContext(ctx, "persist followup question", "error", err)
	}
	if err := s.mem.AppendTurn(ctx, in.UserID, sessionID, memory.Turn{Role: agent.RoleAssistant, Content: answer}); err != nil {
		log.ErrorContext(ctx, "persist followup answer", "error", err)
	}

	log.InfoContext(ctx, "followup answered", "turns", res.Turns)
	return FollowupOutput{Answer: answer}, nil
}
