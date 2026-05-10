package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// AccountStore handles account and account-member data access.
type AccountStore struct {
	DB
	q sqlcgen.Querier
}

// NewAccountStore creates an AccountStore backed by pool.
func NewAccountStore(pool *pgxpool.Pool) *AccountStore {
	return &AccountStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

func newAccountStoreForTest(q sqlcgen.Querier) *AccountStore {
	return &AccountStore{q: q}
}

// ListByUser returns all accounts the user is a member of.
func (s *AccountStore) ListByUser(ctx context.Context, userID string) ([]sqlcgen.Account, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListAccountsByUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("list accounts by user: %w", err)
	}
	return rows, nil
}

// GetByID returns a single account.
func (s *AccountStore) GetByID(ctx context.Context, accountID string) (*sqlcgen.Account, error) {
	aid, err := parseUUID(accountID)
	if err != nil {
		return nil, err
	}
	a, err := s.q.GetAccountByID(ctx, aid)
	if err != nil {
		return nil, fmt.Errorf("get account by id: %w", err)
	}
	return &a, nil
}

// CreateAccountParams groups inputs for Create.
type CreateAccountParams struct {
	Institution       string
	ExternalAccountID *string
	Name              string
	AccountType       sqlcgen.AccountTypeEnum
	Currency          string
	IsActive          bool
}

// Create inserts a new account and makes userID the owner.
func (s *AccountStore) Create(ctx context.Context, p CreateAccountParams, userID string) (*sqlcgen.Account, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}

	a, err := s.q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
		Institution:       p.Institution,
		ExternalAccountID: p.ExternalAccountID,
		Name:              p.Name,
		AccountType:       p.AccountType,
		Currency:          p.Currency,
		IsActive:          p.IsActive,
	})
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	if err := s.q.AddAccountMember(ctx, sqlcgen.AddAccountMemberParams{
		AccountID: a.ID,
		UserID:    uid,
		Role:      sqlcgen.MemberRoleEnumOwner,
	}); err != nil {
		return nil, fmt.Errorf("add account member: %w", err)
	}

	return &a, nil
}

// AddMember adds an additional user to an existing account.
func (s *AccountStore) AddMember(ctx context.Context, accountID, userID string, role sqlcgen.MemberRoleEnum) error {
	aid, err := parseUUID(accountID)
	if err != nil {
		return err
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	return s.q.AddAccountMember(ctx, sqlcgen.AddAccountMemberParams{
		AccountID: aid,
		UserID:    uid,
		Role:      role,
	})
}

// GetDetails returns the account_details row for an account.
func (s *AccountStore) GetDetails(ctx context.Context, accountID uuid.UUID) (*sqlcgen.AccountDetail, error) {
	d, err := s.q.GetAccountDetails(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get account details: %w", err)
	}
	return &d, nil
}
