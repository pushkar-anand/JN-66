package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pushkaranand/finagent/internal/llm"
)

// ManageLabels adds or removes labels on a transaction.
type ManageLabels struct {
	labels labelQuerier
}

// NewManageLabels creates the tool.
func NewManageLabels(labels labelQuerier) *ManageLabels {
	return &ManageLabels{labels: labels}
}

// Definition returns the tool descriptor.
func (t *ManageLabels) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "manage_labels",
		Description: "Add or remove a label on a transaction. Use action='add' or action='remove'.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":         map[string]any{"type": "string", "description": "add or remove"},
				"transaction_id": map[string]any{"type": "string", "description": "Transaction UUID"},
				"label_id":       map[string]any{"type": "string", "description": "Label UUID"},
			},
			"required": []string{"action", "transaction_id", "label_id"},
		},
	}
}

type manageLabelsArgs struct {
	Action        string `json:"action"`
	TransactionID string `json:"transaction_id"`
	LabelID       string `json:"label_id"`
}

// Execute adds or removes the label.
func (t *ManageLabels) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args manageLabelsArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	switch args.Action {
	case "add":
		if err := t.labels.AddToTransaction(ctx, args.TransactionID, args.LabelID); err != nil {
			return "", fmt.Errorf("add label: %w", err)
		}
		return fmt.Sprintf("Label %s added to transaction %s.", args.LabelID, args.TransactionID), nil
	case "remove":
		if err := t.labels.RemoveFromTransaction(ctx, args.TransactionID, args.LabelID); err != nil {
			return "", fmt.Errorf("remove label: %w", err)
		}
		return fmt.Sprintf("Label %s removed from transaction %s.", args.LabelID, args.TransactionID), nil
	default:
		return "", fmt.Errorf("unknown action %q: use 'add' or 'remove'", args.Action)
	}
}
