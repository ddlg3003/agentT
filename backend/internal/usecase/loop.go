package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/vngcloud/agentt/internal/domain/agent"
	"github.com/vngcloud/agentt/internal/domain/digest"
)

// ErrMaxTurnsExceeded is returned when the agent loop hits its hard turn limit
// without the model producing a final (tool-free) answer. The loop fails loudly
// rather than returning a half-built digest — a partial digest that looks
// complete is worse than a clear error.
var ErrMaxTurnsExceeded = errors.New("agent loop exceeded max turns")

// loopResult is what one agentLoop.run produces: the model's final text message
// plus the audit trail of every tool call made along the way.
type loopResult struct {
	Final   agent.Message
	Sources []digest.Source
	Turns   int
}

// agentLoop is the reusable think → act → observe engine shared by the daily,
// follow-up, and monthly use cases. The only things that vary between them are
// the system prompt, the initial messages, and the tool set — the loop itself
// is identical, which is the whole point: one engine, many invocations.
type agentLoop struct {
	llm      agent.ToolCaller
	tools    map[string]agent.Tool
	defs     []agent.ToolDef
	maxTurns int
	log      *slog.Logger
}

// newAgentLoop builds a loop over the given tool set. The tools' definitions are
// snapshotted once into defs (the schema advertised to the model each turn).
func newAgentLoop(llm agent.ToolCaller, tools []agent.Tool, maxTurns int, log *slog.Logger) *agentLoop {
	reg := make(map[string]agent.Tool, len(tools))
	defs := make([]agent.ToolDef, 0, len(tools))
	for _, t := range tools {
		def := t.Definition()
		reg[def.Name] = t
		defs = append(defs, def)
	}
	return &agentLoop{llm: llm, tools: reg, defs: defs, maxTurns: maxTurns, log: log}
}

// run drives the loop until the model returns a tool-free message (done) or the
// turn limit is hit (error). messages is the seeded conversation (system prompt
// + initial user instruction); the loop appends assistant and tool-result turns
// as it goes.
//
// Stop condition is derived from content — the absence of tool calls — never
// from the provider's stop_reason (per the agentic-harness "observe content,
// not metadata" rule). Tool dispatch is sequential by design (no goroutines):
// the MVP has no tool that benefits from parallelism, and serial-by-default is
// the safe choice.
func (l *agentLoop) run(ctx context.Context, messages []agent.Message) (loopResult, error) {
	var sources []digest.Source

	for turn := 0; turn < l.maxTurns; turn++ {
		resp, err := l.llm.CompleteWithTools(ctx, messages, l.defs)
		if err != nil {
			return loopResult{}, fmt.Errorf("llm turn %d: %w", turn, err)
		}
		messages = append(messages, resp)

		// No tool calls → the model is done. This is the stop signal.
		if len(resp.ToolCalls) == 0 {
			return loopResult{Final: resp, Sources: sources, Turns: turn + 1}, nil
		}

		// Act: dispatch each requested tool sequentially and observe the result.
		results := make([]agent.ToolResult, 0, len(resp.ToolCalls))
		for _, call := range resp.ToolCalls {
			out, src := l.dispatch(ctx, call)
			results = append(results, out)
			if src != nil {
				sources = append(sources, *src)
			}
		}
		// Feed observations back as a single user turn carrying all results.
		messages = append(messages, agent.Message{Role: agent.RoleUser, ToolResults: results})
	}

	l.log.Error("agent loop exceeded max turns", "maxTurns", l.maxTurns)
	return loopResult{}, ErrMaxTurnsExceeded
}

// dispatch runs one tool call. A missing tool or a tool error never aborts the
// loop: the failure is reported back to the model as an error result (with the
// "[DATA UNAVAILABLE: reason]" convention) so it can adapt, and only successful
// calls contribute to the audit trail.
func (l *agentLoop) dispatch(ctx context.Context, call agent.ToolCall) (agent.ToolResult, *digest.Source) {
	tool, ok := l.tools[call.Name]
	if !ok {
		l.log.Warn("unknown tool requested", "tool", call.Name)
		return agent.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("[DATA UNAVAILABLE: no such tool %q]", call.Name),
			IsError: true,
		}, nil
	}

	out, err := tool.Run(ctx, call.Input)
	if err != nil {
		l.log.Warn("tool failed", "tool", call.Name, "error", err)
		return agent.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("[DATA UNAVAILABLE: %s failed: %v]", call.Name, err),
			IsError: true,
		}, nil
	}

	src := &digest.Source{
		ToolName: call.Name,
		Input:    rawOrNull(call.Input),
		Output:   jsonOutput(out),
	}
	return agent.ToolResult{CallID: call.ID, Content: out}, src
}

// rawOrNull returns valid JSON for an audit record, defaulting empty input to
// JSON null so the stored Source is always valid JSON.
func rawOrNull(in json.RawMessage) json.RawMessage {
	if len(in) == 0 {
		return json.RawMessage("null")
	}
	return in
}

// jsonOutput keeps the audit trail valid JSON: if a tool returned JSON it is
// stored verbatim, otherwise (e.g. markdown from get_knowledge) it is encoded
// as a JSON string.
func jsonOutput(out string) json.RawMessage {
	if json.Valid([]byte(out)) {
		return json.RawMessage(out)
	}
	b, _ := json.Marshal(out)
	return b
}
