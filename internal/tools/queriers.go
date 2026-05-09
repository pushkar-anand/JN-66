package tools

//go:generate go tool mockgen -source=queriers.go -destination=mock_queriers_test.go -package=tools

import (
	"context"
	"time"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

type accountQuerier interface {
	ListByUser(ctx context.Context, userID string) ([]sqlcgen.Account, error)
}

type transactionQuerier interface {
	List(ctx context.Context, p store.ListTransactionsParams) ([]sqlcgen.VTransaction, error)
	GetSpendingByCategory(ctx context.Context, userID string, from, to time.Time, accountID *string) ([]store.SpendingRow, error)
}

type recurringQuerier interface {
	List(ctx context.Context, userID string) ([]sqlcgen.RecurringPayment, error)
}

type labelQuerier interface {
	AddToTransaction(ctx context.Context, txnID, labelID string) error
	RemoveFromTransaction(ctx context.Context, txnID, labelID string) error
}

type memoryQuerier interface {
	Save(ctx context.Context, userID *string, content string, memType sqlcgen.MemoryTypeEnum, tags []string) (*sqlcgen.AgentMemory, error)
	Recall(ctx context.Context, userID string, queryTags []string, limit int32) ([]sqlcgen.AgentMemory, error)
}
