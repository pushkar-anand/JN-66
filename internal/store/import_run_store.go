package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// ImportRunStore manages import_runs records.
type ImportRunStore struct {
	DB
	q sqlcgen.Querier
}

// NewImportRunStore creates an ImportRunStore backed by pool.
func NewImportRunStore(pool *pgxpool.Pool) *ImportRunStore {
	return &ImportRunStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

// CreateParams groups inputs for starting a new import run.
type CreateImportRunParams struct {
	UserID    uuid.UUID
	AccountID *uuid.UUID
	Provider  sqlcgen.ImportProviderEnum
	SourceRef string
}

// Create inserts a new import run with status=running.
func (s *ImportRunStore) Create(ctx context.Context, p CreateImportRunParams) (*sqlcgen.ImportRun, error) {
	var accountID pgtype.UUID
	if p.AccountID != nil {
		accountID = toPgtypeUUID(*p.AccountID)
	}

	run, err := s.q.CreateImportRun(ctx, sqlcgen.CreateImportRunParams{
		UserID:    p.UserID,
		AccountID: accountID,
		Provider:  p.Provider,
		SourceRef: &p.SourceRef,
		Metadata:  []byte("{}"),
	})
	if err != nil {
		return nil, fmt.Errorf("create import run: %w", err)
	}
	return &run, nil
}

// UpdateCounts persists running row counts mid-import.
func (s *ImportRunStore) UpdateCounts(ctx context.Context, id uuid.UUID, parsed, inserted, duplicate, failed int) error {
	if err := s.q.UpdateImportRunCounts(ctx, sqlcgen.UpdateImportRunCountsParams{
		ID:            id,
		RowsParsed:    int32(parsed),
		RowsInserted:  int32(inserted),
		RowsDuplicate: int32(duplicate),
		RowsFailed:    int32(failed),
	}); err != nil {
		return fmt.Errorf("update import run counts: %w", err)
	}
	return nil
}

// Finish marks the run as complete (success or failed) with an optional error message.
func (s *ImportRunStore) Finish(ctx context.Context, id uuid.UUID, status sqlcgen.ImportStatusEnum, errDetail string) error {
	var detail *string
	if errDetail != "" {
		detail = &errDetail
	}
	if err := s.q.FinishImportRun(ctx, sqlcgen.FinishImportRunParams{
		ID:          id,
		Status:      status,
		ErrorDetail: detail,
	}); err != nil {
		return fmt.Errorf("finish import run: %w", err)
	}
	return nil
}
