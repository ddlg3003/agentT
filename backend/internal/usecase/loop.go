package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
// CompactSummary is non-empty when in-loop context compaction fired; callers
// that manage persistent history (e.g. the follow-up loop) should use it to
// update their store rather than appending individual turns.
type loopResult struct {
	Final          agent.Message
	Sources        []digest.Source
	Turns          int
	CompactSummary string          // non-empty if compaction occurred
	CompactRecent  []agent.Message // verbatim messages preserved after compact (excl. system / tool messages)
}

// compactConfig enables in-loop context compaction. When the non-system
// message count crosses threshold, the loop calls the LLM once (no tools) to
// summarise old messages, replaces them with a compact boundary, and continues.
// Compaction fires at most once per run; failure is non-fatal and the loop
// continues with the original messages.
type compactConfig struct {
	threshold  int // non-system message count that triggers compaction
	keepRecent int // verbatim messages to preserve after the compact boundary
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
	compact  *compactConfig // nil = compaction disabled
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
	var compactSummary string
	var compactRecent []agent.Message

	for turn := 0; turn < l.maxTurns; turn++ {
		// Compact once when the context has grown past threshold. The flag
		// (compactSummary != "") prevents a second compaction in the same run.
		if l.compact != nil && compactSummary == "" {
			if compacted, summary, recent, ok := l.maybeCompact(ctx, messages, turn); ok {
				messages = compacted
				compactSummary = summary
				compactRecent = recent
			}
		}
		l.log.InfoContext(ctx, "→ calling model",
			"turn", turn+1, "maxTurns", l.maxTurns, "messages", len(messages))

		start := time.Now()
		resp, err := l.llm.CompleteWithTools(ctx, messages, l.defs)
		if err != nil {
			l.log.ErrorContext(ctx, "✗ model call failed",
				"turn", turn+1, "elapsed", time.Since(start).Round(time.Millisecond), "error", err)
			return loopResult{}, fmt.Errorf("llm turn %d: %w", turn, err)
		}
		messages = append(messages, resp)

		// Surface any thinking text the model emitted alongside its action.
		if text := strings.TrimSpace(resp.Content); text != "" {
			l.log.InfoContext(ctx, "← model said",
				"turn", turn+1, "elapsed", time.Since(start).Round(time.Millisecond),
				"text", preview(text, 200))
		}

		// No tool calls → the model is done. This is the stop signal.
		if len(resp.ToolCalls) == 0 {
			l.log.InfoContext(ctx, "✓ loop done — model returned final answer",
				"turns", turn+1, "toolCalls", len(sources))
			return loopResult{
				Final:          resp,
				Sources:        sources,
				Turns:          turn + 1,
				CompactSummary: compactSummary,
				CompactRecent:  compactRecent,
			}, nil
		}

		names := make([]string, len(resp.ToolCalls))
		for i, c := range resp.ToolCalls {
			names[i] = c.Name
		}
		l.log.InfoContext(ctx, "⚙ model requested tools",
			"turn", turn+1, "count", len(resp.ToolCalls), "tools", strings.Join(names, ","))

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
	// Return any compactSummary produced mid-run so callers can still persist
	// the compact boundary — without it the next request reloads full
	// pre-compact history, triggers compaction again, and hits max turns again.
	return loopResult{CompactSummary: compactSummary}, ErrMaxTurnsExceeded
}

// maybeCompact checks whether messages have grown past the compact threshold
// and, if so, calls the LLM once (no tools) to summarise the old messages.
// Returns (compacted messages, summary, verbatim slice, true) on success, or
// (nil, "", nil, false) if the threshold is not met or the summarisation fails.
// The verbatim slice is the tail of non-system messages preserved as-is; the
// caller stores this alongside the summary for session persistence.
func (l *agentLoop) maybeCompact(ctx context.Context, messages []agent.Message, turn int) ([]agent.Message, string, []agent.Message, bool) {
	// Count leading system messages.
	sysEnd := 0
	for _, m := range messages {
		if m.Role != agent.RoleSystem {
			break
		}
		sysEnd++
	}
	nonSys := messages[sysEnd:]
	if len(nonSys) <= l.compact.threshold {
		return nil, "", nil, false
	}
	keep := l.compact.keepRecent
	if keep >= len(nonSys) {
		return nil, "", nil, false
	}
	toSummarise := nonSys[:len(nonSys)-keep]
	verbatim := nonSys[len(nonSys)-keep:]

	// Build a plain-text representation of the messages to summarise.
	// Tool-call assistant messages have empty Content (the data lives in
	// ToolCalls/ToolResults), so we render those explicitly — otherwise the
	// summarisation prompt contains only blank lines for every tool round-trip
	// and the LLM cannot preserve key findings from those calls.
	var b strings.Builder
	b.WriteString("Summarize the following conversation excerpt into one compact paragraph. " +
		"Preserve every factual correction, key finding, and conclusion. Omit pleasantries.\n\n")
	for _, m := range toSummarise {
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		if m.Content != "" {
			b.WriteString(preview(m.Content, 500))
		}
		for _, tc := range m.ToolCalls {
			fmt.Fprintf(&b, " [tool_call:%s %s]", tc.Name, preview(string(tc.Input), 300))
		}
		for _, tr := range m.ToolResults {
			if tr.IsError {
				fmt.Fprintf(&b, " [tool_result:error %s]", preview(tr.Content, 300))
			} else {
				fmt.Fprintf(&b, " [tool_result %s]", preview(tr.Content, 300))
			}
		}
		b.WriteByte('\n')
	}

	resp, err := l.llm.Complete(ctx, []agent.Message{{Role: agent.RoleUser, Content: b.String()}})
	if err != nil {
		l.log.WarnContext(ctx, "✂ compact summarize failed, continuing without compaction",
			"turn", turn+1, "error", err)
		return nil, "", nil, false
	}
	summary := resp.Content

	// Rebuild: system + compact injection (user+assistant pair) + verbatim.
	injection := []agent.Message{
		{Role: agent.RoleUser, Content: "[Conversation summary]\n" + summary},
		{Role: agent.RoleAssistant, Content: "Understood."},
	}
	result := make([]agent.Message, 0, sysEnd+len(injection)+len(verbatim))
	result = append(result, messages[:sysEnd]...)
	result = append(result, injection...)
	result = append(result, verbatim...)

	l.log.InfoContext(ctx, "✂ context compacted",
		"turn", turn+1, "summarised", len(toSummarise), "kept", len(verbatim),
		"summaryLen", len(summary))
	return result, summary, verbatim, true
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

	l.log.InfoContext(ctx, "  ↳ tool call", "tool", call.Name, "input", preview(string(call.Input), 160))

	start := time.Now()
	out, err := tool.Run(ctx, call.Input)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		l.log.WarnContext(ctx, "  ↳ tool failed", "tool", call.Name, "elapsed", elapsed, "error", err)
		return agent.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("[DATA UNAVAILABLE: %s failed: %v]", call.Name, err),
			IsError: true,
		}, nil
	}
	l.log.InfoContext(ctx, "  ↳ tool ok", "tool", call.Name, "elapsed", elapsed, "bytes", len(out))

	src := &digest.Source{
		ToolName: call.Name,
		Input:    rawOrNull(call.Input),
		Output:   jsonOutput(out),
	}
	return agent.ToolResult{CallID: call.ID, Content: out}, src
}

// preview trims a string to n runes (collapsing newlines) for tidy single-line
// log fields — enough to see what the model/tool is doing without flooding logs.
func preview(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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
