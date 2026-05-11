package tools

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pushkaranand/finagent/internal/llm"
	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// GetAccountSummary returns all accounts the user has access to.
type GetAccountSummary struct {
	userID   string
	accounts accountQuerier
}

// NewGetAccountSummary creates the tool bound to the current user.
func NewGetAccountSummary(userID string, accounts accountQuerier) *GetAccountSummary {
	return &GetAccountSummary{userID: userID, accounts: accounts}
}

// Definition returns the tool descriptor.
func (t *GetAccountSummary) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "get_account_summary",
		Description: "List all accounts the user has access to, with type, class, and current balance.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id": map[string]any{"type": "string", "description": "User ID (defaults to current user)"},
			},
		},
	}
}

type getAccountSummaryArgs struct {
	UserID string `json:"user_id"`
}

// Execute returns a formatted account list.
func (t *GetAccountSummary) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args getAccountSummaryArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	accounts, err := t.accounts.ListByUser(ctx, cmp.Or(args.UserID, t.userID))
	if err != nil {
		return "", fmt.Errorf("get accounts: %w", err)
	}

	if len(accounts) == 0 {
		return "No accounts found.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Accounts (%d):\n\n", len(accounts))
	for _, a := range accounts {
		active := ""
		if !a.IsActive {
			active = " [inactive]"
		}
		fmt.Fprintf(&sb, "• %s (%s) — %s/%s%s  %s  id:%s\n",
			a.Name,
			a.Institution,
			a.AccountType,
			a.AccountClass,
			active,
			formatBalance(a),
			a.ID,
		)
	}
	return sb.String(), nil
}

func formatBalance(a sqlcgen.Account) string {
	if !a.BalanceAsOf.Valid {
		if a.AccountClass == sqlcgen.AccountClassEnumLiability {
			return "Outstanding: —"
		}
		return "Balance: —"
	}
	date := a.BalanceAsOf.Time.Format("2 Jan 2006")
	amount := model.Money(a.CurrentBalance).String()
	if a.AccountClass == sqlcgen.AccountClassEnumLiability {
		return fmt.Sprintf("Outstanding: %s (as of %s)", amount, date)
	}
	return fmt.Sprintf("Balance: %s (as of %s)", amount, date)
}
