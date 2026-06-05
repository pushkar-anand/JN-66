package eval

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// Scenarios is the full eval suite. UserID is filled by the runner at startup.
var Scenarios = []EvalCase{
	{
		Name:              "account_summary",
		Input:             "What accounts do I have?",
		MustCallTools:     []string{"get_account_summary"},
		OutputMustContain: []string{"HDFC"},
	},
	{
		Name:          "spending_breakdown",
		Input:         "How much did I spend in April 2026?",
		MustCallTools: []string{"get_spending_breakdown"},
		MaxLLMRounds:  3,
		// Static: must at least mention a rupee amount.
		OutputMustContain: []string{"₹"},
		// Dynamic: verify the agent reports the correct total from DB.
		ComputeExpected: func(ctx context.Context, pool *pgxpool.Pool, userID string) ([]string, error) {
			txnStore := store.NewTransactionStore(pool)
			from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC)
			rows, err := txnStore.GetSpendingByCategory(ctx, userID, from, to, nil)
			if err != nil {
				return nil, err
			}
			var total int64
			for _, r := range rows {
				total += r.TotalAmount
			}
			return paiseToINRStrings(total), nil
		},
	},
	{
		Name:                   "investment_direct",
		Input:                  "How much did I invest in May 2026?",
		MustCallTools:          []string{"query_transactions"},
		MaxLLMRounds:           6,
		OutputMustContainOneOf: []string{"5,000", "5000", "₹5"},
	},
	{
		Name:              "transactions_list",
		Input:             "Show me my last 5 transactions",
		MustCallTools:     []string{"query_transactions"},
		OutputMustContain: []string{"Zomato", "NACH"},
	},
	{
		Name:              "recurring_list",
		Input:             "What are my active subscriptions?",
		MustCallTools:     []string{"list_recurring"},
		OutputMustContain: []string{"Netflix"},
	},
	{
		Name:          "remember_fact",
		Input:         "Remember that I pay rent of ₹25,000 every month to my landlord",
		MustCallTools: []string{"remember_fact"},
	},
	{
		Name:              "recall_after_remember",
		PreambleInputs:    []string{"Remember that I pay rent of ₹25,000 every month to my landlord"},
		Input:             "What do you know about my rent from your memory?",
		MustCallTools:     []string{"recall_facts"},
		OutputMustContain: []string{"25,000", "rent"},
	},
	{
		Name:                   "label_transaction",
		Input:                  "Show me my last 5 transactions and label the Zomato one as food-delivery",
		MustCallTools:          []string{"query_transactions", "manage_labels"},
		MaxLLMRounds:           6,
		OutputMustContain:      []string{"food-delivery"},
		OutputMustContainOneOf: []string{"added", "labeled", "tagged", "applied"},
	},
	{
		Name:              "fd_list",
		Input:             "What fixed deposits do I have?",
		MustCallTools:     []string{"list_fds"},
		MaxLLMRounds:      3,
		OutputMustContain: []string{"7.50", "1,00,000"},
	},
	{
		Name:                   "fd_record",
		Input:                  "I opened an FD at SBI: ₹50,000 at 7.25%, 6 months, starts 2026-06-01, matures 2026-12-01",
		MustCallTools:          []string{"manage_fd"},
		MaxLLMRounds:           4,
		OutputMustContainOneOf: []string{"created", "recorded", "saved", "FD"},
	},
	{
		// Agent must ask for missing details rather than calling manage_fd with incomplete data.
		Name:             "fd_incomplete_prompts_for_details",
		Input:            "I opened an FD",
		MustNotCallTools: []string{"manage_fd"},
		MaxLLMRounds:     3,
		OutputMustContainOneOf: []string{
			"principal", "amount", "interest", "rate", "tenure", "months", "maturity", "bank", "institution",
		},
	},
	{
		Name:         "max_rounds_respected",
		Input:        "Analyse everything about my finances",
		MaxLLMRounds: 8,
		// No tool or output assertions — just verify agent returns without panic.
	},
	{
		// Agent can answer using either get_account_summary or get_investment_summary;
		// both are valid paths. Assert only on output content.
		Name:              "has_zerodha_account",
		Input:             "Do I have a Zerodha account?",
		MaxLLMRounds:      4,
		OutputMustContain: []string{"zerodha"},
	},
	{
		Name:              "equity_summary",
		Input:             "What is my equity portfolio worth?",
		MustCallTools:     []string{"get_investment_summary"},
		MaxLLMRounds:      3,
		OutputMustContain: []string{"₹"},
		// Dynamic: verify the agent cites the equity-only value (not equity+SGB).
		// GetEquitySummary includes SGB, so we use GetEquityHoldingsByType to isolate equity.
		ComputeExpected: func(ctx context.Context, pool *pgxpool.Pool, userID string) ([]string, error) {
			rows, err := store.NewZerodhaStore(pool).GetEquityHoldingsByType(ctx, userID)
			if err != nil {
				return nil, err
			}
			return paiseToINRStrings(equityPaiseOnly(rows)), nil
		},
	},
	{
		// "What MFs do I hold?" → agent lists individual holdings, no portfolio total.
		// Dynamic value check doesn't apply here; tool-call and ₹-symbol checks suffice.
		Name:              "mf_summary",
		Input:             "What mutual funds do I hold?",
		MustCallTools:     []string{"get_mf_holdings"},
		MaxLLMRounds:      3,
		OutputMustContain: []string{"₹"},
	},
	{
		Name:              "portfolio_total",
		Input:             "What is my total investment portfolio value across equity and mutual funds?",
		MustCallTools:     []string{"get_investment_summary"},
		MaxLLMRounds:      3,
		OutputMustContain: []string{"₹"},
		// Dynamic: verify the agent cites the correct combined equity + MF value from DB.
		ComputeExpected: func(ctx context.Context, pool *pgxpool.Pool, userID string) ([]string, error) {
			zs := store.NewZerodhaStore(pool)
			rows, err := zs.GetEquityHoldingsByType(ctx, userID)
			if err != nil {
				return nil, err
			}
			mf, err := zs.GetMFSummary(ctx, userID)
			if err != nil {
				return nil, err
			}
			return paiseToINRStrings(equityPaiseOnly(rows) + mf.CurrentValuePaise), nil
		},
	},
}

// equityPaiseOnly returns the current value in paise for the "equity" type row
// from a GetEquityHoldingsByType result set (excludes SGB).
func equityPaiseOnly(rows []sqlcgen.GetZerodhaEquityHoldingsByTypeRow) int64 {
	for _, r := range rows {
		if r.HoldingType == "equity" {
			return r.CurrentValuePaise
		}
	}
	return 0
}

// paiseToINRStrings returns common string representations of a paise amount that
// an LLM might use in its response. For amounts below 1 lakh, Indian and Western
// grouping are identical. For amounts ≥ 1 lakh both formats are included.
//
// Example: 4874200 paise → ["48742", "48,742"]
// Example: 52345678 paise → ["523456", "5,23,456", "523,456"]
func paiseToINRStrings(paise int64) []string {
	rupees := paise / 100
	raw := strconv.FormatInt(rupees, 10)
	indian := indianComma(rupees)
	seen := map[string]bool{raw: true}
	out := []string{raw}
	if !seen[indian] {
		seen[indian] = true
		out = append(out, indian)
	}
	if rupees >= 100000 {
		western := westernComma(rupees)
		if !seen[western] {
			out = append(out, western)
		}
	}
	return out
}

// indianComma formats n with Indian grouping (last group of 3, then groups of 2).
// E.g. 523456 → "5,23,456".
func indianComma(n int64) string {
	if n < 0 {
		return "-" + indianComma(-n)
	}
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	result := s[len(s)-3:]
	s = s[:len(s)-3]
	for len(s) > 2 {
		result = s[len(s)-2:] + "," + result
		s = s[:len(s)-2]
	}
	return fmt.Sprintf("%s,%s", s, result)
}

// westernComma formats n with standard Western grouping (groups of 3).
// E.g. 523456 → "523,456".
func westernComma(n int64) string {
	if n < 0 {
		return "-" + westernComma(-n)
	}
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	result := ""
	for i, ch := range s {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			result += ","
		}
		result += string(ch)
	}
	return result
}
