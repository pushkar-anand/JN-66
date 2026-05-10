package store

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

func TestLabelStore_List_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	want := []sqlcgen.Label{{ID: uuid.New(), Name: "groceries"}}
	q.EXPECT().ListLabels(gomock.Any(), gomock.Any()).Return(want, nil)

	got, err := s.List(t.Context(), testUserID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLabelStore_List_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	_, err := s.List(t.Context(), "bad-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestLabelStore_List_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	q.EXPECT().ListLabels(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	_, err := s.List(t.Context(), testUserID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list labels")
}

func TestLabelStore_AddToTransaction_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	txnID := uuid.New()
	labelID := uuid.New()
	q.EXPECT().AddTransactionLabel(gomock.Any(), gomock.Any()).Return(nil)

	err := s.AddToTransaction(t.Context(), txnID.String(), labelID.String())
	require.NoError(t, err)
}

func TestLabelStore_AddToTransaction_InvalidTxnID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	labelID := uuid.New()
	err := s.AddToTransaction(t.Context(), "bad-txn", labelID.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestLabelStore_AddToTransaction_InvalidLabelID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	txnID := uuid.New()
	err := s.AddToTransaction(t.Context(), txnID.String(), "bad-label")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestLabelStore_AddToTransaction_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	txnID := uuid.New()
	labelID := uuid.New()
	q.EXPECT().AddTransactionLabel(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	err := s.AddToTransaction(t.Context(), txnID.String(), labelID.String())
	require.Error(t, err)
}

func TestLabelStore_RemoveFromTransaction_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	txnID := uuid.New()
	labelID := uuid.New()
	q.EXPECT().RemoveTransactionLabel(gomock.Any(), gomock.Any()).Return(nil)

	err := s.RemoveFromTransaction(t.Context(), txnID.String(), labelID.String())
	require.NoError(t, err)
}

func TestLabelStore_RemoveFromTransaction_InvalidTxnID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	labelID := uuid.New()
	err := s.RemoveFromTransaction(t.Context(), "bad-txn", labelID.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestLabelStore_RemoveFromTransaction_InvalidLabelID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	txnID := uuid.New()
	err := s.RemoveFromTransaction(t.Context(), txnID.String(), "bad-label")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestLabelStore_RemoveFromTransaction_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	txnID := uuid.New()
	labelID := uuid.New()
	q.EXPECT().RemoveTransactionLabel(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	err := s.RemoveFromTransaction(t.Context(), txnID.String(), labelID.String())
	require.Error(t, err)
}

func TestLabelStore_ListForTransaction_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	txnID := uuid.New()
	want := []sqlcgen.Label{{ID: uuid.New(), Name: "groceries"}}
	q.EXPECT().ListTransactionLabels(gomock.Any(), txnID).Return(want, nil)

	got, err := s.ListForTransaction(t.Context(), txnID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLabelStore_ListForTransaction_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newLabelStoreForTest(q)

	txnID := uuid.New()
	q.EXPECT().ListTransactionLabels(gomock.Any(), txnID).Return(nil, errors.New("db error"))

	_, err := s.ListForTransaction(t.Context(), txnID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list transaction labels")
}
