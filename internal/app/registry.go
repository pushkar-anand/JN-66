// Package app provides shared wiring used by multiple entry points.
package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pushkaranand/finagent/internal/store"
	"github.com/pushkaranand/finagent/internal/tools"
)

// BuildToolRegistry wires the standard set of agent tools for userID.
// Zerodha investment tools are excluded here because they require a
// different querier depending on the caller:
//   - production (cmd/finagent): pass ZerodhaService for lazy-sync behaviour
//   - eval (cmd/eval): pass ZerodhaStore for DB-only access (no API key needed)
//
// Call registry.Register(tools.NewGetInvestmentSummary(...)) etc. after this
// function returns to add the investment tools with the appropriate querier.
func BuildToolRegistry(_ context.Context, pool *pgxpool.Pool, userID string) *tools.Registry {
	txnStore := store.NewTransactionStore(pool)
	accountStore := store.NewAccountStore(pool)
	labelStore := store.NewLabelStore(pool)
	recurringStore := store.NewRecurringStore(pool)
	memoryStore := store.NewMemoryStore(pool)

	registry := tools.NewRegistry()
	registry.Register(tools.NewQueryTransactions(userID, txnStore))
	registry.Register(tools.NewGetAccountSummary(userID, accountStore))
	registry.Register(tools.NewGetSpendingBreakdown(userID, txnStore))
	registry.Register(tools.NewManageLabels(userID, labelStore))
	registry.Register(tools.NewListRecurring(userID, recurringStore))
	registry.Register(tools.NewRememberFact(userID, memoryStore))
	registry.Register(tools.NewRecallFacts(userID, memoryStore))
	return registry
}
