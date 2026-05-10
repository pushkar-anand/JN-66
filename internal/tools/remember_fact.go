package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pushkaranand/finagent/internal/llm"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// RememberFact stores a fact in agent_memories.
type RememberFact struct {
	userID   string
	memories memoryQuerier
}

// NewRememberFact creates the tool bound to the current user.
func NewRememberFact(userID string, memories memoryQuerier) *RememberFact {
	return &RememberFact{userID: userID, memories: memories}
}

// Definition returns the tool descriptor.
func (t *RememberFact) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "remember_fact",
		Description: "Store a fact the user has stated so it can be recalled in future conversations. Use for payment habits (rent, bills, autopay dates), merchant hints, tagging rules, and user preferences.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content":     map[string]any{"type": "string", "description": "The fact to remember, in plain text"},
				"memory_type": map[string]any{"type": "string", "description": "general|tagging_hint|recurring_hint|preference"},
				"tags":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Topic tags for retrieval"},
				"user_id":     map[string]any{"type": "string", "description": "User this memory belongs to (empty = current user)"},
			},
			"required": []string{"content"},
		},
	}
}

type rememberFactArgs struct {
	Content    string   `json:"content"`
	MemoryType string   `json:"memory_type"`
	Tags       []string `json:"tags"`
	UserID     string   `json:"user_id"`
}

// Execute saves the memory.
func (t *RememberFact) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args rememberFactArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	memType := sqlcgen.MemoryTypeEnumGeneral
	switch args.MemoryType {
	case "tagging_hint":
		memType = sqlcgen.MemoryTypeEnumTaggingHint
	case "recurring_hint":
		memType = sqlcgen.MemoryTypeEnumRecurringHint
	case "preference":
		memType = sqlcgen.MemoryTypeEnumPreference
	}

	userID := t.userID
	if args.UserID != "" {
		userID = args.UserID
	}

	tags := args.Tags
	if len(tags) == 0 {
		tags = autoTags(args.Content)
	}

	m, err := t.memories.Save(ctx, &userID, args.Content, memType, tags)
	if err != nil {
		return "", fmt.Errorf("save memory: %w", err)
	}
	return fmt.Sprintf("Remembered: %q (id: %s)", m.Content, m.ID), nil
}
