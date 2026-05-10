package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// LabelStore handles label data access.
type LabelStore struct {
	DB
	q sqlcgen.Querier
}

// NewLabelStore creates a LabelStore backed by pool.
func NewLabelStore(pool *pgxpool.Pool) *LabelStore {
	return &LabelStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

func newLabelStoreForTest(q sqlcgen.Querier) *LabelStore {
	return &LabelStore{q: q}
}

// List returns all labels visible to the user (personal + shared).
func (s *LabelStore) List(ctx context.Context, userID string) ([]sqlcgen.Label, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListLabels(ctx, toPgtypeUUID(uid))
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	return rows, nil
}

// AddToTransaction attaches a label to a transaction.
func (s *LabelStore) AddToTransaction(ctx context.Context, txnID, labelID string) error {
	tid, err := parseUUID(txnID)
	if err != nil {
		return err
	}
	lid, err := parseUUID(labelID)
	if err != nil {
		return err
	}
	return s.q.AddTransactionLabel(ctx, sqlcgen.AddTransactionLabelParams{
		TransactionID: tid,
		LabelID:       lid,
		Source:        sqlcgen.DetectionSourceEnumUser,
	})
}

// RemoveFromTransaction detaches a label from a transaction.
func (s *LabelStore) RemoveFromTransaction(ctx context.Context, txnID, labelID string) error {
	tid, err := parseUUID(txnID)
	if err != nil {
		return err
	}
	lid, err := parseUUID(labelID)
	if err != nil {
		return err
	}
	return s.q.RemoveTransactionLabel(ctx, sqlcgen.RemoveTransactionLabelParams{
		TransactionID: tid,
		LabelID:       lid,
	})
}

// ListForTransaction returns all labels attached to a transaction.
func (s *LabelStore) ListForTransaction(ctx context.Context, txnID uuid.UUID) ([]sqlcgen.Label, error) {
	rows, err := s.q.ListTransactionLabels(ctx, txnID)
	if err != nil {
		return nil, fmt.Errorf("list transaction labels: %w", err)
	}
	return rows, nil
}
