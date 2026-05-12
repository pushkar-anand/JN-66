package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/pushkaranand/finagent/internal/llm"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// GetInvestmentHoldings lists Zerodha equity and SGB holdings.
type GetInvestmentHoldings struct {
	userID  string
	zerodha zerodhaQuerier
}

// NewGetInvestmentHoldings creates the tool bound to the current user.
func NewGetInvestmentHoldings(userID string, zerodha zerodhaQuerier) *GetInvestmentHoldings {
	return &GetInvestmentHoldings{userID: userID, zerodha: zerodha}
}

func (t *GetInvestmentHoldings) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "get_investment_holdings",
		Description: "List Zerodha stock and SGB holdings with quantity, average price, current price, and P&L. Auto-refreshes if cache is older than 4 hours.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filter_type": map[string]any{
					"type":        "string",
					"enum":        []string{"equity", "sgb"},
					"description": "Optional: filter to 'equity' (stocks/ETFs) or 'sgb' (Sovereign Gold Bonds). Omit for all.",
				},
			},
		},
	}
}

type getInvestmentHoldingsArgs struct {
	FilterType string `json:"filter_type"`
}

func (t *GetInvestmentHoldings) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args getInvestmentHoldingsArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	holdings, err := t.zerodha.GetEquityHoldings(ctx, t.userID)
	if err != nil {
		if errors.Is(err, store.ErrZerodhaTokenExpired) {
			return "Zerodha token has expired or is not set up. Run: finagent zerodha auth", nil
		}
		return "", fmt.Errorf("get holdings: %w", err)
	}
	slog.DebugContext(ctx, "tool:get_investment_holdings done", slog.Int("holdings", len(holdings)))

	filtered := filterHoldings(holdings, args.FilterType)
	if len(filtered) == 0 {
		return "No holdings found.", nil
	}

	label := "Holdings"
	switch args.FilterType {
	case "equity":
		label = "Equity Holdings"
	case "sgb":
		label = "SGB Holdings"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s (%d):\n\n", label, len(filtered))
	fmt.Fprintf(&sb, "%-20s %-8s %-7s %6s %12s %12s %12s %7s\n",
		"Symbol", "Exchange", "Type", "Qty", "Avg Price", "Last Price", "P&L", "P&L%")
	sb.WriteString(strings.Repeat("-", 90) + "\n")

	for _, h := range filtered {
		htype := holdingType(h.Tradingsymbol)
		avgRupees := float64(h.AvgPricePaise) / 100
		lastRupees := float64(h.LastPricePaise) / 100
		pnlRupees := float64(h.PnlPaise) / 100
		pnlPct := 0.0
		if h.AvgPricePaise != 0 {
			pnlPct = float64(h.PnlPaise) / float64(int64(h.Quantity)*h.AvgPricePaise) * 100
		}
		sign := "+"
		if pnlRupees < 0 {
			sign = ""
		}
		fmt.Fprintf(&sb, "%-20s %-8s %-7s %6d %12.2f %12.2f %s%11.2f %6.1f%%\n",
			h.Tradingsymbol, h.Exchange, htype, h.Quantity,
			avgRupees, lastRupees, sign, pnlRupees, pnlPct)
	}
	return sb.String(), nil
}

func holdingType(symbol string) string {
	if strings.HasPrefix(symbol, "SGB") {
		return "sgb"
	}
	return "equity"
}

func filterHoldings(holdings []sqlcgen.ZerodhaEquityHolding, filterType string) []sqlcgen.ZerodhaEquityHolding {
	if filterType == "" {
		return holdings
	}
	out := holdings[:0:0]
	for _, h := range holdings {
		if holdingType(h.Tradingsymbol) == filterType {
			out = append(out, h)
		}
	}
	return out
}

// pnlPercent computes P&L percentage given pnl and invested paise.
func pnlPercent(pnlPaise, investedPaise int64) float64 {
	if investedPaise == 0 {
		return 0
	}
	return math.Round(float64(pnlPaise)/float64(investedPaise)*10000) / 100
}
