package llm

import (
	"context"
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

var _ agent.LLMClient = (*Anthropic)(nil)

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
