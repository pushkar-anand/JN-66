package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// TransactionStore handles transaction and enrichment data access.
type TransactionStore struct {
	DB
	q sqlcgen.Querier
}

// NewTransactionStore creates a TransactionStore backed by pool.
func NewTransactionStore(pool *pgxpool.Pool) *TransactionStore {
	return &TransactionStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

func newTransactionStoreForTest(q sqlcgen.Querier) *TransactionStore {
	return &TransactionStore{q: q}
}

// IdempotencyKeyExists reports whether a transaction with the given key already exists.
func (s *TransactionStore) IdempotencyKeyExists(ctx context.Context, key string) (bool, error) {
	exists, err := s.q.GetIdempotencyKeyExists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("check idempotency key: %w", err)
	}
	return exists, nil
}

// InsertTransactionParams groups all inputs for inserting a new transaction.
type InsertTransactionParams struct {
	AccountID              uuid.UUID
	UserID                 uuid.UUID
	IdempotencyKey         string
	ReferenceNumber        *string
	Amount                 model.Money
	Currency               string
	Direction              sqlcgen.TxnDirectionEnum
	OriginalAmount         *model.Money
	OriginalCurrency       *string
	Description            string
	CounterpartyName       *string
	CounterpartyIdentifier *string
	PaymentMode            *sqlcgen.PaymentModeEnum
	TxnDate                time.Time
	PostedDate             *time.Time
}

// Insert inserts an immutable transaction row and a blank enrichment row.
func (s *TransactionStore) Insert(ctx context.Context, p InsertTransactionParams) (*sqlcgen.Transaction, error) {
	var origAmount model.Money
	if p.OriginalAmount != nil {
		origAmount = *p.OriginalAmount
	}

	t, err := s.q.InsertTransaction(ctx, sqlcgen.InsertTransactionParams{
		AccountID:              p.AccountID,
		UserID:                 p.UserID,
		IdempotencyKey:         p.IdempotencyKey,
		ReferenceNumber:        p.ReferenceNumber,
		Amount:                 p.Amount,
		Currency:               p.Currency,
		Direction:              p.Direction,
		OriginalAmount:         origAmount,
		OriginalCurrency:       p.OriginalCurrency,
		ExchangeRate:           pgtype.Numeric{},
		Description:            p.Description,
		CounterpartyName:       p.CounterpartyName,
		CounterpartyIdentifier: p.CounterpartyIdentifier,
		PaymentMode:            p.PaymentMode,
		TxnDate:                pgDate(p.TxnDate),
		PostedDate:             pgDatePtr(p.PostedDate),
	})
	if err != nil {
		return nil, fmt.Errorf("insert transaction: %w", err)
	}

	// Always create a blank enrichment row so joins never miss.
	if err := s.q.InsertTransactionEnrichment(ctx, t.ID); err != nil {
		return nil, fmt.Errorf("insert enrichment: %w", err)
	}
	return &t, nil
}

// ListTransactionsParams controls ListTransactions filtering.
type ListTransactionsParams struct {
	UserID                 string
	From, To               *time.Time
	AccountID              *string
	CategoryID             *string
	MinAmount, MaxAmount   *int64
	PaymentMode            *sqlcgen.PaymentModeEnum
	CounterpartyIdentifier *string
	Direction              *sqlcgen.TxnDirectionEnum
	Limit                  int32
	Offset                 int32
}

// List returns transactions matching the given filters.
func (s *TransactionStore) List(ctx context.Context, p ListTransactionsParams) ([]sqlcgen.VTransaction, error) {
	uid, err := parseUUID(p.UserID)
	if err != nil {
		return nil, err
	}

	var accountID pgtype.UUID
	if p.AccountID != nil {
		id, err := parseUUID(*p.AccountID)
		if err != nil {
			return nil, err
		}
		accountID = toPgtypeUUID(id)
	}

	var categoryID pgtype.UUID
	if p.CategoryID != nil {
		id, err := parseUUID(*p.CategoryID)
		if err != nil {
			return nil, err
		}
		categoryID = toPgtypeUUID(id)
	}

	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.q.ListTransactions(ctx, sqlcgen.ListTransactionsParams{
		UserID:                 uid,
		FromDate:               pgDatePtr(p.From),
		ToDate:                 pgDatePtr(p.To),
		AccountID:              accountID,
		CategoryID:             categoryID,
		MinAmount:              p.MinAmount,
		MaxAmount:              p.MaxAmount,
		PaymentMode:            p.PaymentMode,
		CounterpartyIdentifier: p.CounterpartyIdentifier,
		Direction:              p.Direction,
		PageLimit:              limit,
		PageOffset:             p.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	return rows, nil
}

// SpendingRow is a single row from GetSpendingByCategory.
type SpendingRow struct {
	CategoryID   uuid.UUID
	CategorySlug string
	CategoryName string
	Depth        int16
	TotalAmount  int64
	TxnCount     int64
}

// GetSpendingByCategory returns per-category spending totals for a date range.
func (s *TransactionStore) GetSpendingByCategory(ctx context.Context, userID string, from, to time.Time, accountID *string) ([]SpendingRow, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}

	var aid pgtype.UUID
	if accountID != nil {
		id, err := parseUUID(*accountID)
		if err != nil {
			return nil, err
		}
		aid = toPgtypeUUID(id)
	}

	rows, err := s.q.GetSpendingByCategory(ctx, sqlcgen.GetSpendingByCategoryParams{
		UserID:    uid,
		FromDate:  pgDate(from),
		ToDate:    pgDate(to),
		AccountID: aid,
	})
	if err != nil {
		return nil, fmt.Errorf("get spending by category: %w", err)
	}

	out := make([]SpendingRow, len(rows))
	for i, r := range rows {
		out[i] = SpendingRow{
			CategoryID:   r.CategoryID,
			CategorySlug: r.CategorySlug,
			CategoryName: r.CategoryName,
			Depth:        r.Depth,
			TotalAmount:  r.TotalAmount,
			TxnCount:     r.TxnCount,
		}
	}
	return out, nil
}

// EnrichmentParams groups the optional fields set during import enrichment.
type EnrichmentParams struct {
	TransactionID         string
	DescriptionNormalized *string
	CategoryID            *string
	CounterpartyName      *string
	CounterpartyID        *string
	TaggingStatus         *sqlcgen.TaggingStatusEnum
}

// UpdateEnrichment applies LLM-derived fields to an existing enrichment row.
func (s *TransactionStore) UpdateEnrichment(ctx context.Context, p EnrichmentParams) error {
	txnID, err := parseUUID(p.TransactionID)
	if err != nil {
		return err
	}

	var catID pgtype.UUID
	if p.CategoryID != nil {
		id, err := parseUUID(*p.CategoryID)
		if err != nil {
			return err
		}
		catID = toPgtypeUUID(id)
	}

	if err := s.q.UpdateEnrichment(ctx, sqlcgen.UpdateEnrichmentParams{
		TransactionID:         txnID,
		DescriptionNormalized: p.DescriptionNormalized,
		CategoryID:            catID,
		TaggingStatus:         p.TaggingStatus,
	}); err != nil {
		return fmt.Errorf("update enrichment: %w", err)
	}
	return nil
}
