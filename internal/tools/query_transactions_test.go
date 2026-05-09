package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestQueryTransactions_FallsBackToBoundUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	var gotParams store.ListTransactionsParams
	q.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p store.ListTransactionsParams) ([]sqlcgen.VTransaction, error) {
			gotParams = p
			return nil, nil
		})

	_, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Equal(t, boundUser, gotParams.UserID)
}

func TestQueryTransactions_ExplicitUserIDOverrides(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	var gotParams store.ListTransactionsParams
	q.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p store.ListTransactionsParams) ([]sqlcgen.VTransaction, error) {
			gotParams = p
			return nil, nil
		})

	_, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{"user_id":"explicit-user"}`)
	require.NoError(t, err)
	assert.Equal(t, "explicit-user", gotParams.UserID)
}

func TestQueryTransactions_DatesParsedCorrectly(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	var gotParams store.ListTransactionsParams
	q.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p store.ListTransactionsParams) ([]sqlcgen.VTransaction, error) {
			gotParams = p
			return nil, nil
		})

	_, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{"from":"2025-01-01","to":"2025-01-31"}`)
	require.NoError(t, err)
	require.NotNil(t, gotParams.From)
	require.NotNil(t, gotParams.To)
	assert.Equal(t, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), *gotParams.From)
	assert.Equal(t, time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), *gotParams.To)
}

func TestQueryTransactions_InvalidFromDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	_, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{"from":"not-a-date"}`)
	require.Error(t, err)
}

func TestQueryTransactions_OverLimitResetsToDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	var gotParams store.ListTransactionsParams
	q.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p store.ListTransactionsParams) ([]sqlcgen.VTransaction, error) {
			gotParams = p
			return nil, nil
		})

	_, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{"limit":999}`)
	require.NoError(t, err)
	// limit > 50 resets to default 20, not clamped to 50
	assert.Equal(t, int32(20), gotParams.Limit)
}

func TestQueryTransactions_LimitDefaultsTo20(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	var gotParams store.ListTransactionsParams
	q.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p store.ListTransactionsParams) ([]sqlcgen.VTransaction, error) {
			gotParams = p
			return nil, nil
		})

	_, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Equal(t, int32(20), gotParams.Limit)
}

func TestQueryTransactions_CreditShownAsUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	q.EXPECT().List(gomock.Any(), gomock.Any()).Return([]sqlcgen.VTransaction{{
		ID:          uuid.New(),
		Direction:   sqlcgen.TxnDirectionEnumCredit,
		Description: "Salary",
		Amount:      500000,
		TxnDate:     pgtype.Date{Time: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}}, nil)

	got, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "↑")
	assert.NotContains(t, got, "↓")
}

func TestQueryTransactions_DebitShownAsDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	q.EXPECT().List(gomock.Any(), gomock.Any()).Return([]sqlcgen.VTransaction{{
		ID:          uuid.New(),
		Direction:   sqlcgen.TxnDirectionEnumDebit,
		Description: "Zomato",
		Amount:      45000,
		TxnDate:     pgtype.Date{Time: time.Date(2025, 5, 2, 0, 0, 0, 0, time.UTC), Valid: true},
	}}, nil)

	got, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "↓")
}

func TestQueryTransactions_ShowsCounterpartyAndMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	mode := sqlcgen.PaymentModeEnumUpi
	cpty := "Ramesh Kumar"
	q.EXPECT().List(gomock.Any(), gomock.Any()).Return([]sqlcgen.VTransaction{{
		ID:               uuid.New(),
		Direction:        sqlcgen.TxnDirectionEnumDebit,
		Description:      "Transfer",
		Amount:           100000,
		TxnDate:          pgtype.Date{Time: time.Date(2025, 5, 3, 0, 0, 0, 0, time.UTC), Valid: true},
		CounterpartyName: &cpty,
		PaymentMode:      &mode,
	}}, nil)

	got, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "Ramesh Kumar")
	assert.Contains(t, got, "upi")
}

func TestQueryTransactions_NoResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)
	q.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)

	got, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Equal(t, "No transactions found matching the criteria.", got)
}

func TestQueryTransactions_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)
	q.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))

	_, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{}`)
	require.Error(t, err)
}

func TestQueryTransactions_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	_, err := NewQueryTransactions(boundUser, q).Execute(t.Context(), "", `{bad`)
	require.Error(t, err)
}
