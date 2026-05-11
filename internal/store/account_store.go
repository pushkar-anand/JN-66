package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pushkaranand/finagent/internal/model"
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
		Metadata:          []byte("{}"),
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

// AccountMetaParams holds optional metadata extracted from a statement file.
type AccountMetaParams struct {
	AccountNumber string
	IFSC          string
}

// FindOrCreate finds the user's single account for the given institution, or creates one.
// Returns the account and true if it was newly created.
// Returns an error if the user has multiple accounts for that institution — the caller
// should ask the user to pass an explicit --account flag.
func (s *AccountStore) FindOrCreate(ctx context.Context, userID, institution string, meta AccountMetaParams) (*sqlcgen.Account, bool, error) {
	accounts, err := s.ListByUser(ctx, userID)
	if err != nil {
		return nil, false, err
	}

	var matches []sqlcgen.Account
	for _, a := range accounts {
		if strings.EqualFold(a.Institution, institution) {
			matches = append(matches, a)
		}
	}

	switch len(matches) {
	case 0:
		// Auto-create.
		name := strings.ToUpper(institution) + " Savings"
		if len(meta.AccountNumber) >= 4 {
			name += " ****" + meta.AccountNumber[len(meta.AccountNumber)-4:]
		}
		a, err := s.Create(ctx, CreateAccountParams{
			Institution: institution,
			Name:        name,
			AccountType: sqlcgen.AccountTypeEnumBankSavings,
			Currency:    "INR",
			IsActive:    true,
		}, userID)
		if err != nil {
			return nil, false, err
		}
		if meta.AccountNumber != "" || meta.IFSC != "" {
			var acctNum, ifsc *string
			if meta.AccountNumber != "" {
				acctNum = &meta.AccountNumber
			}
			if meta.IFSC != "" {
				ifsc = &meta.IFSC
			}
			_ = s.q.UpsertAccountDetails(ctx, sqlcgen.UpsertAccountDetailsParams{
				AccountID:     a.ID,
				AccountNumber: acctNum,
				IfscCode:      ifsc,
			})
		}
		return a, true, nil

	case 1:
		return &matches[0], false, nil

	default:
		return nil, false, fmt.Errorf("found %d %s accounts for this user — pass --account <uuid> to disambiguate", len(matches), institution)
	}
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

// UpdateBalance sets current_balance and balance_as_of for an account.
// The update is guarded: it only applies if asOf is more recent than the stored balance_as_of,
// so re-importing an older statement never overwrites a newer known balance.
func (s *AccountStore) UpdateBalance(ctx context.Context, accountID string, balance model.Money, asOf time.Time) error {
	aid, err := parseUUID(accountID)
	if err != nil {
		return err
	}
	return s.q.UpdateAccountBalance(ctx, sqlcgen.UpdateAccountBalanceParams{
		ID:      aid,
		Balance: balance,
		AsOf:    pgtype.Date{Time: asOf, Valid: true},
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
