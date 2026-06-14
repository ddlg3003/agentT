package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/vngcloud/agentt/internal/domain/agent"
)

// scriptedLLM returns a pre-programmed message per turn, so loop control flow can
// be tested deterministically without a real provider.
type scriptedLLM struct {
	turns []agent.Message
	calls int
}

func (s *scriptedLLM) Complete(_ context.Context, _ []agent.Message) (agent.Message, error) {
	return agent.Message{}, errors.New("not used")
}

func (s *scriptedLLM) CompleteWithTools(_ context.Context, _ []agent.Message, _ []agent.ToolDef) (agent.Message, error) {
	if s.calls >= len(s.turns) {
		// Keep emitting the last scripted turn (used by the max-turns case).
		return s.turns[len(s.turns)-1], nil
	}
	m := s.turns[s.calls]
	s.calls++
	return m, nil
}

// fakeTool is a no-op tool that records how many times it ran.
type fakeTool struct {
	name string
	out  string
	ran  int
}

func (f *fakeTool) Definition() agent.ToolDef {
	return agent.ToolDef{Name: f.name, Description: "fake", InputSchema: json.RawMessage(`{"type":"object"}`)}
}

func (f *fakeTool) Run(_ context.Context, _ json.RawMessage) (string, error) {
	f.ran++
	return f.out, nil
}

func toolCall(id, name string) agent.ToolCall {
	return agent.ToolCall{ID: id, Name: name, Input: json.RawMessage(`{"x":1}`)}
}

func TestAgentLoop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		turns       []agent.Message
		maxTurns    int
		wantErr     error
		wantTurns   int
		wantSources int
		wantFinal   string
		wantToolRan int
	}{
		{
			name:      "stops immediately when no tool calls",
			turns:     []agent.Message{{Role: agent.RoleAssistant, Content: "done"}},
			maxTurns:  5,
			wantTurns: 1,
			wantFinal: "done",
		},
		{
			name: "dispatches tool then stops on text",
			turns: []agent.Message{
				{Role: agent.RoleAssistant, ToolCalls: []agent.ToolCall{toolCall("c1", "echo_tool")}},
				{Role: agent.RoleAssistant, Content: "final answer"},
			},
			maxTurns:    5,
			wantTurns:   2,
			wantSources: 1,
			wantFinal:   "final answer",
			wantToolRan: 1,
		},
		{
			name: "unknown tool yields error result and no source, loop continues",
			turns: []agent.Message{
				{Role: agent.RoleAssistant, ToolCalls: []agent.ToolCall{toolCall("c1", "missing_tool")}},
				{Role: agent.RoleAssistant, Content: "recovered"},
			},
			maxTurns:    5,
			wantTurns:   2,
			wantSources: 0,
			wantFinal:   "recovered",
		},
		{
			name: "exceeding max turns is a hard error",
			turns: []agent.Message{
				{Role: agent.RoleAssistant, ToolCalls: []agent.ToolCall{toolCall("c1", "echo_tool")}},
			},
			maxTurns: 3,
			wantErr:  ErrMaxTurnsExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ft := &fakeTool{name: "echo_tool", out: `{"ok":true}`}
			llm := &scriptedLLM{turns: tt.turns}
			loop := newAgentLoop(llm, []agent.Tool{ft}, tt.maxTurns, slog.Default())

			res, err := loop.run(context.Background(), []agent.Message{{Role: agent.RoleUser, Content: "go"}})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if res.Turns != tt.wantTurns {
				t.Errorf("turns = %d, want %d", res.Turns, tt.wantTurns)
			}
			if len(res.Sources) != tt.wantSources {
				t.Errorf("sources = %d, want %d", len(res.Sources), tt.wantSources)
			}
			if res.Final.Content != tt.wantFinal {
				t.Errorf("final = %q, want %q", res.Final.Content, tt.wantFinal)
			}
			if tt.wantToolRan != 0 && ft.ran != tt.wantToolRan {
				t.Errorf("tool ran %d, want %d", ft.ran, tt.wantToolRan)
			}
		})
	}
}
