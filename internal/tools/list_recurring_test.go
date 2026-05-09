package tools

import (
	"context"
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

func TestListRecurring_FallsBackToBoundUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockrecurringQuerier(ctrl)
	q.EXPECT().List(gomock.Any(), boundUser).Return(nil, nil)

	got, err := NewListRecurring(boundUser, q).Execute(t.Context(), "", `{}`)
	require.NoError(t, err)
	assert.Equal(t, "No active recurring payments found.", got)
}

func TestListRecurring_ExplicitUserIDOverrides(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockrecurringQuerier(ctrl)
	q.EXPECT().List(gomock.Any(), "other-user").Return(nil, nil)

	_, err := NewListRecurring(boundUser, q).Execute(t.Context(), "", `{"user_id":"other-user"}`)
	require.NoError(t, err)
}

func TestListRecurring_VariableAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockrecurringQuerier(ctrl)
	q.EXPECT().List(gomock.Any(), boundUser).Return([]sqlcgen.RecurringPayment{{
		ID:             uuid.New(),
		Name:           "Netflix",
		ExpectedAmount: model.Money(0), // zero → variable
		Frequency:      sqlcgen.FrequencyEnumMonthly,
		NextExpectedAt: pgtype.Date{Valid: false}, // null → unknown
	}}, nil)

	got, err := NewListRecurring(boundUser, q).Execute(context.Background(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "variable")
	assert.Contains(t, got, "unknown")
}

func TestListRecurring_FixedAmountAndDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockrecurringQuerier(ctrl)
	nextDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	q.EXPECT().List(gomock.Any(), boundUser).Return([]sqlcgen.RecurringPayment{{
		ID:             uuid.New(),
		Name:           "Rent",
		ExpectedAmount: model.Money(2500000), // ₹25000.00
		Frequency:      sqlcgen.FrequencyEnumMonthly,
		NextExpectedAt: pgtype.Date{Time: nextDate, Valid: true},
	}}, nil)

	got, err := NewListRecurring(boundUser, q).Execute(context.Background(), "", `{}`)
	require.NoError(t, err)
	assert.Contains(t, got, "₹25000.00")
	assert.Contains(t, got, "2025-06-01")
	assert.Contains(t, got, "Rent")
}

func TestListRecurring_NoPayments(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockrecurringQuerier(ctrl)
	q.EXPECT().List(gomock.Any(), boundUser).Return(nil, nil)

	got, err := NewListRecurring(boundUser, q).Execute(context.Background(), "", `{}`)
	require.NoError(t, err)
	assert.Equal(t, "No active recurring payments found.", got)
}

func TestListRecurring_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockrecurringQuerier(ctrl)
	q.EXPECT().List(gomock.Any(), boundUser).Return(nil, errors.New("db down"))

	_, err := NewListRecurring(boundUser, q).Execute(context.Background(), "", `{}`)
	require.Error(t, err)
}

func TestListRecurring_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockrecurringQuerier(ctrl)

	_, err := NewListRecurring(boundUser, q).Execute(context.Background(), "", `{bad`)
	require.Error(t, err)
}
