package llm

import (
	"context"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

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

var _ agent.LLMClient = (*OpenAI)(nil)

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
