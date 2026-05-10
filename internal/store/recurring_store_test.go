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

func TestRecurringStore_List_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newRecurringStoreForTest(q)

	uid := uuid.MustParse(testUserID)
	want := []sqlcgen.RecurringPayment{{ID: uuid.New(), Name: "Netflix"}}
	q.EXPECT().ListRecurringPayments(gomock.Any(), uid).Return(want, nil)

	got, err := s.List(t.Context(), testUserID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestRecurringStore_List_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newRecurringStoreForTest(q)

	_, err := s.List(t.Context(), "bad-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestRecurringStore_List_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newRecurringStoreForTest(q)

	uid := uuid.MustParse(testUserID)
	q.EXPECT().ListRecurringPayments(gomock.Any(), uid).Return(nil, errors.New("db error"))

	_, err := s.List(t.Context(), testUserID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list recurring payments")
}

func TestRecurringStore_Create_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newRecurringStoreForTest(q)

	params := sqlcgen.CreateRecurringPaymentParams{
		UserID:    uuid.MustParse(testUserID),
		AccountID: uuid.New(),
		Name:      "Spotify",
		Frequency: sqlcgen.FrequencyEnumMonthly,
	}
	want := sqlcgen.RecurringPayment{ID: uuid.New(), Name: "Spotify"}
	q.EXPECT().CreateRecurringPayment(gomock.Any(), params).Return(want, nil)

	got, err := s.Create(t.Context(), params)
	require.NoError(t, err)
	assert.Equal(t, want.Name, got.Name)
}

func TestRecurringStore_Create_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newRecurringStoreForTest(q)

	params := sqlcgen.CreateRecurringPaymentParams{Name: "Spotify"}
	q.EXPECT().CreateRecurringPayment(gomock.Any(), params).Return(sqlcgen.RecurringPayment{}, errors.New("db error"))

	_, err := s.Create(t.Context(), params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create recurring payment")
}

func TestRecurringStore_Deactivate_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newRecurringStoreForTest(q)

	rid := uuid.New()
	q.EXPECT().DeactivateRecurringPayment(gomock.Any(), rid).Return(nil)

	err := s.Deactivate(t.Context(), rid.String())
	require.NoError(t, err)
}

func TestRecurringStore_Deactivate_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newRecurringStoreForTest(q)

	err := s.Deactivate(t.Context(), "bad-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestRecurringStore_Deactivate_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newRecurringStoreForTest(q)

	rid := uuid.New()
	q.EXPECT().DeactivateRecurringPayment(gomock.Any(), rid).Return(errors.New("db error"))

	err := s.Deactivate(t.Context(), rid.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deactivate recurring payment")
}
