package store

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/pushkaranand/finagent/internal/model"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

var (
	testFrom = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	testTo   = time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
)

func TestTransactionStore_IdempotencyKeyExists_True(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	q.EXPECT().GetIdempotencyKeyExists(gomock.Any(), "key-abc").Return(true, nil)

	exists, err := s.IdempotencyKeyExists(t.Context(), "key-abc")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestTransactionStore_IdempotencyKeyExists_False(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	q.EXPECT().GetIdempotencyKeyExists(gomock.Any(), "key-xyz").Return(false, nil)

	exists, err := s.IdempotencyKeyExists(t.Context(), "key-xyz")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestTransactionStore_IdempotencyKeyExists_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	q.EXPECT().GetIdempotencyKeyExists(gomock.Any(), gomock.Any()).Return(false, errors.New("db error"))

	_, err := s.IdempotencyKeyExists(t.Context(), "key-abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check idempotency key")
}

func TestTransactionStore_Insert_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	txnID := uuid.New()
	inserted := sqlcgen.Transaction{ID: txnID}
	q.EXPECT().InsertTransaction(gomock.Any(), gomock.Any()).Return(inserted, nil)
	q.EXPECT().InsertTransactionEnrichment(gomock.Any(), txnID).Return(nil)

	p := InsertTransactionParams{
		AccountID:      uuid.New(),
		UserID:         uuid.MustParse(testUserID),
		IdempotencyKey: "key-1",
		Amount:         model.Money(100000),
		Currency:       "INR",
		Direction:      sqlcgen.TxnDirectionEnumDebit,
		Description:    "Swiggy order",
		TxnDate:        testFrom,
	}
	got, err := s.Insert(t.Context(), p)
	require.NoError(t, err)
	assert.Equal(t, txnID, got.ID)
}

func TestTransactionStore_Insert_InsertTransactionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	q.EXPECT().InsertTransaction(gomock.Any(), gomock.Any()).Return(sqlcgen.Transaction{}, errors.New("db error"))

	_, err := s.Insert(t.Context(), InsertTransactionParams{TxnDate: testFrom})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert transaction")
}

func TestTransactionStore_Insert_InsertEnrichmentError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	txnID := uuid.New()
	q.EXPECT().InsertTransaction(gomock.Any(), gomock.Any()).Return(sqlcgen.Transaction{ID: txnID}, nil)
	q.EXPECT().InsertTransactionEnrichment(gomock.Any(), txnID).Return(errors.New("enrichment error"))

	_, err := s.Insert(t.Context(), InsertTransactionParams{TxnDate: testFrom})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert enrichment")
}

func TestTransactionStore_List_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	want := []sqlcgen.VTransaction{{ID: uuid.New()}}
	q.EXPECT().ListTransactions(gomock.Any(), gomock.Any()).Return(want, nil)

	got, err := s.List(t.Context(), ListTransactionsParams{
		UserID: testUserID,
		Limit:  10,
	})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestTransactionStore_List_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	_, err := s.List(t.Context(), ListTransactionsParams{UserID: "bad-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestTransactionStore_List_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	q.EXPECT().ListTransactions(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	_, err := s.List(t.Context(), ListTransactionsParams{UserID: testUserID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list transactions")
}

func TestTransactionStore_GetSpendingByCategory_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	catID := uuid.New()
	rows := []sqlcgen.GetSpendingByCategoryRow{
		{
			CategoryID:   catID,
			CategorySlug: "food",
			CategoryName: "Food",
			Depth:        0,
			TotalAmount:  50000,
			TxnCount:     5,
		},
	}
	q.EXPECT().GetSpendingByCategory(gomock.Any(), gomock.Any()).Return(rows, nil)

	got, err := s.GetSpendingByCategory(t.Context(), testUserID, testFrom, testTo, nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, catID, got[0].CategoryID)
	assert.Equal(t, "food", got[0].CategorySlug)
	assert.Equal(t, int64(50000), got[0].TotalAmount)
	assert.Equal(t, int64(5), got[0].TxnCount)
}

func TestTransactionStore_GetSpendingByCategory_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	_, err := s.GetSpendingByCategory(t.Context(), "bad-uuid", testFrom, testTo, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestTransactionStore_GetSpendingByCategory_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	q.EXPECT().GetSpendingByCategory(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	_, err := s.GetSpendingByCategory(t.Context(), testUserID, testFrom, testTo, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get spending by category")
}

func TestTransactionStore_GetSpendingByCategory_WithAccountID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	accountID := uuid.New()
	accountIDStr := accountID.String()
	q.EXPECT().GetSpendingByCategory(gomock.Any(), gomock.Any()).Return([]sqlcgen.GetSpendingByCategoryRow{}, nil)

	got, err := s.GetSpendingByCategory(t.Context(), testUserID, testFrom, testTo, &accountIDStr)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestTransactionStore_List_DefaultLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	q.EXPECT().ListTransactions(gomock.Any(), gomock.Any()).Return(nil, nil)
	// Limit 0 should default to 20 internally.
	_, err := s.List(t.Context(), ListTransactionsParams{UserID: testUserID, Limit: 0})
	require.NoError(t, err)
}

func TestTransactionStore_List_WithDateFilters(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	q.EXPECT().ListTransactions(gomock.Any(), gomock.Any()).Return(nil, nil)
	from, to := testFrom, testTo
	_, err := s.List(t.Context(), ListTransactionsParams{
		UserID: testUserID,
		From:   &from,
		To:     &to,
	})
	require.NoError(t, err)
}

func TestTransactionStore_List_WithAccountID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	q.EXPECT().ListTransactions(gomock.Any(), gomock.Any()).Return(nil, nil)
	aid := uuid.New().String()
	_, err := s.List(t.Context(), ListTransactionsParams{UserID: testUserID, AccountID: &aid})
	require.NoError(t, err)
}

func TestTransactionStore_List_InvalidAccountID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	bad := "not-a-uuid"
	_, err := s.List(t.Context(), ListTransactionsParams{UserID: testUserID, AccountID: &bad})
	require.Error(t, err)
}

func TestTransactionStore_List_WithCategoryID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	q.EXPECT().ListTransactions(gomock.Any(), gomock.Any()).Return(nil, nil)
	cid := uuid.New().String()
	_, err := s.List(t.Context(), ListTransactionsParams{UserID: testUserID, CategoryID: &cid})
	require.NoError(t, err)
}

func TestTransactionStore_List_InvalidCategoryID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	bad := "not-a-uuid"
	_, err := s.List(t.Context(), ListTransactionsParams{UserID: testUserID, CategoryID: &bad})
	require.Error(t, err)
}

func TestTransactionStore_Insert_WithOriginalAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	txnID := uuid.New()
	q.EXPECT().InsertTransaction(gomock.Any(), gomock.Any()).Return(sqlcgen.Transaction{ID: txnID}, nil)
	q.EXPECT().InsertTransactionEnrichment(gomock.Any(), txnID).Return(nil)

	orig := model.Money(200)
	p := InsertTransactionParams{
		AccountID:      uuid.New(),
		UserID:         uuid.MustParse(testUserID),
		IdempotencyKey: "key-orig",
		Amount:         model.Money(100000),
		Currency:       "INR",
		Direction:      sqlcgen.TxnDirectionEnumDebit,
		Description:    "Forex purchase",
		TxnDate:        testFrom,
		OriginalAmount: &orig,
	}
	_, err := s.Insert(t.Context(), p)
	require.NoError(t, err)
}

func TestTransactionStore_GetSpendingByCategory_InvalidAccountID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newTransactionStoreForTest(q)

	bad := "not-a-uuid"
	_, err := s.GetSpendingByCategory(t.Context(), testUserID, testFrom, testTo, &bad)
	require.Error(t, err)
}
