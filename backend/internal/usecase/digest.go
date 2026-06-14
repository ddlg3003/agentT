package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vngcloud/agentt/internal/domain/agent"
	"github.com/vngcloud/agentt/internal/domain/digest"
	"github.com/vngcloud/agentt/internal/domain/memory"
)

// Default hard turn limits per loop (agentic-harness: hard stops, not soft
// warnings). Daily/monthly differ because the monthly rollup reads more context.
const (
	defaultMaxDailyTurns   = 10
	defaultMaxMonthlyTurns = 15
)

// DigestService runs the three agent loops of the product — daily digest
// generation, follow-up Q&A (with PO corrections), and the monthly rollup — over
// a shared think→act→observe engine (agentLoop). The only things that differ
// between loops are the system prompt, the seed message, and the tool set.
type DigestService struct {
	llm       agent.ToolCaller
	repo      digest.Repository
	mem       memory.Repository
	readTools []agent.Tool // read-only tools shared by all loops
	// newWriteTool builds the per-request update_digest tool, scoped to one
	// digest (date) and actor (userID). It is supplied by the composition root so
	// usecase depends only on the agent.Tool port, never on infra. Only the
	// follow-up loop is given this tool — daily/monthly stay read-only by code.
	newWriteTool func(date, userID string) agent.Tool
	now          func() time.Time
	log          *slog.Logger

	maxDailyTurns   int
	maxMonthlyTurns int
}

// NewDigestService wires the service from its ports. readTools are the read-only
// data tools (query_bi/query_jira/query_gitlab/get_knowledge); newWriteTool
// builds the update_digest tool used only by the follow-up loop.
func NewDigestService(
	llm agent.ToolCaller,
	repo digest.Repository,
	mem memory.Repository,
	readTools []agent.Tool,
	newWriteTool func(date, userID string) agent.Tool,
) *DigestService {
	return &DigestService{
		llm:             llm,
		repo:            repo,
		mem:             mem,
		readTools:       readTools,
		newWriteTool:    newWriteTool,
		now:             time.Now,
		log:             slog.Default(),
		maxDailyTurns:   defaultMaxDailyTurns,
		maxMonthlyTurns: defaultMaxMonthlyTurns,
	}
}

// agentDigest is the JSON shape the daily loop's final message must produce. It
// is a subset of digest.DailyDigest — the loop fills in Date/GeneratedAt/Sources.
type agentDigest struct {
	Metrics   []digest.Metric `json:"metrics"`
	Events    []digest.Event  `json:"events"`
	Reasoning string          `json:"reasoning"`
}

// GenerateDaily runs the daily loop for a date and persists the resulting digest.
// The loop uses only read-only tools — there is no write tool in scope, so the
// run cannot mutate anything (constrain by code, not instruction).
func (s *DigestService) GenerateDaily(ctx context.Context, date string) (digest.DailyDigest, error) {
	log := s.log.With("loop", "daily", "date", date)

	loop := newAgentLoop(s.llm, s.readTools, s.maxDailyTurns, log)
	messages := []agent.Message{
		{Role: agent.RoleSystem, Content: dailyPrompt},
		{Role: agent.RoleUser, Content: "Generate the daily digest for DATE: " + date},
	}

	res, err := loop.run(ctx, messages)
	if err != nil {
		log.ErrorContext(ctx, "daily loop failed", "error", err)
		return digest.DailyDigest{}, err
	}

	parsed, err := parseAgentDigest(res.Final.Content)
	if err != nil {
		log.ErrorContext(ctx, "parse daily digest output", "error", err)
		return digest.DailyDigest{}, fmt.Errorf("parse digest: %w", err)
	}

	d := digest.DailyDigest{
		Date:        date,
		GeneratedAt: s.now().UTC(),
		Metrics:     parsed.Metrics,
		Events:      parsed.Events,
		Reasoning:   parsed.Reasoning,
		Sources:     res.Sources,
	}
	if err := s.repo.Save(ctx, d); err != nil {
		log.ErrorContext(ctx, "save digest", "error", err)
		return digest.DailyDigest{}, fmt.Errorf("save digest: %w", err)
	}
	log.InfoContext(ctx, "daily digest generated", "turns", res.Turns, "metrics", len(d.Metrics), "events", len(d.Events))
	return d, nil
}

// Get loads a stored digest by date.
func (s *DigestService) Get(ctx context.Context, date string) (digest.DailyDigest, error) {
	return s.repo.Get(ctx, date)
}

// ListDates returns the dates that have a stored digest, newest first.
func (s *DigestService) ListDates(ctx context.Context) ([]string, error) {
	return s.repo.ListDates(ctx)
}

// FlagDigest marks a digest as incorrect with a PO note. This is a direct PO
// action (no LLM loop): it loads, flags, and saves, recording the flag as a
// correction so the action is part of the audit trail.
func (s *DigestService) FlagDigest(ctx context.Context, date, userID, note string) (digest.DailyDigest, error) {
	d, err := s.repo.Get(ctx, date)
	if err != nil {
		return digest.DailyDigest{}, err
	}
	d.Flagged = true
	d.FlagNote = note
	d.Corrections = append(d.Corrections, digest.Correction{
		Field: "flag", OldValue: "false", NewValue: "true", Note: note, By: userID, At: s.now().UTC(),
	})
	if err := s.repo.Save(ctx, d); err != nil {
		return digest.DailyDigest{}, err
	}
	return d, nil
}

// parseAgentDigest extracts the JSON object from the model's final message,
// tolerating a ```json fence or surrounding prose.
func parseAgentDigest(content string) (agentDigest, error) {
	raw := extractJSONObject(content)
	if raw == "" {
		return agentDigest{}, fmt.Errorf("no JSON object found in model output")
	}
	var d agentDigest
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return agentDigest{}, err
	}
	return d, nil
}

// extractJSONObject returns the substring from the first '{' to the last '}',
// which covers both bare JSON and fenced ```json blocks.
func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}
