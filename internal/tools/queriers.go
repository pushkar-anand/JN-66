package tools

//go:generate go tool mockgen -source=queriers.go -destination=mock_queriers_test.go -package=tools

import (
	"context"
	"strings"
	"time"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// autoTags extracts lowercase keywords (>4 chars) from text as fallback tags.
func autoTags(text string) []string {
	seen := make(map[string]struct{})
	var tags []string
	word := make([]byte, 0, 16)
	for i := range len(text) {
		c := text[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			if c >= 'A' && c <= 'Z' {
				c += 32
			}
			word = append(word, c)
		} else if len(word) > 0 {
			if len(word) > 4 {
				w := string(word)
				if _, ok := seen[w]; !ok {
					seen[w] = struct{}{}
					tags = append(tags, w)
				}
			}
			word = word[:0]
		}
	}
	if len(word) > 4 {
		w := string(word)
		if _, ok := seen[w]; !ok {
			tags = append(tags, w)
		}
	}
	if len(tags) == 0 {
		// last resort: first word of any length lowercased
		first := strings.Fields(strings.ToLower(text))
		if len(first) > 0 {
			return first[:1]
		}
	}
	return tags
}

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
	FindOrCreate(ctx context.Context, userID, name string) (string, error)
	AddToTransaction(ctx context.Context, txnID, labelID string) error
	RemoveFromTransaction(ctx context.Context, txnID, labelID string) error
}

type memoryQuerier interface {
	Save(ctx context.Context, userID *string, content string, memType sqlcgen.MemoryTypeEnum, tags []string) (*sqlcgen.AgentMemory, error)
	Recall(ctx context.Context, userID string, queryTags []string, limit int32) ([]sqlcgen.AgentMemory, error)
}
