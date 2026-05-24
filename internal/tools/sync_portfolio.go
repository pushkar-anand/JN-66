package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/pushkaranand/finagent/internal/llm"
	"github.com/pushkaranand/finagent/internal/store"
)

// SyncPortfolio fetches fresh Zerodha holdings from the Kite Connect API.
// loginURLFunc, if non-nil, generates a fresh Kite login URL for in-chat
// re-authentication (wired only when the HTTP server is running). Nil in
// CLI-only mode, where the tool falls back to a CLI instruction.
type SyncPortfolio struct {
	userID       string
	zerodha      zerodhaSyncer
	loginURLFunc func() string
}

// NewSyncPortfolio creates the tool. loginURLFunc may be nil (CLI-only mode).
func NewSyncPortfolio(userID string, zerodha zerodhaSyncer, loginURLFunc func() string) *SyncPortfolio {
	return &SyncPortfolio{userID: userID, zerodha: zerodha, loginURLFunc: loginURLFunc}
}

// Definition returns the tool descriptor.
func (t *SyncPortfolio) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "sync_portfolio",
		Description: "Force a fresh sync of Zerodha holdings from the Kite Connect API. Call this when the user asks to sync or refresh their portfolio, when portfolio data is outdated (cache auto-refreshes every 24 hours), or after the user confirms they have completed re-authentication. If the Zerodha session has expired, this tool returns re-authentication instructions.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

// Execute fetches fresh holdings and returns a sync summary.
func (t *SyncPortfolio) Execute(ctx context.Context, _ string, _ string) (string, error) {
	uid, err := uuid.Parse(t.userID)
	if err != nil {
		return "", fmt.Errorf("sync_portfolio: invalid user id %q: %w", t.userID, err)
	}
	slog.DebugContext(ctx, "tool:sync_portfolio start")
	eq, mf, err := t.zerodha.ForceSync(ctx, uid)
	if err != nil {
		if errors.Is(err, store.ErrZerodhaTokenExpired) {
			if t.loginURLFunc != nil {
				return fmt.Sprintf(
					"Your Zerodha session has expired. Please re-authenticate using this link:\n\n%s\n\nLet me know once you've logged in.",
					t.loginURLFunc(),
				), nil
			}
			return "Zerodha token has expired or is not set up. Run: finagent zerodha auth", nil
		}
		return "", fmt.Errorf("sync portfolio: %w", err)
	}
	slog.DebugContext(ctx, "tool:sync_portfolio done", slog.Int("equity", eq), slog.Int("mf", mf))
	return fmt.Sprintf("Synced %d equity holdings and %d mutual fund holdings.", eq, mf), nil
}
