package tools

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pushkaranand/finagent/internal/llm"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// QueryTransactions filters transactions by various criteria.
type QueryTransactions struct {
	userID string
	txns   transactionQuerier
}

// NewQueryTransactions creates the tool bound to the current user.
func NewQueryTransactions(userID string, txns transactionQuerier) *QueryTransactions {
	return &QueryTransactions{userID: userID, txns: txns}
}

// Definition returns the tool descriptor.
func (t *QueryTransactions) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "query_transactions",
		Description: "Filter and list bank transactions for spending, income, investments, or any transaction history. Returns up to 50 results. Use this directly for investment/spending queries without needing account info first. All amounts in paise (INR × 100). Dates in YYYY-MM-DD.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id":                 map[string]any{"type": "string", "description": "User ID to query (defaults to current user)"},
				"from":                    map[string]any{"type": "string", "description": "Start date YYYY-MM-DD (inclusive)"},
				"to":                      map[string]any{"type": "string", "description": "End date YYYY-MM-DD (inclusive)"},
				"account_id":              map[string]any{"type": "string", "description": "Filter to a specific account UUID"},
				"category_id":             map[string]any{"type": "string", "description": "Filter to a specific category UUID"},
				"min_amount":              map[string]any{"type": "integer", "description": "Minimum amount in paise"},
				"max_amount":              map[string]any{"type": "integer", "description": "Maximum amount in paise"},
				"payment_mode":            map[string]any{"type": "string", "description": "upi|neft|rtgs|imps|nach|cheque|atm|pos|emi|online|upi_autopay"},
				"counterparty_identifier": map[string]any{"type": "string", "description": "VPA or account+IFSC"},
				"direction":               map[string]any{"type": "string", "description": "debit or credit"},
				"limit":                   map[string]any{"type": "integer", "description": "Max results (default 20, max 50)"},
				"offset":                  map[string]any{"type": "integer", "description": "Pagination offset"},
			},
		},
	}
}

type queryTransactionsArgs struct {
	UserID                 string `json:"user_id"`
	From                   string `json:"from"`
	To                     string `json:"to"`
	AccountID              string `json:"account_id"`
	CategoryID             string `json:"category_id"`
	MinAmount              *int64 `json:"min_amount"`
	MaxAmount              *int64 `json:"max_amount"`
	PaymentMode            string `json:"payment_mode"`
	CounterpartyIdentifier string `json:"counterparty_identifier"`
	Direction              string `json:"direction"`
	Limit                  int32  `json:"limit"`
	Offset                 int32  `json:"offset"`
}

// Execute runs the transaction query.
func (t *QueryTransactions) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args queryTransactionsArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	limit := args.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	p := store.ListTransactionsParams{
		UserID:    cmp.Or(args.UserID, t.userID),
		MinAmount: args.MinAmount,
		MaxAmount: args.MaxAmount,
		Limit:     limit,
		Offset:    args.Offset,
	}

	if args.From != "" {
		d, err := time.Parse("2006-01-02", args.From)
		if err != nil {
			return "", fmt.Errorf("invalid from date: %w", err)
		}
		p.From = &d
	}
	if args.To != "" {
		d, err := time.Parse("2006-01-02", args.To)
		if err != nil {
			return "", fmt.Errorf("invalid to date: %w", err)
		}
		p.To = &d
	}
	if args.AccountID != "" {
		p.AccountID = &args.AccountID
	}
	if args.CategoryID != "" {
		p.CategoryID = &args.CategoryID
	}
	if args.CounterpartyIdentifier != "" {
		p.CounterpartyIdentifier = &args.CounterpartyIdentifier
	}
	if args.PaymentMode != "" {
		// Take only the first value — model sometimes passes comma-separated modes.
		raw := strings.SplitN(args.PaymentMode, ",", 2)[0]
		mode := sqlcgen.PaymentModeEnum(strings.TrimSpace(raw))
		p.PaymentMode = &mode
	}
	if args.Direction != "" {
		dir := sqlcgen.TxnDirectionEnum(args.Direction)
		p.Direction = &dir
	}

	rows, err := t.txns.List(ctx, p)
	if err != nil {
		return "", fmt.Errorf("query transactions: %w", err)
	}

	if len(rows) == 0 {
		return "No transactions found matching the criteria.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d transaction(s):\n\n", len(rows))
	for _, r := range rows {
		dir := "↓"
		if r.Direction == sqlcgen.TxnDirectionEnumCredit {
			dir = "↑"
		}
		fmt.Fprintf(&sb, "id:%s  %s %s %s ₹%.2f",
			r.ID,
			r.TxnDate.Time.Format("2006-01-02"),
			dir,
			r.Description,
			float64(r.Amount)/100,
		)
		if r.CounterpartyName != nil {
			fmt.Fprintf(&sb, " (from/to: %s)", *r.CounterpartyName)
		}
		if r.PaymentMode != nil {
			fmt.Fprintf(&sb, " [%s]", *r.PaymentMode)
		}
		fmt.Fprintln(&sb)
	}
	return sb.String(), nil
}
