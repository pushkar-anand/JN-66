package tools

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/pushkaranand/finagent/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetSpendingBreakdown_FallsBackToBoundUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)
	q.EXPECT().GetSpendingByCategory(gomock.Any(), boundUser, gomock.Any(), gomock.Any(), gomock.Nil()).
		Return(nil, nil)

	got, err := NewGetSpendingBreakdown(boundUser, q).Execute(t.Context(), "", `{"from":"2025-01-01","to":"2025-01-31"}`)
	require.NoError(t, err)
	assert.Contains(t, got, "No spending found")
}

func TestGetSpendingBreakdown_InvalidFromDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	_, err := NewGetSpendingBreakdown(boundUser, q).Execute(t.Context(), "", `{"from":"bad","to":"2025-01-31"}`)
	require.Error(t, err)
}

func TestGetSpendingBreakdown_InvalidToDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	_, err := NewGetSpendingBreakdown(boundUser, q).Execute(t.Context(), "", `{"from":"2025-01-01","to":"bad"}`)
	require.Error(t, err)
}

func TestGetSpendingBreakdown_Depth0ContributesToTotal(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	rows := []store.SpendingRow{
		{CategoryID: uuid.New(), CategoryName: "Food", Depth: 0, TotalAmount: 100000, TxnCount: 5},
		{CategoryID: uuid.New(), CategoryName: "Delivery", Depth: 1, TotalAmount: 60000, TxnCount: 3},
		{CategoryID: uuid.New(), CategoryName: "Transport", Depth: 0, TotalAmount: 50000, TxnCount: 2},
	}
	q.EXPECT().GetSpendingByCategory(gomock.Any(), boundUser, gomock.Any(), gomock.Any(), gomock.Nil()).
		Return(rows, nil)

	got, err := NewGetSpendingBreakdown(boundUser, q).Execute(t.Context(), "", `{"from":"2025-01-01","to":"2025-01-31"}`)
	require.NoError(t, err)
	assert.Contains(t, got, "Food")
	assert.Contains(t, got, "Transport")
	assert.Contains(t, got, "Total: ₹1500.00") // (100000 + 50000) / 100
	// depth=1 sub-category is indented
	assert.Contains(t, got, "  • Delivery")
}

func TestGetSpendingBreakdown_NoResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)
	q.EXPECT().GetSpendingByCategory(gomock.Any(), boundUser, gomock.Any(), gomock.Any(), gomock.Nil()).
		Return(nil, nil)

	got, err := NewGetSpendingBreakdown(boundUser, q).Execute(t.Context(), "", `{"from":"2025-01-01","to":"2025-01-31"}`)
	require.NoError(t, err)
	assert.Contains(t, got, "No spending found between")
}

func TestGetSpendingBreakdown_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)
	q.EXPECT().GetSpendingByCategory(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil()).
		Return(nil, errors.New("db down"))

	_, err := NewGetSpendingBreakdown(boundUser, q).Execute(t.Context(), "", `{"from":"2025-01-01","to":"2025-01-31"}`)
	require.Error(t, err)
}

func TestGetSpendingBreakdown_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMocktransactionQuerier(ctrl)

	_, err := NewGetSpendingBreakdown(boundUser, q).Execute(t.Context(), "", `{bad`)
	require.Error(t, err)
}
