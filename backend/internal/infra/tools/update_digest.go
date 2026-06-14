package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/vngcloud/agentt/internal/domain/agent"
	"github.com/vngcloud/agentt/internal/domain/digest"
)

// UpdateDigest is the only write tool in the system, and it is wired in only for
// the follow-up Q&A loop — never the daily/monthly loops, which stay read-only.
// It mutates the agent's own digest store (never an external source), and every
// change is recorded as a digest.Correction that preserves the prior value, so
// PO edits remain fully traceable.
//
// The tool is scoped to one digest (date) and one actor (userID), bound at
// construction time, so the model can only ever correct the digest under
// discussion and the audit trail always attributes the change correctly.
type UpdateDigest struct {
	repo   digest.Repository
	date   string
	userID string
	now    func() time.Time
}

// NewUpdateDigest builds the write tool bound to a specific digest and actor.
func NewUpdateDigest(repo digest.Repository, date, userID string, now func() time.Time) *UpdateDigest {
	if now == nil {
		now = time.Now
	}
	return &UpdateDigest{repo: repo, date: date, userID: userID, now: now}
}

var _ agent.Tool = (*UpdateDigest)(nil)

type updateInput struct {
	Field    string `json:"field"`     // "reasoning" | "metric" | "flag"
	Partner  string `json:"partner"`   // required when field=metric
	Step     string `json:"step"`      // required when field=metric
	NewValue string `json:"new_value"` // new text, or numeric string for a metric
	Note     string `json:"note"`      // the PO's stated reason for the change
}

// Definition advertises update_digest to the model.
func (u *UpdateDigest) Definition() agent.ToolDef {
	return agent.ToolDef{
		Name: "update_digest",
		Description: "Apply a PO-requested correction to THIS digest and persist it. " +
			"Use only when the PO explicitly asks to correct something. " +
			"field=reasoning replaces the narrative; field=metric overwrites one metric value " +
			"(provide partner, step, and a numeric new_value as a fraction for rates, e.g. 0.045); " +
			"field=flag marks the digest as incorrect without changing values. " +
			"Always include a note with the PO's reason. The original value is preserved for audit.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "field": {"type": "string", "enum": ["reasoning", "metric", "flag"]},
    "partner": {"type": "string", "description": "partner code when field=metric, e.g. SHB"},
    "step": {"type": "string", "description": "funnel step code when field=metric, e.g. s20s120"},
    "new_value": {"type": "string", "description": "new reasoning text, or numeric string when field=metric"},
    "note": {"type": "string", "description": "the PO's reason for the correction"}
  },
  "required": ["field", "note"]
}`),
	}
}

// Run loads the bound digest, applies the correction, records the audit entry,
// flags the digest, and saves it.
func (u *UpdateDigest) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var in updateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	d, err := u.repo.Get(ctx, u.date)
	if err != nil {
		return "", fmt.Errorf("load digest %s: %w", u.date, err)
	}

	corr := digest.Correction{Note: in.Note, By: u.userID, At: u.now()}
	switch in.Field {
	case "reasoning":
		corr.Field = "reasoning"
		corr.OldValue = d.Reasoning
		corr.NewValue = in.NewValue
		d.Reasoning = in.NewValue

	case "metric":
		idx := findMetric(d.Metrics, in.Partner, in.Step)
		if idx < 0 {
			return "", fmt.Errorf("no metric for partner=%s step=%s in digest %s", in.Partner, in.Step, u.date)
		}
		newVal, err := strconv.ParseFloat(in.NewValue, 64)
		if err != nil {
			return "", fmt.Errorf("metric new_value must be numeric, got %q: %w", in.NewValue, err)
		}
		corr.Field = "metric:" + in.Partner + "/" + in.Step
		corr.OldValue = strconv.FormatFloat(d.Metrics[idx].Value, 'f', -1, 64)
		corr.NewValue = strconv.FormatFloat(newVal, 'f', -1, 64)
		d.Metrics[idx].Value = newVal

	case "flag":
		corr.Field = "flag"
		corr.OldValue = strconv.FormatBool(d.Flagged)
		corr.NewValue = "true"

	default:
		return "", fmt.Errorf("unknown field %q (want reasoning|metric|flag)", in.Field)
	}

	d.Flagged = true
	d.FlagNote = in.Note
	d.Corrections = append(d.Corrections, corr)

	if err := u.repo.Save(ctx, d); err != nil {
		return "", fmt.Errorf("save corrected digest: %w", err)
	}

	resp, _ := json.Marshal(map[string]any{
		"status":       "applied",
		"field":        corr.Field,
		"old_value":    corr.OldValue,
		"new_value":    corr.NewValue,
		"flagged":      true,
		"corrected_by": u.userID,
	})
	return string(resp), nil
}

func findMetric(metrics []digest.Metric, partner, step string) int {
	for i, m := range metrics {
		if m.Partner == partner && m.Step == step {
			return i
		}
	}
	return -1
}
