// Package app provides shared wiring used by multiple entry points.
package app

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pushkaranand/finagent/internal/store"
	"github.com/pushkaranand/finagent/internal/tools"
)

// BuildToolRegistry wires the standard set of agent tools for userID and
// returns both the registry and the MemoryStore it wired internally.
// The caller must pass the returned MemoryStore to agent.New so that the
// agent's memory reader and the memory tools share a single instance.
//
// Zerodha investment tools are excluded here because they require a
// different querier depending on the caller:
//   - production (cmd/finagent): pass ZerodhaService for lazy-sync behaviour
//   - eval (cmd/eval): pass ZerodhaStore for DB-only access (no API key needed)
//
// Call registry.Register(tools.NewGetInvestmentSummary(...)) etc. after this
// function returns to add the investment tools with the appropriate querier.
//
// roPool and schema are optional: when roPool is non-nil, get_schema and
// execute_sql are registered using the pre-built schema string.
func BuildToolRegistry(pool *pgxpool.Pool, roPool *pgxpool.Pool, schema string, userID string, embedder store.Embedder) (*tools.Registry, *store.MemoryStore) {
	txnStore := store.NewTransactionStore(pool)
	accountStore := store.NewAccountStore(pool)
	labelStore := store.NewLabelStore(pool)
	recurringStore := store.NewRecurringStore(pool)
	memoryStore := store.NewMemoryStore(pool, embedder)
	fdStore := store.NewFDStore(pool)

	registry := tools.NewRegistry()
	registry.Register(tools.NewQueryTransactions(userID, txnStore))
	registry.Register(tools.NewGetAccountSummary(userID, accountStore))
	registry.Register(tools.NewGetSpendingBreakdown(userID, txnStore))
	registry.Register(tools.NewManageLabels(userID, labelStore))
	registry.Register(tools.NewListRecurring(userID, recurringStore))
	registry.Register(tools.NewRememberFact(userID, memoryStore))
	registry.Register(tools.NewRecallFacts(userID, memoryStore))
	registry.Register(tools.NewManageFD(userID, fdStore))
	registry.Register(tools.NewListFDs(userID, fdStore))
	if roPool != nil {
		registry.Register(tools.NewGetSchema(schema))
		registry.Register(tools.NewExecuteSQL(roPool))
	}
	return registry, memoryStore
}
