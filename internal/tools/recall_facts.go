package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pushkaranand/finagent/internal/llm"
)

// RecallFacts searches agent_memories by topic tags.
type RecallFacts struct {
	userID   string
	memories memoryQuerier
}

// NewRecallFacts creates the tool bound to the current user.
func NewRecallFacts(userID string, memories memoryQuerier) *RecallFacts {
	return &RecallFacts{userID: userID, memories: memories}
}

// Definition returns the tool descriptor.
func (t *RecallFacts) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "recall_facts",
		Description: "Search stored memories by topic tags. Use when you need to check if the user has told you something relevant before.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tags": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Topic tags to search by",
				},
			},
			"required": []string{"tags"},
		},
	}
}

type recallFactsArgs struct {
	Tags []string `json:"tags"`
}

// Execute returns matching memories.
func (t *RecallFacts) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args recallFactsArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	rows, err := t.memories.Recall(ctx, t.userID, args.Tags, 10)
	if err != nil {
		return "", fmt.Errorf("recall facts: %w", err)
	}

	if len(rows) == 0 {
		return "No memories found matching those tags.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Recalled %d fact(s):\n\n", len(rows))
	for _, m := range rows {
		fmt.Fprintf(&sb, "• [%s] %s\n", m.MemoryType, m.Content)
	}
	return sb.String(), nil
}
