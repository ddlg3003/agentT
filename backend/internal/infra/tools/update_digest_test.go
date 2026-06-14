package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/vngcloud/agentt/internal/domain/digest"
)

// memRepo is an in-memory digest.Repository for tests.
type memRepo struct {
	m map[string]digest.DailyDigest
}

func newMemRepo() *memRepo { return &memRepo{m: map[string]digest.DailyDigest{}} }

func (r *memRepo) Save(_ context.Context, d digest.DailyDigest) error {
	r.m[d.Date] = d
	return nil
}
func (r *memRepo) Get(_ context.Context, date string) (digest.DailyDigest, error) {
	d, ok := r.m[date]
	if !ok {
		return digest.DailyDigest{}, digest.ErrNotFound
	}
	return d, nil
}
func (r *memRepo) ListDates(context.Context) ([]string, error)                     { return nil, nil }
func (r *memRepo) ListMonth(context.Context, string) ([]digest.DailyDigest, error) { return nil, nil }

func fixedClock() func() time.Time {
	t := time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

func TestUpdateDigestMetricCorrection(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	_ = repo.Save(context.Background(), digest.DailyDigest{
		Date: "2026-03-15",
		Metrics: []digest.Metric{
			{Partner: "SHB", Step: "s20s120", Name: "E2E Rate", Unit: "%", Value: 0.037},
		},
	})

	tool := NewUpdateDigest(repo, "2026-03-15", "po@vng", fixedClock())
	out, err := tool.Run(context.Background(), json.RawMessage(
		`{"field":"metric","partner":"SHB","step":"s20s120","new_value":"0.045","note":"BI revised the number"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("output not valid JSON: %s", out)
	}

	got, _ := repo.Get(context.Background(), "2026-03-15")
	if got.Metrics[0].Value != 0.045 {
		t.Errorf("metric value = %v, want 0.045", got.Metrics[0].Value)
	}
	if !got.Flagged {
		t.Error("digest should be flagged after correction")
	}
	if len(got.Corrections) != 1 {
		t.Fatalf("corrections = %d, want 1", len(got.Corrections))
	}
	c := got.Corrections[0]
	if c.OldValue != "0.037" || c.NewValue != "0.045" {
		t.Errorf("correction old/new = %s/%s, want 0.037/0.045", c.OldValue, c.NewValue)
	}
	if c.By != "po@vng" {
		t.Errorf("correction by = %s, want po@vng", c.By)
	}
}

func TestUpdateDigestReasoning(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	_ = repo.Save(context.Background(), digest.DailyDigest{Date: "2026-03-15", Reasoning: "original"})

	tool := NewUpdateDigest(repo, "2026-03-15", "po@vng", fixedClock())
	if _, err := tool.Run(context.Background(), json.RawMessage(
		`{"field":"reasoning","new_value":"corrected narrative","note":"PO rewrite"}`)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, _ := repo.Get(context.Background(), "2026-03-15")
	if got.Reasoning != "corrected narrative" {
		t.Errorf("reasoning = %q, want corrected", got.Reasoning)
	}
	if got.Corrections[0].OldValue != "original" {
		t.Errorf("old reasoning not preserved: %q", got.Corrections[0].OldValue)
	}
}
