// Package digest holds the core domain entities and ports for the PO digest
// product: the structured daily memo the agent produces, its audit trail, and
// the persistence port. It depends on nothing outside the standard library —
// storage backends (SQLite today) live in infra and are injected at the edge.
package digest

import (
	"context"
	"encoding/json"
	"time"
)

// DailyDigest is the structured memo for one day: the funnel metrics that
// moved, the events (tickets/MRs) that plausibly explain them, the agent's
// narrative, and a full audit trail of the tool calls that produced it.
//
// The Cash Loan domain is a per-partner conversion funnel (whitelist → traffic
// → demand → pass-rule → filling → submission → approval → signing → e2e),
// tracked over time. A digest captures one day's slice; the monthly rollup
// synthesises many digests into the month-over-month partner report.
type DailyDigest struct {
	Date        string    `json:"date"` // YYYY-MM-DD
	GeneratedAt time.Time `json:"generated_at"`

	Metrics   []Metric `json:"metrics"`   // funnel numbers per partner/step
	Events    []Event  `json:"events"`    // tickets + MRs in the window
	Reasoning string   `json:"reasoning"` // narrative: why the numbers moved
	Sources   []Source `json:"sources"`   // audit: every tool call in/out

	// PO oversight. A digest can be flagged as incorrect and corrected via the
	// follow-up Q&A loop; corrections never overwrite the original value blindly
	// — the prior value is preserved in the Correction record for traceability.
	Flagged     bool         `json:"flagged"`
	FlagNote    string       `json:"flag_note,omitempty"`
	Corrections []Correction `json:"corrections,omitempty"`
}

// Metric is one funnel number for one partner at one step. The funnel is
// per-partner (SHB / CAKE / TNEX / VP ...), so Partner + Step identify it.
type Metric struct {
	Partner  string  `json:"partner"`   // "SHB", "CAKE", "TNEX", "VP", or "ALL"
	Step     string  `json:"step"`      // funnel step code, e.g. "s20s120"
	Name     string  `json:"name"`      // human label, e.g. "E2E Rate"
	Unit     string  `json:"unit"`      // "%" or "#"
	Value    float64 `json:"value"`     // the value on Date
	DeltaDay float64 `json:"delta_day"` // change vs the previous day
	DeltaMoM float64 `json:"delta_mom"` // change vs the same day last month
}

// Event is a ticket or merge request in the digest window that may explain a
// metric movement. The agent is instructed not to assert causality without a
// corresponding Event or business rule to support it.
type Event struct {
	Source        string   `json:"source"` // "jira" | "gitlab"
	ID            string   `json:"id"`     // e.g. "LOAN-451" or "!1234"
	Title         string   `json:"title"`
	Type          string   `json:"type"`   // bug | feature | incident | mr
	Status        string   `json:"status"` // done | merged | ...
	OccurredAt    string   `json:"occurred_at"`
	LinkedTickets []string `json:"linked_tickets,omitempty"`
}

// Source is one entry in the audit trail: which tool was called, with what
// input, and what it returned. The loop records these automatically for every
// tool call, so every number in a digest is traceable to its origin without
// trusting the model to self-report.
type Source struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
	Output   json.RawMessage `json:"output"`
}

// Correction is an audit record of a PO-requested change to a digest, applied
// through the follow-up Q&A loop. The original (AI-generated) value is kept in
// OldValue so the change is fully traceable.
type Correction struct {
	Field    string    `json:"field"`     // e.g. "reasoning" or "metric:SHB/s20s120"
	OldValue string    `json:"old_value"` // value before the correction
	NewValue string    `json:"new_value"` // value the PO requested
	Note     string    `json:"note"`      // PO's reason
	By       string    `json:"by"`        // user id who requested it
	At       time.Time `json:"at"`
}

// Repository is the port for persisting and loading digests. Infra provides the
// concrete store (SQLite); the use case only sees this interface.
type Repository interface {
	// Save persists a digest, overwriting any existing digest for the same date.
	Save(ctx context.Context, d DailyDigest) error
	// Get loads the digest for a date. It returns ErrNotFound if none exists.
	Get(ctx context.Context, date string) (DailyDigest, error)
	// ListDates returns the dates that have a stored digest, newest first.
	ListDates(ctx context.Context) ([]string, error)
	// ListMonth returns all digests whose date falls in the given month
	// (ym = "2026-03"), oldest first — the input to the monthly rollup.
	ListMonth(ctx context.Context, ym string) ([]DailyDigest, error)
}
