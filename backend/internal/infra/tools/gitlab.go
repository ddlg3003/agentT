package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vngcloud/agentt/internal/domain/agent"
)

// Gitlab is the query_gitlab tool: it returns merge requests merged within a
// date range, optionally filtered by project. MRs link code changes (and their
// Jira tickets) to the dates metrics moved.
type Gitlab struct {
	fs mockFS
}

// NewGitlab builds the query_gitlab tool.
func NewGitlab(mockBase string) *Gitlab { return &Gitlab{fs: mockFS{base: mockBase}} }

var _ agent.Tool = (*Gitlab)(nil)

type mergeRequest struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Project           string   `json:"project"`
	MergedAt          string   `json:"merged_at"`
	LinkedJiraTickets []string `json:"linked_jira_tickets"`
}

type gitlabInput struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Project string `json:"project"`
}

// Definition advertises query_gitlab to the model.
func (g *Gitlab) Definition() agent.ToolDef {
	return agent.ToolDef{
		Name: "query_gitlab",
		Description: "Return merge requests merged within a date range, optionally filtered by project. " +
			"Each MR includes linked Jira tickets, connecting shipped code to metric movements.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "from": {"type": "string", "description": "start date inclusive, YYYY-MM-DD"},
    "to": {"type": "string", "description": "end date inclusive, YYYY-MM-DD"},
    "project": {"type": "string", "description": "optional project filter, e.g. loan-api"}
  },
  "required": ["from", "to"]
}`),
	}
}

// Run filters MRs by merged_at within [from,to] plus optional project.
func (g *Gitlab) Run(_ context.Context, input json.RawMessage) (string, error) {
	var in gitlabInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	from, to, err := parseRange(in.From, in.To)
	if err != nil {
		return "", err
	}

	raw, err := g.fs.readFile("gitlab", "merge_requests.json")
	if err != nil {
		return "", err
	}
	var mrs []mergeRequest
	if err := json.Unmarshal(raw, &mrs); err != nil {
		return "", fmt.Errorf("parse gitlab data: %w", err)
	}

	out := make([]mergeRequest, 0, len(mrs))
	for _, mr := range mrs {
		merged, err := time.Parse(time.RFC3339, mr.MergedAt)
		if err != nil {
			continue
		}
		day := merged.UTC().Truncate(24 * time.Hour)
		if day.Before(from) || day.After(to) {
			continue
		}
		if in.Project != "" && mr.Project != in.Project {
			continue
		}
		out = append(out, mr)
	}

	resp, err := json.Marshal(map[string]any{"merge_requests": out})
	if err != nil {
		return "", fmt.Errorf("marshal gitlab response: %w", err)
	}
	return string(resp), nil
}
