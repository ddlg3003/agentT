package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vngcloud/agentt/internal/domain/agent"
)

// Jira is the query_jira tool: it returns tickets closed within a date range,
// optionally filtered by partner and type. These are the events the agent
// correlates with metric movements.
type Jira struct {
	fs mockFS
}

// NewJira builds the query_jira tool.
func NewJira(mockBase string) *Jira { return &Jira{fs: mockFS{base: mockBase}} }

var _ agent.Tool = (*Jira)(nil)

type jiraTicket struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Type     string  `json:"type"`
	Status   string  `json:"status"`
	Partner  string  `json:"partner"`
	ClosedAt *string `json:"closed_at"`
}

type jiraInput struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Partners []string `json:"partners"`
	Types    []string `json:"types"`
}

// Definition advertises query_jira to the model.
func (j *Jira) Definition() agent.ToolDef {
	return agent.ToolDef{
		Name: "query_jira",
		Description: "Return Jira tickets (bugs, features, incidents) closed within a date range, " +
			"optionally filtered by partner and type. Use this to find events that may explain metric movements.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "from": {"type": "string", "description": "start date inclusive, YYYY-MM-DD"},
    "to": {"type": "string", "description": "end date inclusive, YYYY-MM-DD"},
    "partners": {"type": "array", "items": {"type": "string"}, "description": "optional partner filter"},
    "types": {"type": "array", "items": {"type": "string"}, "description": "optional type filter: bug, feature, incident"}
  },
  "required": ["from", "to"]
}`),
	}
}

// Run filters tickets by closed_at within [from,to] plus optional partner/type.
func (j *Jira) Run(_ context.Context, input json.RawMessage) (string, error) {
	var in jiraInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	from, to, err := parseRange(in.From, in.To)
	if err != nil {
		return "", err
	}

	raw, err := j.fs.readFile("jira", "tickets.json")
	if err != nil {
		return "", err
	}
	var tickets []jiraTicket
	if err := json.Unmarshal(raw, &tickets); err != nil {
		return "", fmt.Errorf("parse jira data: %w", err)
	}

	wantPartner := toSet(in.Partners)
	wantType := toSet(in.Types)

	out := make([]jiraTicket, 0, len(tickets))
	for _, t := range tickets {
		if t.ClosedAt == nil {
			continue // only closed tickets fall in a window
		}
		closed, err := time.Parse(time.RFC3339, *t.ClosedAt)
		if err != nil {
			continue
		}
		day := closed.UTC().Truncate(24 * time.Hour)
		if day.Before(from) || day.After(to) {
			continue
		}
		if len(wantPartner) > 0 && !wantPartner[t.Partner] {
			continue
		}
		if len(wantType) > 0 && !wantType[t.Type] {
			continue
		}
		out = append(out, t)
	}

	resp, err := json.Marshal(map[string]any{"tickets": out})
	if err != nil {
		return "", fmt.Errorf("marshal jira response: %w", err)
	}
	return string(resp), nil
}

// parseRange parses an inclusive [from,to] day range, normalised to UTC midnight.
func parseRange(fromStr, toStr string) (time.Time, time.Time, error) {
	from, err := time.Parse(dateLayout, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid from date %q: %w", fromStr, err)
	}
	to, err := time.Parse(dateLayout, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid to date %q: %w", toStr, err)
	}
	return from, to, nil
}
