package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pushkaranand/finagent/internal/llm"
	"github.com/pushkaranand/finagent/internal/store"
)

// ListRecurring returns active recurring payments for the user.
type ListRecurring struct {
	recurring *store.RecurringStore
}

// NewListRecurring creates the tool.
func NewListRecurring(recurring *store.RecurringStore) *ListRecurring {
	return &ListRecurring{recurring: recurring}
}

// Definition returns the tool descriptor.
func (t *ListRecurring) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "list_recurring",
		Description: "List all active recurring payments (subscriptions, EMIs, NACH, UPI AutoPay) with expected amounts and next charge dates.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id": map[string]any{"type": "string", "description": "User ID (defaults to current user)"},
			},
		},
	}
}

type listRecurringArgs struct {
	UserID string `json:"user_id"`
}

// Execute returns the list of active recurring payments.
func (t *ListRecurring) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args listRecurringArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	rows, err := t.recurring.List(ctx, args.UserID)
	if err != nil {
		return "", fmt.Errorf("list recurring: %w", err)
	}

	if len(rows) == 0 {
		return "No active recurring payments found.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Active recurring payments (%d):\n\n", len(rows))
	for _, r := range rows {
		// ExpectedAmount is model.Money (int64 alias); zero means unset.
		amount := "variable"
		if r.ExpectedAmount != 0 {
			amount = fmt.Sprintf("₹%.2f", float64(r.ExpectedAmount)/100)
		}
		next := "unknown"
		if r.NextExpectedAt.Valid {
			next = r.NextExpectedAt.Time.Format("2006-01-02")
		}
		fmt.Fprintf(&sb, "• %s — %s/%s, next: %s  id:%s\n",
			r.Name, amount, r.Frequency, next, r.ID)
	}
	return sb.String(), nil
}
