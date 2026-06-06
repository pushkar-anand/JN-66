package tools

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
)

// stubFD returns a minimal FixedDeposit for use in mock returns.
func stubFD(id uuid.UUID) *sqlcgen.FixedDeposit {
	return &sqlcgen.FixedDeposit{
		ID:              id,
		AccountID:       uuid.New(),
		PrincipalAmount: model.Money(5000000), // ₹50,000
		InterestRateBps: 725,                  // 7.25%
		TenureMonths:    6,
		StartDate:       pgtype.Date{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		MaturityDate:    pgtype.Date{Time: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		InterestPayout:  sqlcgen.FdPayoutEnumCumulative,
		AutoRenewalType: sqlcgen.FdRenewalTypeEnumNone,
		Status:          sqlcgen.FdStatusEnumActive,
	}
}

func TestManageFD_Definition(t *testing.T) {
	def := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Definition()
	assert.Equal(t, "manage_fd", def.Name)
}

func TestManageFD_InvalidJSON(t *testing.T) {
	_, err := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Execute(t.Context(), "", `{bad`)
	require.Error(t, err)
}

func TestManageFD_UnknownAction(t *testing.T) {
	_, err := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Execute(t.Context(), "", `{"action":"delete"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}

// --- create ---

func TestManageFD_Create_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	fdID := uuid.New()
	m.EXPECT().CreateWithAccount(gomock.Any(), gomock.Any()).Return(stubFD(fdID), nil)

	got, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"create",
		"institution":"sbi",
		"principal_amount":50000,
		"interest_rate":7.25,
		"tenure_months":6,
		"start_date":"2026-06-01",
		"maturity_date":"2026-12-01"
	}`)
	require.NoError(t, err)
	assert.Contains(t, got, fdID.String())
	assert.Contains(t, got, "7.25%")
}

func TestManageFD_Create_DefaultsToCurrentUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	m.EXPECT().CreateWithAccount(gomock.Any(), gomock.AssignableToTypeOf(store.CreateFDParams{})).
		DoAndReturn(func(_ any, p store.CreateFDParams) (*sqlcgen.FixedDeposit, error) {
			assert.Equal(t, boundUser, p.UserID)
			return stubFD(uuid.New()), nil
		})

	_, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"create","institution":"hdfc","principal_amount":10000,
		"interest_rate":7.0,"tenure_months":12,
		"start_date":"2026-01-01","maturity_date":"2027-01-01"
	}`)
	require.NoError(t, err)
}

func TestManageFD_Create_ExplicitUserIDOverrides(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	m.EXPECT().CreateWithAccount(gomock.Any(), gomock.AssignableToTypeOf(store.CreateFDParams{})).
		DoAndReturn(func(_ any, p store.CreateFDParams) (*sqlcgen.FixedDeposit, error) {
			assert.Equal(t, "other-user", p.UserID)
			return stubFD(uuid.New()), nil
		})

	_, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"create","user_id":"other-user","institution":"hdfc",
		"principal_amount":10000,"interest_rate":7.0,"tenure_months":12,
		"start_date":"2026-01-01","maturity_date":"2027-01-01"
	}`)
	require.NoError(t, err)
}

func TestManageFD_Create_MissingInstitution(t *testing.T) {
	_, err := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Execute(t.Context(), "", `{
		"action":"create","principal_amount":50000,"interest_rate":7.25,
		"tenure_months":6,"start_date":"2026-06-01","maturity_date":"2026-12-01"
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "institution")
}

func TestManageFD_Create_MissingPrincipal(t *testing.T) {
	_, err := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Execute(t.Context(), "", `{
		"action":"create","institution":"sbi","interest_rate":7.25,
		"tenure_months":6,"start_date":"2026-06-01","maturity_date":"2026-12-01"
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "principal_amount")
}

func TestManageFD_Create_BadStartDate(t *testing.T) {
	_, err := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Execute(t.Context(), "", `{
		"action":"create","institution":"sbi","principal_amount":50000,
		"interest_rate":7.25,"tenure_months":6,
		"start_date":"not-a-date","maturity_date":"2026-12-01"
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start_date")
}

func TestManageFD_Create_InvalidInterestPayout(t *testing.T) {
	_, err := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Execute(t.Context(), "", `{
		"action":"create","institution":"sbi","principal_amount":50000,
		"interest_rate":7.25,"tenure_months":6,
		"start_date":"2026-06-01","maturity_date":"2026-12-01",
		"interest_payout":"weekly"
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interest_payout")
}

func TestManageFD_Create_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	m.EXPECT().CreateWithAccount(gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))

	_, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"create","institution":"sbi","principal_amount":50000,
		"interest_rate":7.25,"tenure_months":6,
		"start_date":"2026-06-01","maturity_date":"2026-12-01"
	}`)
	require.Error(t, err)
}

func TestManageFD_Create_PrincipalConvertedToPaise(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	m.EXPECT().CreateWithAccount(gomock.Any(), gomock.AssignableToTypeOf(store.CreateFDParams{})).
		DoAndReturn(func(_ any, p store.CreateFDParams) (*sqlcgen.FixedDeposit, error) {
			assert.Equal(t, model.Money(5000000), p.PrincipalAmount) // ₹50,000 = 5000000 paise
			assert.Equal(t, int16(725), p.InterestRateBps)           // 7.25% = 725 bps
			return stubFD(uuid.New()), nil
		})

	_, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"create","institution":"sbi","principal_amount":50000,
		"interest_rate":7.25,"tenure_months":6,
		"start_date":"2026-06-01","maturity_date":"2026-12-01"
	}`)
	require.NoError(t, err)
}

// --- mark_matured / mark_prematurely_closed ---

func TestManageFD_MarkMatured_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	fdID := uuid.New()
	matured := stubFD(fdID)
	matured.Status = sqlcgen.FdStatusEnumMatured
	matured.ActualPayoutAmount = model.Money(5362500) // ₹53,625
	m.EXPECT().UpdateStatus(gomock.Any(), gomock.AssignableToTypeOf(store.UpdateStatusParams{})).
		DoAndReturn(func(_ any, p store.UpdateStatusParams) (*sqlcgen.FixedDeposit, error) {
			assert.Equal(t, fdID.String(), p.FDID)
			assert.Equal(t, sqlcgen.FdStatusEnumMatured, p.Status)
			assert.Equal(t, model.Money(5362500), p.ActualPayoutAmount)
			return matured, nil
		})

	got, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"mark_matured","fd_id":"`+fdID.String()+`","actual_payout_amount":53625
	}`)
	require.NoError(t, err)
	assert.Contains(t, got, "matured")
}

func TestManageFD_MarkMatured_MissingFDID(t *testing.T) {
	_, err := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Execute(
		t.Context(), "", `{"action":"mark_matured","actual_payout_amount":50000}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fd_id")
}

func TestManageFD_MarkPrematurelyClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	fdID := uuid.New()
	closed := stubFD(fdID)
	closed.Status = sqlcgen.FdStatusEnumPrematurelyClosed
	closed.ActualPayoutAmount = model.Money(4900000)
	m.EXPECT().UpdateStatus(gomock.Any(), gomock.AssignableToTypeOf(store.UpdateStatusParams{})).
		DoAndReturn(func(_ any, p store.UpdateStatusParams) (*sqlcgen.FixedDeposit, error) {
			assert.Equal(t, sqlcgen.FdStatusEnumPrematurelyClosed, p.Status)
			return closed, nil
		})

	got, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"mark_prematurely_closed","fd_id":"`+fdID.String()+`","actual_payout_amount":49000
	}`)
	require.NoError(t, err)
	assert.Contains(t, got, "prematurely_closed")
}

// --- mark_renewed ---

func TestManageFD_Renew_HappyPath_PrincipalOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	oldID := uuid.New()
	newID := uuid.New()

	old := stubFD(oldID)
	old.AutoRenewalType = sqlcgen.FdRenewalTypeEnumPrincipalOnly

	newFD := stubFD(newID)
	newFD.PrincipalAmount = model.Money(5000000) // same as old principal

	m.EXPECT().Get(gomock.Any(), oldID.String(), boundUser).Return(old, nil)
	m.EXPECT().RenewFD(gomock.Any(), gomock.AssignableToTypeOf(store.RenewFDParams{})).
		DoAndReturn(func(_ any, p store.RenewFDParams) (*sqlcgen.FixedDeposit, error) {
			// principal_only: new principal = old principal, not payout amount
			assert.Equal(t, old.PrincipalAmount, p.NewPrincipalAmount)
			return newFD, nil
		})

	got, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"mark_renewed","fd_id":"`+oldID.String()+`","institution":"sbi",
		"actual_payout_amount":53625,"new_interest_rate":7.5,"new_tenure_months":12
	}`)
	require.NoError(t, err)
	assert.Contains(t, got, newID.String())
}

func TestManageFD_Renew_PrincipalAndInterest(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	oldID := uuid.New()

	old := stubFD(oldID)
	old.AutoRenewalType = sqlcgen.FdRenewalTypeEnumPrincipalAndInterest

	m.EXPECT().Get(gomock.Any(), oldID.String(), boundUser).Return(old, nil)
	m.EXPECT().RenewFD(gomock.Any(), gomock.AssignableToTypeOf(store.RenewFDParams{})).
		DoAndReturn(func(_ any, p store.RenewFDParams) (*sqlcgen.FixedDeposit, error) {
			// principal_and_interest: new principal = actual payout amount
			assert.Equal(t, model.Money(5362500), p.NewPrincipalAmount)
			return stubFD(uuid.New()), nil
		})

	_, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"mark_renewed","fd_id":"`+oldID.String()+`","institution":"sbi",
		"actual_payout_amount":53625,"new_interest_rate":7.5,"new_tenure_months":12
	}`)
	require.NoError(t, err)
}

func TestManageFD_Renew_NewStartDateDerivedFromOldMaturity(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	oldID := uuid.New()

	old := stubFD(oldID)
	old.MaturityDate = pgtype.Date{Time: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), Valid: true}

	m.EXPECT().Get(gomock.Any(), oldID.String(), boundUser).Return(old, nil)
	m.EXPECT().RenewFD(gomock.Any(), gomock.AssignableToTypeOf(store.RenewFDParams{})).
		DoAndReturn(func(_ any, p store.RenewFDParams) (*sqlcgen.FixedDeposit, error) {
			assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), p.NewStartDate)
			assert.Equal(t, time.Date(2027, 12, 1, 0, 0, 0, 0, time.UTC), p.NewMaturityDate) // +12 months
			return stubFD(uuid.New()), nil
		})

	_, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"mark_renewed","fd_id":"`+oldID.String()+`","institution":"sbi",
		"actual_payout_amount":53625,"new_interest_rate":7.5,"new_tenure_months":12
	}`)
	require.NoError(t, err)
}

func TestManageFD_Renew_MissingInstitution(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	fdID := uuid.New()
	// No Get call expected — institution is validated before hitting the DB.

	_, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"mark_renewed","fd_id":"`+fdID.String()+`",
		"actual_payout_amount":53625,"new_interest_rate":7.5,"new_tenure_months":12
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "institution")
	_ = ctrl // no expectations, but keep controller for cleanup
}

func TestManageFD_Renew_PayoutTypePropagated(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	oldID := uuid.New()

	old := stubFD(oldID)
	old.InterestPayout = sqlcgen.FdPayoutEnumMonthly

	m.EXPECT().Get(gomock.Any(), oldID.String(), boundUser).Return(old, nil)
	m.EXPECT().RenewFD(gomock.Any(), gomock.AssignableToTypeOf(store.RenewFDParams{})).
		DoAndReturn(func(_ any, p store.RenewFDParams) (*sqlcgen.FixedDeposit, error) {
			assert.Equal(t, sqlcgen.FdPayoutEnumMonthly, p.NewInterestPayout)
			return stubFD(uuid.New()), nil
		})

	_, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"mark_renewed","fd_id":"`+oldID.String()+`","institution":"sbi",
		"actual_payout_amount":53625,"new_interest_rate":7.5,"new_tenure_months":12
	}`)
	require.NoError(t, err)
}

func TestManageFD_Create_RateOverflow(t *testing.T) {
	_, err := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Execute(t.Context(), "", `{
		"action":"create","institution":"sbi","principal_amount":50000,
		"interest_rate":99,"tenure_months":6,
		"start_date":"2026-06-01","maturity_date":"2026-12-01"
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interest_rate")
}

func TestManageFD_Renew_MissingFDID(t *testing.T) {
	_, err := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Execute(
		t.Context(), "", `{"action":"mark_renewed","actual_payout_amount":50000,"new_interest_rate":7.5,"new_tenure_months":12}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fd_id")
}

func TestManageFD_Renew_MissingActualPayout(t *testing.T) {
	fdID := uuid.New()
	_, err := NewManageFD(boundUser, NewMockfdManager(gomock.NewController(t))).Execute(
		t.Context(), "", `{"action":"mark_renewed","fd_id":"`+fdID.String()+`","new_interest_rate":7.5,"new_tenure_months":12}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "actual_payout_amount")
}

func TestManageFD_Renew_GetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdManager(ctrl)
	fdID := uuid.New()
	m.EXPECT().Get(gomock.Any(), fdID.String(), boundUser).Return(nil, errors.New("not found"))

	_, err := NewManageFD(boundUser, m).Execute(t.Context(), "", `{
		"action":"mark_renewed","fd_id":"`+fdID.String()+`","institution":"sbi",
		"actual_payout_amount":53625,"new_interest_rate":7.5,"new_tenure_months":12
	}`)
	require.Error(t, err)
}
