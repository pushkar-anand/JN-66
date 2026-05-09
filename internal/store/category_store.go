package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// CategoryStore handles category lookups.
type CategoryStore struct {
	DB
	q *sqlcgen.Queries
}

// NewCategoryStore creates a CategoryStore backed by pool.
func NewCategoryStore(pool *pgxpool.Pool) *CategoryStore {
	return &CategoryStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

// List returns all categories ordered by depth then slug.
func (s *CategoryStore) List(ctx context.Context) ([]sqlcgen.Category, error) {
	rows, err := s.q.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return rows, nil
}

// GetBySlug returns a category by its slug.
func (s *CategoryStore) GetBySlug(ctx context.Context, slug string) (*sqlcgen.Category, error) {
	c, err := s.q.GetCategoryBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("get category by slug: %w", err)
	}
	return &c, nil
}
