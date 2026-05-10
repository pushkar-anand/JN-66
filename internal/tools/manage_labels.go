package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pushkaranand/finagent/internal/llm"
)

// ManageLabels adds or removes labels on a transaction by label name.
type ManageLabels struct {
	userID string
	labels labelQuerier
}

// NewManageLabels creates the tool bound to the current user.
func NewManageLabels(userID string, labels labelQuerier) *ManageLabels {
	return &ManageLabels{userID: userID, labels: labels}
}

// Definition returns the tool descriptor.
func (t *ManageLabels) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "manage_labels",
		Description: "Add or remove a label on a transaction by label name. The label is created automatically if it does not exist.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":         map[string]any{"type": "string", "description": "add or remove"},
				"transaction_id": map[string]any{"type": "string", "description": "Transaction UUID"},
				"label_name":     map[string]any{"type": "string", "description": "Label name (e.g. 'food-delivery', 'business')"},
			},
			"required": []string{"action", "transaction_id", "label_name"},
		},
	}
}

type manageLabelsArgs struct {
	Action        string `json:"action"`
	TransactionID string `json:"transaction_id"`
	LabelName     string `json:"label_name"`
}

// Execute adds or removes the label, creating it first if needed.
func (t *ManageLabels) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args manageLabelsArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	labelID, err := t.labels.FindOrCreate(ctx, t.userID, args.LabelName)
	if err != nil {
		return "", fmt.Errorf("resolve label: %w", err)
	}

	switch args.Action {
	case "add":
		if err := t.labels.AddToTransaction(ctx, args.TransactionID, labelID); err != nil {
			return "", fmt.Errorf("add label: %w", err)
		}
		return fmt.Sprintf("Label %q added to transaction %s.", args.LabelName, args.TransactionID), nil
	case "remove":
		if err := t.labels.RemoveFromTransaction(ctx, args.TransactionID, labelID); err != nil {
			return "", fmt.Errorf("remove label: %w", err)
		}
		return fmt.Sprintf("Label %q removed from transaction %s.", args.LabelName, args.TransactionID), nil
	default:
		return "", fmt.Errorf("unknown action %q: use 'add' or 'remove'", args.Action)
	}
}
