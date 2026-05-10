package tools

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pushkaranand/finagent/internal/llm"
)

// GetSpendingBreakdown returns per-category spending totals for a date range.
type GetSpendingBreakdown struct {
	userID string
	txns   transactionQuerier
}

// NewGetSpendingBreakdown creates the tool bound to the current user.
func NewGetSpendingBreakdown(userID string, txns transactionQuerier) *GetSpendingBreakdown {
	return &GetSpendingBreakdown{userID: userID, txns: txns}
}

// Definition returns the tool descriptor.
func (t *GetSpendingBreakdown) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "get_spending_breakdown",
		Description: "Total debit spending grouped by category for a date range. Returns amounts in paise (INR × 100).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id":    map[string]any{"type": "string", "description": "User ID (defaults to current user)"},
				"from":       map[string]any{"type": "string", "description": "Start date YYYY-MM-DD (inclusive)"},
				"to":         map[string]any{"type": "string", "description": "End date YYYY-MM-DD (inclusive)"},
				"account_id": map[string]any{"type": "string", "description": "Filter to a specific account UUID"},
			},
			"required": []string{"from", "to"},
		},
	}
}

type getSpendingBreakdownArgs struct {
	UserID    string `json:"user_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	AccountID string `json:"account_id"`
}

// Execute returns the spending breakdown.
func (t *GetSpendingBreakdown) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args getSpendingBreakdownArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	from, err := time.Parse("2006-01-02", args.From)
	if err != nil {
		return "", fmt.Errorf("invalid from date: %w", err)
	}
	to, err := time.Parse("2006-01-02", args.To)
	if err != nil {
		return "", fmt.Errorf("invalid to date: %w", err)
	}

	var accountID *string
	if args.AccountID != "" {
		accountID = &args.AccountID
	}

	rows, err := t.txns.GetSpendingByCategory(ctx, cmp.Or(args.UserID, t.userID), from, to, accountID)
	if err != nil {
		return "", fmt.Errorf("get spending breakdown: %w", err)
	}

	if len(rows) == 0 {
		return fmt.Sprintf("No spending found between %s and %s.", args.From, args.To), nil
	}

	var total int64
	var sb strings.Builder
	fmt.Fprintf(&sb, "Spending %s → %s:\n\n", args.From, args.To)
	for _, r := range rows {
		indent := ""
		if r.Depth == 1 {
			indent = "  "
		}
		fmt.Fprintf(&sb, "%s• %s: ₹%.2f (%d txns)\n",
			indent, r.CategoryName, float64(r.TotalAmount)/100, r.TxnCount)
		total += r.TotalAmount
	}
	fmt.Fprintf(&sb, "\nTotal: ₹%.2f", float64(total)/100)
	return sb.String(), nil
}
