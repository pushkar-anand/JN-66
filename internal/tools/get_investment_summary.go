package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pushkaranand/finagent/internal/llm"
	"github.com/pushkaranand/finagent/internal/store"
)

// GetInvestmentSummary returns a portfolio overview across equity, SGBs, and MFs.
type GetInvestmentSummary struct {
	userID  string
	zerodha zerodhaQuerier
}

// NewGetInvestmentSummary creates the tool bound to the current user.
func NewGetInvestmentSummary(userID string, zerodha zerodhaQuerier) *GetInvestmentSummary {
	return &GetInvestmentSummary{userID: userID, zerodha: zerodha}
}

func (t *GetInvestmentSummary) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "get_investment_summary",
		Description: "Portfolio overview: total current value, invested value, P&L, and breakdown by asset class (equity, SGB, mutual funds). Auto-refreshes if cache is older than 4 hours.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func (t *GetInvestmentSummary) Execute(ctx context.Context, _ string, _ string) (string, error) {
	eqByType, err := t.zerodha.GetEquityHoldingsByType(ctx, t.userID)
	if err != nil {
		if errors.Is(err, store.ErrZerodhaTokenExpired) {
			return "Zerodha token has expired or is not set up. Run: finagent zerodha auth", nil
		}
		return "", fmt.Errorf("get equity by type: %w", err)
	}

	mfSummary, err := t.zerodha.GetMFSummary(ctx, t.userID)
	if err != nil && !errors.Is(err, store.ErrZerodhaTokenExpired) {
		return "", fmt.Errorf("get mf summary: %w", err)
	}
	slog.DebugContext(ctx, "tool:get_investment_summary done")

	var totalCurrentPaise, totalInvestedPaise, totalPnlPaise int64
	var sb strings.Builder

	sb.WriteString("Investment Portfolio Summary:\n\n")
	sb.WriteString(fmt.Sprintf("%-12s %6s %14s %14s %12s %7s\n",
		"Class", "Count", "Current Value", "Invested", "P&L", "P&L%"))
	sb.WriteString(strings.Repeat("-", 70) + "\n")

	for _, row := range eqByType {
		pct := pnlPercent(row.TotalPnlPaise, row.InvestedValuePaise)
		sign := "+"
		if row.TotalPnlPaise < 0 {
			sign = ""
		}
		fmt.Fprintf(&sb, "%-12s %6d %14.2f %14.2f %s%11.2f %6.1f%%\n",
			row.HoldingType,
			row.HoldingCount,
			float64(row.CurrentValuePaise)/100,
			float64(row.InvestedValuePaise)/100,
			sign, float64(row.TotalPnlPaise)/100, pct)
		totalCurrentPaise += row.CurrentValuePaise
		totalInvestedPaise += row.InvestedValuePaise
		totalPnlPaise += row.TotalPnlPaise
	}

	if mfSummary.HoldingCount > 0 {
		pct := pnlPercent(mfSummary.TotalPnlPaise, mfSummary.InvestedValuePaise)
		sign := "+"
		if mfSummary.TotalPnlPaise < 0 {
			sign = ""
		}
		fmt.Fprintf(&sb, "%-12s %6d %14.2f %14.2f %s%11.2f %6.1f%%\n",
			"mf",
			mfSummary.HoldingCount,
			float64(mfSummary.CurrentValuePaise)/100,
			float64(mfSummary.InvestedValuePaise)/100,
			sign, float64(mfSummary.TotalPnlPaise)/100, pct)
		totalCurrentPaise += mfSummary.CurrentValuePaise
		totalInvestedPaise += mfSummary.InvestedValuePaise
		totalPnlPaise += mfSummary.TotalPnlPaise
	}

	sb.WriteString(strings.Repeat("-", 70) + "\n")
	totalPct := pnlPercent(totalPnlPaise, totalInvestedPaise)
	sign := "+"
	if totalPnlPaise < 0 {
		sign = ""
	}
	fmt.Fprintf(&sb, "%-12s %6s %14.2f %14.2f %s%11.2f %6.1f%%\n",
		"TOTAL", "",
		float64(totalCurrentPaise)/100,
		float64(totalInvestedPaise)/100,
		sign, float64(totalPnlPaise)/100, totalPct)

	return sb.String(), nil
}
