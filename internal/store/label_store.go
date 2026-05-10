package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// FindOrCreate looks up a label by its slug; creates it as a personal label if not found.
func (s *LabelStore) FindOrCreate(ctx context.Context, userID, name string) (string, error) {
	slug := slugify(name)
	existing, err := s.q.GetLabelBySlug(ctx, slug)
	if err == nil {
		return existing.ID.String(), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("find label: %w", err)
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return "", err
	}
	label, err := s.q.CreateLabel(ctx, sqlcgen.CreateLabelParams{
		UserID: toPgtypeUUID(uid),
		Name:   name,
		Slug:   slug,
	})
	if err != nil {
		return "", fmt.Errorf("create label: %w", err)
	}
	return label.ID.String(), nil
}

// slugify converts a human label name to a URL-safe slug.
func slugify(s string) string {
	var b strings.Builder
	prevHyphen := false
	for _, c := range strings.ToLower(s) {
		if unicode.IsLetter(c) || unicode.IsDigit(c) {
			b.WriteRune(c)
			prevHyphen = false
		} else if !prevHyphen && b.Len() > 0 {
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// ListForTransaction returns all labels attached to a transaction.
func (s *LabelStore) ListForTransaction(ctx context.Context, txnID uuid.UUID) ([]sqlcgen.Label, error) {
	rows, err := s.q.ListTransactionLabels(ctx, txnID)
	if err != nil {
		return nil, fmt.Errorf("list transaction labels: %w", err)
	}
	return rows, nil
}
