package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// RecurringStore handles recurring_payments data access.
type RecurringStore struct {
	DB
	q *sqlcgen.Queries
}

// NewRecurringStore creates a RecurringStore backed by pool.
func NewRecurringStore(pool *pgxpool.Pool) *RecurringStore {
	return &RecurringStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

// List returns all active recurring payments for the user.
func (s *RecurringStore) List(ctx context.Context, userID string) ([]sqlcgen.RecurringPayment, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListRecurringPayments(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("list recurring payments: %w", err)
	}
	return rows, nil
}

// Create inserts a new recurring payment rule.
func (s *RecurringStore) Create(ctx context.Context, p sqlcgen.CreateRecurringPaymentParams) (*sqlcgen.RecurringPayment, error) {
	r, err := s.q.CreateRecurringPayment(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("create recurring payment: %w", err)
	}
	return &r, nil
}

// Deactivate marks a recurring payment as inactive.
func (s *RecurringStore) Deactivate(ctx context.Context, id string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	if err := s.q.DeactivateRecurringPayment(ctx, uid); err != nil {
		return fmt.Errorf("deactivate recurring payment: %w", err)
	}
	return nil
}
