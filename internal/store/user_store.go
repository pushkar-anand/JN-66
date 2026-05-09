package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// UserStore handles user lookups.
type UserStore struct {
	DB
	q *sqlcgen.Queries
}

// NewUserStore creates a UserStore backed by pool.
func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

// GetByEmail returns the user with the given email address.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*sqlcgen.User, error) {
	u, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

// GetByID returns the user with the given ID.
func (s *UserStore) GetByID(ctx context.Context, id string) (*sqlcgen.User, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	u, err := s.q.GetUserByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

// List returns all users sorted by name.
func (s *UserStore) List(ctx context.Context) ([]sqlcgen.User, error) {
	users, err := s.q.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}
