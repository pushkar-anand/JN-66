package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// UserStore handles user lookups.
type UserStore struct {
	DB
	q sqlcgen.Querier
}

// NewUserStore creates a UserStore backed by pool.
func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

func newUserStoreForTest(q sqlcgen.Querier) *UserStore {
	return &UserStore{q: q}
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

// CreateUserParams holds inputs for creating a new user.
type CreateUserParams struct {
	Name     string
	Email    string
	Phone    string
	Timezone string
}

// Create inserts a new user and returns the created record.
func (s *UserStore) Create(ctx context.Context, p CreateUserParams) (*sqlcgen.User, error) {
	tz := p.Timezone
	if tz == "" {
		tz = "Asia/Kolkata"
	}
	var phone *string
	if p.Phone != "" {
		phone = &p.Phone
	}
	u, err := s.q.CreateUser(ctx, sqlcgen.CreateUserParams{
		Name:        p.Name,
		Email:       p.Email,
		Phone:       phone,
		Timezone:    tz,
		Preferences: []byte("{}"),
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

// UpdateDOB sets the date_of_birth for the given user.
func (s *UserStore) UpdateDOB(ctx context.Context, userID string, dob time.Time) (*sqlcgen.User, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	u, err := s.q.UpdateUserDOB(ctx, sqlcgen.UpdateUserDOBParams{ID: uid, DateOfBirth: pgDate(dob)})
	if err != nil {
		return nil, fmt.Errorf("update user dob: %w", err)
	}
	return &u, nil
}
