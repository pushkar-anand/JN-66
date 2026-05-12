package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pushkaranand/finagent/internal/llm"
	"github.com/pushkaranand/finagent/internal/store"
)

// GetMFHoldings lists Zerodha mutual fund holdings.
type GetMFHoldings struct {
	userID  string
	zerodha zerodhaQuerier
}

// NewGetMFHoldings creates the tool bound to the current user.
func NewGetMFHoldings(userID string, zerodha zerodhaQuerier) *GetMFHoldings {
	return &GetMFHoldings{userID: userID, zerodha: zerodha}
}

func (t *GetMFHoldings) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "get_mf_holdings",
		Description: "List Zerodha mutual fund holdings with folio, units, NAV, current value, and P&L. Auto-refreshes if cache is older than 4 hours.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func (t *GetMFHoldings) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	// argsJSON may be empty or "{}"
	_ = json.RawMessage(argsJSON)

	holdings, err := t.zerodha.GetMFHoldings(ctx, t.userID)
	if err != nil {
		if errors.Is(err, store.ErrZerodhaTokenExpired) {
			return "Zerodha token has expired or is not set up. Run: finagent zerodha auth", nil
		}
		return "", fmt.Errorf("get mf holdings: %w", err)
	}
	slog.DebugContext(ctx, "tool:get_mf_holdings done", slog.Int("holdings", len(holdings)))

	if len(holdings) == 0 {
		return "No mutual fund holdings found.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Mutual Fund Holdings (%d):\n\n", len(holdings))
	fmt.Fprintf(&sb, "%-40s %-14s %10s %10s %14s %12s %7s\n",
		"Fund", "Folio", "Units", "Avg NAV", "Current Value", "P&L", "P&L%")
	sb.WriteString(strings.Repeat("-", 115) + "\n")

	for _, h := range holdings {
		units := numericToFloat64(h.Units)
		avgNav := float64(h.AvgNavPaise) / 100
		nav := float64(h.NavPaise) / 100
		currentValue := units * nav
		pnl := float64(h.PnlPaise) / 100
		invested := units * avgNav
		pct := 0.0
		if invested != 0 {
			pct = math.Round(pnl/invested*10000) / 100
		}
		sign := "+"
		if pnl < 0 {
			sign = ""
		}
		fmt.Fprintf(&sb, "%-40s %-14s %10.4f %10.2f %14.2f %s%11.2f %6.1f%%\n",
			truncate(h.Fund, 40), h.Folio, units, avgNav, currentValue, sign, pnl, pct)
	}
	return sb.String(), nil
}

func numericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid || n.Int == nil {
		return 0
	}
	f, _ := new(big.Float).SetPrec(64).SetInt(n.Int).Float64()
	if n.Exp != 0 {
		f *= math.Pow10(int(n.Exp))
	}
	return f
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
