package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vngcloud/agentt/internal/domain/agent"
)

// MonthlyReport is the synthesized month-over-month PO report.
type MonthlyReport struct {
	Month      string `json:"month"`    // "2026-03"
	Markdown   string `json:"markdown"` // the report body
	NumDigests int    `json:"num_digests"`
}

// GenerateMonthly runs the monthly rollup loop: it loads every stored daily
// digest for the month, injects them as ground truth, and asks the agent to
// synthesize the per-partner report following the PO template. The loop is
// read-only (no write tool) — it only reads digests and may re-query BI to fill
// month-over-month gaps.
func (s *DigestService) GenerateMonthly(ctx context.Context, ym string) (MonthlyReport, error) {
	log := s.log.With("loop", "monthly", "month", ym)

	digests, err := s.repo.ListMonth(ctx, ym)
	if err != nil {
		log.ErrorContext(ctx, "list month digests", "error", err)
		return MonthlyReport{}, err
	}
	if len(digests) == 0 {
		return MonthlyReport{}, fmt.Errorf("no digests stored for month %s", ym)
	}

	ground, err := json.MarshalIndent(digests, "", "  ")
	if err != nil {
		return MonthlyReport{}, fmt.Errorf("marshal digests: %w", err)
	}

	loop := newAgentLoop(s.llm, s.readTools, s.maxMonthlyTurns, log)
	messages := []agent.Message{
		{Role: agent.RoleSystem, Content: monthlyPrompt},
		{Role: agent.RoleUser, Content: fmt.Sprintf(
			"Synthesize the monthly report for MONTH: %s.\nHere are the %d daily digests for the month as JSON:\n%s",
			ym, len(digests), string(ground))},
	}

	res, err := loop.run(ctx, messages)
	if err != nil {
		log.ErrorContext(ctx, "monthly loop failed", "error", err)
		return MonthlyReport{}, err
	}

	log.InfoContext(ctx, "monthly report generated", "turns", res.Turns, "digests", len(digests))
	return MonthlyReport{Month: ym, Markdown: res.Final.Content, NumDigests: len(digests)}, nil
}
