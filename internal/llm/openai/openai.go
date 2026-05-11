// Package openai implements the llm.Provider interface using an OpenAI API-compatible backend.
package openai

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	goai "github.com/sashabaranov/go-openai"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"

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

	slog.DebugContext(ctx, "llm request",
		slog.String("model", req.Model),
		slog.Int("messages", len(req.Messages)),
		slog.Int("tools", len(req.Tools)),
	)
	t0 := time.Now()

	resp, err := c.inner.CreateChatCompletion(ctx, greq)
	if err != nil {
		slog.ErrorContext(ctx, "llm request failed",
			slog.String("model", req.Model),
			bwglogger.Error(err),
		)
		return llm.ChatResponse{}, fmt.Errorf("chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return llm.ChatResponse{}, fmt.Errorf("chat completion: no choices returned")
	}

	choice := resp.Choices[0]
	msg := fromGoAIMessage(choice.Message)

	stopReason := string(choice.FinishReason)
	slog.DebugContext(ctx, "llm response",
		slog.String("model", req.Model),
		slog.Int("input_tok", resp.Usage.PromptTokens),
		slog.Int("output_tok", resp.Usage.CompletionTokens),
		slog.String("stop", stopReason),
		slog.Int64("dur_ms", time.Since(t0).Milliseconds()),
	)

	return llm.ChatResponse{
		Message:      msg,
		StopReason:   stopReason,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}, nil
}

// Embed returns a text embedding vector.
func (c *Client) Embed(ctx context.Context, req llm.EmbedRequest) (llm.EmbedResponse, error) {
	slog.DebugContext(ctx, "embed request",
		slog.String("model", req.Model),
		slog.Int("inputs", len(req.Input)),
	)
	resp, err := c.inner.CreateEmbeddings(ctx, goai.EmbeddingRequest{
		Model: goai.EmbeddingModel(req.Model),
		Input: req.Input,
	})
	if err != nil {
		slog.ErrorContext(ctx, "embed failed",
			slog.String("model", req.Model),
			bwglogger.Error(err),
		)
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
