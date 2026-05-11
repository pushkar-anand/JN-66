package tools

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const boundUser = "11111111-1111-1111-1111-111111111111"

func TestGetAccountSummary_FallsBackToBoundUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)
	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return(nil, nil)

	got, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Equal(t, "No accounts found.", got)
}

func TestGetAccountSummary_ExplicitUserIDOverrides(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)
	q.EXPECT().ListByUser(gomock.Any(), "other-user").Return(nil, nil)

	_, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{"user_id":"other-user"}`)
	require.NoError(t, err)
}

func TestGetAccountSummary_FormatsActiveAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)

	id := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return([]sqlcgen.Account{{
		ID:           id,
		Name:         "HDFC Savings",
		Institution:  "hdfc",
		AccountType:  sqlcgen.AccountTypeEnumBankSavings,
		AccountClass: sqlcgen.AccountClassEnumAsset,
		IsActive:     true,
	}}, nil)

	got, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "HDFC Savings")
	assert.Contains(t, got, "hdfc")
	assert.Contains(t, got, "id:"+id.String())
	assert.NotContains(t, got, "[inactive]")
}

func TestGetAccountSummary_FormatsInactiveAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)

	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return([]sqlcgen.Account{{
		ID:           uuid.New(),
		Name:         "Old CC",
		Institution:  "citi",
		AccountType:  sqlcgen.AccountTypeEnumCreditCard,
		AccountClass: sqlcgen.AccountClassEnumLiability,
		IsActive:     false,
	}}, nil)

	got, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "[inactive]")
}

func TestGetAccountSummary_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)
	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return(nil, errors.New("db down"))

	_, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.Error(t, err)
}

func TestGetAccountSummary_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)

	_, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `not json`)
	require.Error(t, err)
}

func TestGetAccountSummary_Definition(t *testing.T) {
	q := NewMockaccountQuerier(gomock.NewController(t))
	def := NewGetAccountSummary(boundUser, q).Definition()
	assert.Equal(t, "get_account_summary", def.Name)
}

func TestGetAccountSummary_AssetShowsBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)

	asOf := time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC)
	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return([]sqlcgen.Account{{
		ID:             uuid.New(),
		Name:           "HDFC Savings",
		Institution:    "hdfc",
		AccountType:    sqlcgen.AccountTypeEnumBankSavings,
		AccountClass:   sqlcgen.AccountClassEnumAsset,
		IsActive:       true,
		CurrentBalance: model.Money(12345678), // ₹1,23,456.78
		BalanceAsOf:    pgtype.Date{Time: asOf, Valid: true},
	}}, nil)

	got, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "Balance: ₹123456.78")
	assert.Contains(t, got, "30 Apr 2025")
	assert.NotContains(t, got, "Outstanding")
}

func TestGetAccountSummary_LiabilityShowsOutstanding(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)

	asOf := time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC)
	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return([]sqlcgen.Account{{
		ID:             uuid.New(),
		Name:           "Axis Credit Card",
		Institution:    "axis",
		AccountType:    sqlcgen.AccountTypeEnumCreditCard,
		AccountClass:   sqlcgen.AccountClassEnumLiability,
		IsActive:       true,
		CurrentBalance: model.Money(820000), // ₹8,200.00
		BalanceAsOf:    pgtype.Date{Time: asOf, Valid: true},
	}}, nil)

	got, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "Outstanding: ₹8200.00")
	assert.Contains(t, got, "15 Apr 2025")
	assert.NotContains(t, got, "Balance:")
}

func TestGetAccountSummary_NoBalanceShowsDash(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockaccountQuerier(ctrl)

	q.EXPECT().ListByUser(gomock.Any(), boundUser).Return([]sqlcgen.Account{{
		ID:           uuid.New(),
		Name:         "Zerodha Demat",
		Institution:  "zerodha",
		AccountType:  sqlcgen.AccountTypeEnumDemat,
		AccountClass: sqlcgen.AccountClassEnumAsset,
		IsActive:     true,
		BalanceAsOf:  pgtype.Date{Valid: false},
	}}, nil)

	got, err := NewGetAccountSummary(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "Balance: —")
}
