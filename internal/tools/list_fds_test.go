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

func TestListFDs_Definition(t *testing.T) {
	def := NewListFDs(boundUser, NewMockfdLister(gomock.NewController(t))).Definition()
	assert.Equal(t, "list_fds", def.Name)
}

func TestListFDs_InvalidJSON(t *testing.T) {
	_, err := NewListFDs(boundUser, NewMockfdLister(gomock.NewController(t))).Execute(t.Context(), "", `{bad`)
	require.Error(t, err)
}

func TestListFDs_EmptyResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdLister(ctrl)
	m.EXPECT().ListByUser(gomock.Any(), gomock.Any()).Return(nil, nil)

	got, err := NewListFDs(boundUser, m).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Equal(t, "No fixed deposits found.", got)
}

func TestListFDs_DefaultsToActiveStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdLister(ctrl)
	active := sqlcgen.FdStatusEnumActive
	m.EXPECT().ListByUser(gomock.Any(), store.ListFDsParams{
		UserID: boundUser,
		Status: &active,
	}).Return(nil, nil)

	_, err := NewListFDs(boundUser, m).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
}

func TestListFDs_StatusAll_PassesNilFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdLister(ctrl)
	m.EXPECT().ListByUser(gomock.Any(), store.ListFDsParams{
		UserID: boundUser,
		Status: nil,
	}).Return(nil, nil)

	_, err := NewListFDs(boundUser, m).Execute(t.Context(), "", `{"status":"all"}`)
	require.NoError(t, err)
}

func TestListFDs_ExplicitUserIDOverrides(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdLister(ctrl)
	active := sqlcgen.FdStatusEnumActive
	m.EXPECT().ListByUser(gomock.Any(), store.ListFDsParams{
		UserID: "other-user",
		Status: &active,
	}).Return(nil, nil)

	_, err := NewListFDs(boundUser, m).Execute(t.Context(), "", `{"user_id":"other-user"}`)
	require.NoError(t, err)
}

func TestListFDs_InvalidStatus(t *testing.T) {
	_, err := NewListFDs(boundUser, NewMockfdLister(gomock.NewController(t))).Execute(t.Context(), "", `{"status":"unknown"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestListFDs_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdLister(ctrl)
	m.EXPECT().ListByUser(gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))

	_, err := NewListFDs(boundUser, m).Execute(t.Context(), "", `{}`)
	require.Error(t, err)
}

func TestListFDs_OutputFormatsAmountAndRate(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdLister(ctrl)
	maturity := time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC)
	m.EXPECT().ListByUser(gomock.Any(), gomock.Any()).Return([]sqlcgen.FixedDeposit{{
		ID:                     uuid.New(),
		PrincipalAmount:        model.Money(10000000), // ₹1,00,000
		InterestRateBps:        750,                   // 7.50%
		TenureMonths:           12,
		StartDate:              pgtype.Date{Time: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		MaturityDate:           pgtype.Date{Time: maturity, Valid: true},
		ExpectedMaturityAmount: model.Money(10750000), // ₹1,07,500
		InterestPayout:         sqlcgen.FdPayoutEnumCumulative,
		AutoRenewalType:        sqlcgen.FdRenewalTypeEnumNone,
		Status:                 sqlcgen.FdStatusEnumActive,
	}}, nil)

	got, err := NewListFDs(boundUser, m).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "7.50%")
	assert.Contains(t, got, "1,00,000") // Indian-comma format via Money.String()
	assert.Contains(t, got, "1,07,500") // expected maturity
	assert.Contains(t, got, "1 Apr 2026")
	assert.Contains(t, got, "1 Apr 2027")
}

func TestListFDs_TotalsPrinted(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdLister(ctrl)
	m.EXPECT().ListByUser(gomock.Any(), gomock.Any()).Return([]sqlcgen.FixedDeposit{
		{
			ID: uuid.New(), PrincipalAmount: model.Money(5000000),
			ExpectedMaturityAmount: model.Money(5362500),
			MaturityDate:           pgtype.Date{Time: time.Now().Add(24 * time.Hour), Valid: true},
			StartDate:              pgtype.Date{Time: time.Now(), Valid: true},
			InterestPayout:         sqlcgen.FdPayoutEnumCumulative,
			AutoRenewalType:        sqlcgen.FdRenewalTypeEnumNone,
			Status:                 sqlcgen.FdStatusEnumActive,
		},
		{
			ID: uuid.New(), PrincipalAmount: model.Money(10000000),
			ExpectedMaturityAmount: model.Money(10750000),
			MaturityDate:           pgtype.Date{Time: time.Now().Add(48 * time.Hour), Valid: true},
			StartDate:              pgtype.Date{Time: time.Now(), Valid: true},
			InterestPayout:         sqlcgen.FdPayoutEnumCumulative,
			AutoRenewalType:        sqlcgen.FdRenewalTypeEnumNone,
			Status:                 sqlcgen.FdStatusEnumActive,
		},
	}, nil)

	got, err := NewListFDs(boundUser, m).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "Total principal")
	assert.Contains(t, got, "Total expected maturity")
}

func TestListFDs_MaturingWithinDays_PassesFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := NewMockfdLister(ctrl)
	active := sqlcgen.FdStatusEnumActive
	m.EXPECT().ListByUser(gomock.Any(), gomock.AssignableToTypeOf(store.ListFDsParams{})).
		DoAndReturn(func(_ any, p store.ListFDsParams) ([]sqlcgen.FixedDeposit, error) {
			assert.Equal(t, &active, p.Status)
			require.NotNil(t, p.MaturingBefore)
			// should be ~30 days from now
			diff := p.MaturingBefore.Sub(time.Now())
			assert.InDelta(t, 30*24*float64(time.Hour), float64(diff), float64(2*time.Hour))
			return nil, nil
		})

	_, err := NewListFDs(boundUser, m).Execute(t.Context(), "", `{"maturing_within_days":30}`)
	require.NoError(t, err)
}
