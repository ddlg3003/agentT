package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/vngcloud/agentt/internal/domain/agent"
)

// Anthropic is an agent.LLMClient backed by Claude via the official
// anthropic-sdk-go. It maps the domain message list onto the Messages API
// (system prompt + alternating user/assistant turns) and extracts the
// assistant's text from the response.
type Anthropic struct {
	client       anthropic.Client
	model        anthropic.Model
	maxTokens    int64
	systemPrompt string
}

// AnthropicOptions configures the Claude client.
type AnthropicOptions struct {
	APIKey       string
	BaseURL      string // optional; overrides https://api.anthropic.com/
	Model        string // defaults to claude-opus-4-8
	MaxTokens    int64  // defaults to 4096
	SystemPrompt string
}

// NewAnthropic builds a Claude-backed LLM client.
func NewAnthropic(opts AnthropicOptions) *Anthropic {
	model := anthropic.ModelClaudeOpus4_8
	if opts.Model != "" {
		model = anthropic.Model(opts.Model)
	}
	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	clientOpts := []option.RequestOption{option.WithAPIKey(opts.APIKey)}
	if opts.BaseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(opts.BaseURL))
	}
	return &Anthropic{
		client:       anthropic.NewClient(clientOpts...),
		model:        model,
		maxTokens:    maxTokens,
		systemPrompt: opts.SystemPrompt,
	}
}

var (
	_ agent.LLMClient  = (*Anthropic)(nil)
	_ agent.ToolCaller = (*Anthropic)(nil)
)

// Complete sends the conversation to Claude and returns the assistant's reply.
// Domain system messages are merged into the top-level system prompt; user and
// assistant messages become the Messages array.
func (a *Anthropic) Complete(ctx context.Context, messages []agent.Message) (agent.Message, error) {
	var systemParts []string
	if a.systemPrompt != "" {
		systemParts = append(systemParts, a.systemPrompt)
	}

	turns := make([]anthropic.MessageParam, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case agent.RoleSystem:
			systemParts = append(systemParts, m.Content)
		case agent.RoleAssistant:
			turns = append(turns, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		default: // user
			turns = append(turns, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: a.maxTokens,
		Messages:  turns,
	}
	if len(systemParts) > 0 {
		params.System = []anthropic.TextBlockParam{{Text: strings.Join(systemParts, "\n\n")}}
	}

	resp, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return agent.Message{}, err
	}

	var b strings.Builder
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(text.Text)
		}
	}
	return agent.Message{Role: agent.RoleAssistant, Content: b.String()}, nil
}

// CompleteWithTools runs one Messages API turn with tools advertised. The
// returned message carries any tool_use blocks as agent.ToolCalls (the model
// wants to act) and any text as Content; an empty ToolCalls slice means the
// model is done — that is the loop's content-derived stop signal.
func (a *Anthropic) CompleteWithTools(ctx context.Context, messages []agent.Message, tools []agent.ToolDef) (agent.Message, error) {
	var systemParts []string
	if a.systemPrompt != "" {
		systemParts = append(systemParts, a.systemPrompt)
	}

	turns := make([]anthropic.MessageParam, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case agent.RoleSystem:
			systemParts = append(systemParts, m.Content)
		case agent.RoleAssistant:
			blocks := make([]anthropic.ContentBlockParamUnion, 0, len(m.ToolCalls)+1)
			if m.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(m.Content))
			}
			for _, tc := range m.ToolCalls {
				var input any
				if len(tc.Input) > 0 {
					if err := json.Unmarshal(tc.Input, &input); err != nil {
						return agent.Message{}, fmt.Errorf("decode tool call input: %w", err)
					}
				}
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
			}
			turns = append(turns, anthropic.NewAssistantMessage(blocks...))
		default: // user
			if len(m.ToolResults) > 0 {
				blocks := make([]anthropic.ContentBlockParamUnion, 0, len(m.ToolResults))
				for _, tr := range m.ToolResults {
					blocks = append(blocks, anthropic.NewToolResultBlock(tr.CallID, tr.Content, tr.IsError))
				}
				turns = append(turns, anthropic.NewUserMessage(blocks...))
			} else {
				turns = append(turns, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
			}
		}
	}

	toolParams := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		schema, err := toAnthropicSchema(t.InputSchema)
		if err != nil {
			return agent.Message{}, fmt.Errorf("tool %s schema: %w", t.Name, err)
		}
		toolParams = append(toolParams, anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: schema,
		}})
	}

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: a.maxTokens,
		Messages:  turns,
		Tools:     toolParams,
	}
	if len(systemParts) > 0 {
		params.System = []anthropic.TextBlockParam{{Text: strings.Join(systemParts, "\n\n")}}
	}

	resp, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return agent.Message{}, err
	}

	var b strings.Builder
	var calls []agent.ToolCall
	for _, block := range resp.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			b.WriteString(v.Text)
		case anthropic.ToolUseBlock:
			calls = append(calls, agent.ToolCall{ID: v.ID, Name: v.Name, Input: v.Input})
		}
	}
	return agent.Message{Role: agent.RoleAssistant, Content: b.String(), ToolCalls: calls}, nil
}

// toAnthropicSchema converts a raw JSON Schema (object with "properties" and
// "required") into the SDK's ToolInputSchemaParam.
func toAnthropicSchema(raw json.RawMessage) (anthropic.ToolInputSchemaParam, error) {
	var s struct {
		Properties any      `json:"properties"`
		Required   []string `json:"required"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return anthropic.ToolInputSchemaParam{}, err
	}
	return anthropic.ToolInputSchemaParam{Properties: s.Properties, Required: s.Required}, nil
}
