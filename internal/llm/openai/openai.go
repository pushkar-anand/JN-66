// Package openai implements the llm.Provider interface using an OpenAI API-compatible backend.
package openai

import (
	"context"
	"fmt"

	goai "github.com/sashabaranov/go-openai"

	"github.com/pushkaranand/finagent/internal/llm"
)

// Client wraps go-openai and implements llm.Provider.
type Client struct {
	inner *goai.Client
}

// New creates a new Client pointed at the given base URL.
func New(baseURL, apiKey string) *Client {
	cfg := goai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return &Client{inner: goai.NewClientWithConfig(cfg)}
}

// Name returns "openai".
func (c *Client) Name() string { return "openai" }

// Chat sends a chat completion request.
func (c *Client) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	msgs := make([]goai.ChatCompletionMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = toGoAIMessage(m)
	}

	tools := make([]goai.Tool, len(req.Tools))
	for i, t := range req.Tools {
		tools[i] = goai.Tool{
			Type: goai.ToolTypeFunction,
			Function: &goai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}

	greq := goai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: msgs,
	}
	if len(tools) > 0 {
		greq.Tools = tools
	}

	resp, err := c.inner.CreateChatCompletion(ctx, greq)
	if err != nil {
		return llm.ChatResponse{}, fmt.Errorf("chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return llm.ChatResponse{}, fmt.Errorf("chat completion: no choices returned")
	}

	choice := resp.Choices[0]
	msg := fromGoAIMessage(choice.Message)

	stopReason := string(choice.FinishReason)
	if stopReason == "tool_calls" {
		stopReason = "tool_calls"
	}

	return llm.ChatResponse{
		Message:      msg,
		StopReason:   stopReason,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}, nil
}

// Embed returns a text embedding vector.
func (c *Client) Embed(ctx context.Context, req llm.EmbedRequest) (llm.EmbedResponse, error) {
	resp, err := c.inner.CreateEmbeddings(ctx, goai.EmbeddingRequest{
		Model: goai.EmbeddingModel(req.Model),
		Input: req.Input,
	})
	if err != nil {
		return llm.EmbedResponse{}, fmt.Errorf("embed: %w", err)
	}
	if len(resp.Data) == 0 {
		return llm.EmbedResponse{}, fmt.Errorf("embed: no data returned")
	}
	return llm.EmbedResponse{Embedding: resp.Data[0].Embedding}, nil
}

func toGoAIMessage(m llm.Message) goai.ChatCompletionMessage {
	msg := goai.ChatCompletionMessage{
		Role:       string(m.Role),
		Content:    m.Content,
		ToolCallID: m.ToolCallID,
		Name:       m.Name,
	}
	for _, tc := range m.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, goai.ToolCall{
			ID:   tc.ID,
			Type: goai.ToolTypeFunction,
			Function: goai.FunctionCall{
				Name:      tc.Name,
				Arguments: tc.ArgsJSON,
			},
		})
	}
	return msg
}

func fromGoAIMessage(m goai.ChatCompletionMessage) llm.Message {
	msg := llm.Message{
		Role:    llm.Role(m.Role),
		Content: m.Content,
	}
	for _, tc := range m.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, llm.ToolCall{
			ID:       tc.ID,
			Name:     tc.Function.Name,
			ArgsJSON: tc.Function.Arguments,
		})
	}
	return msg
}
