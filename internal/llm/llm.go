// Package llm defines the LLM provider interface and all shared request/response types.
package llm

import "context"

// Provider is the interface that every LLM backend must implement.
type Provider interface {
	// Chat sends a chat request and returns the model's response.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	// Embed returns a vector embedding for the given text.
	Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error)
	// Name returns a short identifier for the provider (e.g. "openai").
	Name() string
}

// Role identifies the author of a message in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single turn in a conversation.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // set when Role == RoleTool
	Name       string     `json:"name,omitempty"`          // tool name when Role == RoleTool
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ArgsJSON string `json:"arguments"` // raw JSON string
}

// ToolDefinition describes a callable tool to the LLM.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// ChatRequest is the input to Provider.Chat.
type ChatRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

// ChatResponse is the output from Provider.Chat.
type ChatResponse struct {
	Message    Message `json:"message"`
	StopReason string  `json:"stop_reason"` // "stop" | "tool_calls" | "length"
	InputTokens  int   `json:"input_tokens"`
	OutputTokens int   `json:"output_tokens"`
}

// EmbedRequest is the input to Provider.Embed.
type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbedResponse is the output from Provider.Embed.
type EmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// ToolResultMessage builds the Message that delivers a tool result back to the model.
func ToolResultMessage(toolCallID, result, toolName string) Message {
	return Message{
		Role:       RoleTool,
		Content:    result,
		ToolCallID: toolCallID,
		Name:       toolName,
	}
}

// SystemMessage builds a system-role message.
func SystemMessage(content string) Message {
	return Message{Role: RoleSystem, Content: content}
}

// UserMessage builds a user-role message.
func UserMessage(content string) Message {
	return Message{Role: RoleUser, Content: content}
}
