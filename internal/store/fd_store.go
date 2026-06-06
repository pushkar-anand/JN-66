package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// fdExternalID returns bank_fd_number if set, otherwise a new UUID string.
// Used as external_account_id since the accounts table requires a unique (institution, external_account_id).
func fdExternalID(bankFDNumber *string) string {
	if bankFDNumber != nil && *bankFDNumber != "" {
		return *bankFDNumber
	}
	return uuid.NewString()
}

// FDStore handles fixed deposit data access.
type FDStore struct {
	DB
	q sqlcgen.Querier
}

// NewFDStore creates an FDStore backed by pool.
func NewFDStore(pool *pgxpool.Pool) *FDStore {
	return &FDStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

// CreateFDParams groups inputs for CreateWithAccount.
type CreateFDParams struct {
	UserID                 string
	Institution            string
	AccountName            string
	BankFDNumber           *string
	PrincipalAmount        model.Money
	InterestRateBps        int16
	TenureMonths           int16
	StartDate              time.Time
	MaturityDate           time.Time
	ExpectedMaturityAmount model.Money
	InterestPayout         sqlcgen.FdPayoutEnum
	AutoRenewalType        sqlcgen.FdRenewalTypeEnum
	RenewedFromID          *string
	Notes                  *string
}

// CreateWithAccount atomically creates an fd account and a fixed_deposits row.
func (s *FDStore) CreateWithAccount(ctx context.Context, p CreateFDParams) (*sqlcgen.FixedDeposit, error) {
	uid, err := parseUUID(p.UserID)
	if err != nil {
		return nil, err
	}

	var renewedFrom pgtype.UUID
	if p.RenewedFromID != nil {
		rfid, err := uuid.Parse(*p.RenewedFromID)
		if err != nil {
			return nil, fmt.Errorf("invalid renewed_from_id: %w", err)
		}
		renewedFrom = pgtype.UUID{Bytes: rfid, Valid: true}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := sqlcgen.New(tx)

	acct, err := q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
		Institution:       p.Institution,
		ExternalAccountID: fdExternalID(p.BankFDNumber),
		Name:              p.AccountName,
		AccountType:       sqlcgen.AccountTypeEnumFd,
		Currency:          "INR",
		IsActive:          true,
		Metadata:          []byte("{}"),
	})
	if err != nil {
		return nil, fmt.Errorf("create fd account: %w", err)
	}

	if err := q.AddAccountMember(ctx, sqlcgen.AddAccountMemberParams{
		AccountID: acct.ID,
		UserID:    uid,
		Role:      sqlcgen.MemberRoleEnumOwner,
	}); err != nil {
		return nil, fmt.Errorf("add account member: %w", err)
	}

	fd, err := q.CreateFixedDeposit(ctx, sqlcgen.CreateFixedDepositParams{
		UserID:                 uid,
		AccountID:              acct.ID,
		BankFdNumber:           p.BankFDNumber,
		PrincipalAmount:        p.PrincipalAmount,
		InterestRateBps:        p.InterestRateBps,
		TenureMonths:           p.TenureMonths,
		StartDate:              pgDate(p.StartDate),
		MaturityDate:           pgDate(p.MaturityDate),
		ExpectedMaturityAmount: p.ExpectedMaturityAmount,
		InterestPayout:         p.InterestPayout,
		AutoRenewalType:        p.AutoRenewalType,
		RenewedFromID:          renewedFrom,
		Notes:                  p.Notes,
	})
	if err != nil {
		return nil, fmt.Errorf("create fixed deposit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &fd, nil
}

// RenewFDParams groups inputs for RenewFD.
type RenewFDParams struct {
	UserID             string
	OldFDID            string
	ActualPayoutAmount model.Money
	Institution        string
	NewAccountName     string
	NewBankFDNumber    *string
	NewPrincipalAmount model.Money
	NewInterestRateBps int16
	NewTenureMonths    int16
	NewStartDate       time.Time
	NewMaturityDate    time.Time
	NewInterestPayout  sqlcgen.FdPayoutEnum
	NewAutoRenewalType sqlcgen.FdRenewalTypeEnum
	Notes              *string
}

// RenewFD atomically marks the old FD as renewed and creates a new account + FD row.
func (s *FDStore) RenewFD(ctx context.Context, p RenewFDParams) (*sqlcgen.FixedDeposit, error) {
	uid, err := parseUUID(p.UserID)
	if err != nil {
		return nil, err
	}
	oldID, err := parseUUID(p.OldFDID)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := sqlcgen.New(tx)

	if _, err := q.UpdateFixedDepositStatus(ctx, sqlcgen.UpdateFixedDepositStatusParams{
		Status:             sqlcgen.FdStatusEnumRenewed,
		ActualPayoutAmount: p.ActualPayoutAmount,
		ID:                 oldID,
		UserID:             uid,
	}); err != nil {
		return nil, fmt.Errorf("mark old fd renewed: %w", err)
	}

	acct, err := q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
		Institution:       p.Institution,
		ExternalAccountID: fdExternalID(p.NewBankFDNumber),
		Name:              p.NewAccountName,
		AccountType:       sqlcgen.AccountTypeEnumFd,
		Currency:          "INR",
		IsActive:          true,
		Metadata:          []byte("{}"),
	})
	if err != nil {
		return nil, fmt.Errorf("create renewed fd account: %w", err)
	}

	if err := q.AddAccountMember(ctx, sqlcgen.AddAccountMemberParams{
		AccountID: acct.ID,
		UserID:    uid,
		Role:      sqlcgen.MemberRoleEnumOwner,
	}); err != nil {
		return nil, fmt.Errorf("add account member: %w", err)
	}

	renewedFrom := pgtype.UUID{Bytes: oldID, Valid: true}
	fd, err := q.CreateFixedDeposit(ctx, sqlcgen.CreateFixedDepositParams{
		UserID:                 uid,
		AccountID:              acct.ID,
		BankFdNumber:           p.NewBankFDNumber,
		PrincipalAmount:        p.NewPrincipalAmount,
		InterestRateBps:        p.NewInterestRateBps,
		TenureMonths:           p.NewTenureMonths,
		StartDate:              pgDate(p.NewStartDate),
		MaturityDate:           pgDate(p.NewMaturityDate),
		ExpectedMaturityAmount: 0,
		InterestPayout:         p.NewInterestPayout,
		AutoRenewalType:        p.NewAutoRenewalType,
		RenewedFromID:          renewedFrom,
		Notes:                  p.Notes,
	})
	if err != nil {
		return nil, fmt.Errorf("create renewed fixed deposit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &fd, nil
}

// Get returns a single fixed deposit by ID, scoped to userID.
func (s *FDStore) Get(ctx context.Context, id, userID string) (*sqlcgen.FixedDeposit, error) {
	fid, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	fd, err := s.q.GetFixedDeposit(ctx, sqlcgen.GetFixedDepositParams{ID: fid, UserID: uid})
	if err != nil {
		return nil, fmt.Errorf("get fixed deposit: %w", err)
	}
	return &fd, nil
}

// ListFDsParams groups filters for ListByUser.
type ListFDsParams struct {
	UserID         string
	Status         *sqlcgen.FdStatusEnum
	MaturingBefore *time.Time
}

// ListByUser returns fixed deposits for a user with optional filters.
func (s *FDStore) ListByUser(ctx context.Context, p ListFDsParams) ([]sqlcgen.FixedDeposit, error) {
	uid, err := parseUUID(p.UserID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListFixedDeposits(ctx, sqlcgen.ListFixedDepositsParams{
		UserID:         uid,
		Status:         p.Status,
		MaturingBefore: pgDatePtr(p.MaturingBefore),
	})
	if err != nil {
		return nil, fmt.Errorf("list fixed deposits: %w", err)
	}
	return rows, nil
}

// UpdateStatusParams groups inputs for UpdateStatus.
type UpdateStatusParams struct {
	UserID             string
	FDID               string
	Status             sqlcgen.FdStatusEnum
	ActualPayoutAmount model.Money
}

// UpdateStatus sets the status and actual payout amount on an FD.
func (s *FDStore) UpdateStatus(ctx context.Context, p UpdateStatusParams) (*sqlcgen.FixedDeposit, error) {
	uid, err := parseUUID(p.UserID)
	if err != nil {
		return nil, err
	}
	fid, err := parseUUID(p.FDID)
	if err != nil {
		return nil, err
	}
	fd, err := s.q.UpdateFixedDepositStatus(ctx, sqlcgen.UpdateFixedDepositStatusParams{
		Status:             p.Status,
		ActualPayoutAmount: p.ActualPayoutAmount,
		ID:                 fid,
		UserID:             uid,
	})
	if err != nil {
		return nil, fmt.Errorf("update fixed deposit status: %w", err)
	}
	return &fd, nil
}
