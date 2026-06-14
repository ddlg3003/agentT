package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"

	"github.com/vngcloud/agentt/internal/domain/agent"
)

// OpenAI is an agent.LLMClient backed by the OpenAI Chat Completions API.
type OpenAI struct {
	client       openai.Client
	model        string
	maxTokens    int64
	systemPrompt string
}

// OpenAIOptions configures the OpenAI client.
type OpenAIOptions struct {
	APIKey       string
	BaseURL      string // optional; overrides https://api.openai.com/v1/ (useful for proxies or compatible APIs)
	Model        string // defaults to gpt-4o
	MaxTokens    int64  // defaults to 4096
	SystemPrompt string
}

// NewOpenAI builds an OpenAI-backed LLM client.
func NewOpenAI(opts OpenAIOptions) *OpenAI {
	model := "gpt-4o"
	if opts.Model != "" {
		model = opts.Model
	}
	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	clientOpts := []option.RequestOption{option.WithAPIKey(opts.APIKey)}
	if opts.BaseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(opts.BaseURL))
	}
	return &OpenAI{
		client:       openai.NewClient(clientOpts...),
		model:        model,
		maxTokens:    maxTokens,
		systemPrompt: opts.SystemPrompt,
	}
}

var (
	_ agent.LLMClient  = (*OpenAI)(nil)
	_ agent.ToolCaller = (*OpenAI)(nil)
)

// Complete sends the conversation to OpenAI and returns the assistant's reply.
// Domain system messages are merged into a single system turn prepended to the
// messages array.
func (o *OpenAI) Complete(ctx context.Context, messages []agent.Message) (agent.Message, error) {
	var systemParts []string
	if o.systemPrompt != "" {
		systemParts = append(systemParts, o.systemPrompt)
	}

	var turns []openai.ChatCompletionMessageParamUnion
	for _, m := range messages {
		switch m.Role {
		case agent.RoleSystem:
			systemParts = append(systemParts, m.Content)
		case agent.RoleAssistant:
			turns = append(turns, openai.AssistantMessage(m.Content))
		default: // user
			turns = append(turns, openai.UserMessage(m.Content))
		}
	}

	if len(systemParts) > 0 {
		turns = append([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(strings.Join(systemParts, "\n\n")),
		}, turns...)
	}

	resp, err := o.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     o.model,
		Messages:  turns,
		MaxTokens: openai.Int(o.maxTokens),
	})
	if err != nil {
		return agent.Message{}, err
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	return agent.Message{Role: agent.RoleAssistant, Content: content}, nil
}

// CompleteWithTools runs one Chat Completions turn with function tools. Tool
// calls in the response become agent.ToolCalls; an empty slice means the model
// produced a final answer (the loop's content-derived stop signal).
func (o *OpenAI) CompleteWithTools(ctx context.Context, messages []agent.Message, tools []agent.ToolDef) (agent.Message, error) {
	var systemParts []string
	if o.systemPrompt != "" {
		systemParts = append(systemParts, o.systemPrompt)
	}

	var turns []openai.ChatCompletionMessageParamUnion
	for _, m := range messages {
		switch m.Role {
		case agent.RoleSystem:
			systemParts = append(systemParts, m.Content)
		case agent.RoleAssistant:
			asst := openai.ChatCompletionAssistantMessageParam{}
			if m.Content != "" {
				asst.Content.OfString = openai.String(m.Content)
			}
			for _, tc := range m.ToolCalls {
				args := string(tc.Input)
				if args == "" {
					args = "{}"
				}
				asst.ToolCalls = append(asst.ToolCalls, openai.ChatCompletionMessageToolCallParam{
					ID: tc.ID,
					Function: openai.ChatCompletionMessageToolCallFunctionParam{
						Name:      tc.Name,
						Arguments: args,
					},
				})
			}
			turns = append(turns, openai.ChatCompletionMessageParamUnion{OfAssistant: &asst})
		default: // user
			if len(m.ToolResults) > 0 {
				for _, tr := range m.ToolResults {
					turns = append(turns, openai.ToolMessage(tr.Content, tr.CallID))
				}
			} else {
				turns = append(turns, openai.UserMessage(m.Content))
			}
		}
	}

	if len(systemParts) > 0 {
		turns = append([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(strings.Join(systemParts, "\n\n")),
		}, turns...)
	}

	toolParams := make([]openai.ChatCompletionToolParam, 0, len(tools))
	for _, t := range tools {
		var schema map[string]any
		if err := json.Unmarshal(t.InputSchema, &schema); err != nil {
			return agent.Message{}, fmt.Errorf("tool %s schema: %w", t.Name, err)
		}
		toolParams = append(toolParams, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  shared.FunctionParameters(schema),
			},
		})
	}

	resp, err := o.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     o.model,
		Messages:  turns,
		MaxTokens: openai.Int(o.maxTokens),
		Tools:     toolParams,
	})
	if err != nil {
		return agent.Message{}, err
	}
	if len(resp.Choices) == 0 {
		return agent.Message{Role: agent.RoleAssistant}, nil
	}

	msg := resp.Choices[0].Message
	var calls []agent.ToolCall
	for _, tc := range msg.ToolCalls {
		calls = append(calls, agent.ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}
	return agent.Message{Role: agent.RoleAssistant, Content: msg.Content, ToolCalls: calls}, nil
}
