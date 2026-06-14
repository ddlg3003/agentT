package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/vngcloud/agentt/internal/domain/agent"
)

// Knowledge is the get_knowledge tool: it returns the raw markdown for a named
// business-context topic. A simple topic→filename map keeps the mapping explicit
// and the tool decoupled from the on-disk layout.
type Knowledge struct {
	fs    mockFS
	topic map[string]string
}

// NewKnowledge builds the get_knowledge tool with the default topic map.
func NewKnowledge(mockBase string) *Knowledge {
	return &Knowledge{
		fs: mockFS{base: mockBase},
		topic: map[string]string{
			"business_rules":  "business_rules.md",
			"report_template": "report_template.md",
			"funnel_glossary": "funnel_glossary.md",
		},
	}
}

var _ agent.Tool = (*Knowledge)(nil)

type knowledgeInput struct {
	Topic string `json:"topic"`
}

// Definition advertises get_knowledge, listing the available topics inline so the
// model knows exactly what it can request.
func (k *Knowledge) Definition() agent.ToolDef {
	topics := k.topics()
	return agent.ToolDef{
		Name: "get_knowledge",
		Description: "Return the raw markdown for a business-context topic. Call this early to load " +
			"context before reasoning. Available topics: " + strings.Join(topics, ", ") + ".",
		InputSchema: json.RawMessage(fmt.Sprintf(`{
  "type": "object",
  "properties": {
    "topic": {"type": "string", "enum": [%s], "description": "the knowledge topic to load"}
  },
  "required": ["topic"]
}`, quoteJoin(topics))),
	}
}

// Run reads the markdown for the requested topic.
func (k *Knowledge) Run(_ context.Context, input json.RawMessage) (string, error) {
	var in knowledgeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	file, ok := k.topic[in.Topic]
	if !ok {
		return "", fmt.Errorf("unknown topic %q (available: %s)", in.Topic, strings.Join(k.topics(), ", "))
	}
	b, err := k.fs.readFile("knowledge", file)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (k *Knowledge) topics() []string {
	ts := make([]string, 0, len(k.topic))
	for t := range k.topic {
		ts = append(ts, t)
	}
	sort.Strings(ts)
	return ts
}

func quoteJoin(ss []string) string {
	q := make([]string, len(ss))
	for i, s := range ss {
		q[i] = `"` + s + `"`
	}
	return strings.Join(q, ", ")
}
