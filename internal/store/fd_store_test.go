package store

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
)

func newFDStoreForTest(q sqlcgen.Querier) *FDStore {
	return &FDStore{q: q}
}

func stubFixedDeposit(id uuid.UUID) sqlcgen.FixedDeposit {
	return sqlcgen.FixedDeposit{
		ID:              id,
		UserID:          uuid.MustParse(testUserID),
		PrincipalAmount: 10000000,
		InterestRateBps: 750,
		TenureMonths:    12,
		StartDate:       pgtype.Date{Time: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		MaturityDate:    pgtype.Date{Time: time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		InterestPayout:  sqlcgen.FdPayoutEnumCumulative,
		AutoRenewalType: sqlcgen.FdRenewalTypeEnumNone,
		Status:          sqlcgen.FdStatusEnumActive,
	}
}

// fdExternalID

func TestFDExternalID_WithBankNumber(t *testing.T) {
	n := "FD123456"
	assert.Equal(t, "FD123456", fdExternalID(&n))
}

func TestFDExternalID_NilUsesUUID(t *testing.T) {
	id := fdExternalID(nil)
	_, err := uuid.Parse(id)
	assert.NoError(t, err, "should be a valid UUID")
}

func TestFDExternalID_EmptyStringUsesUUID(t *testing.T) {
	empty := ""
	id := fdExternalID(&empty)
	_, err := uuid.Parse(id)
	assert.NoError(t, err, "should be a valid UUID for empty string")
}

// Get

func TestFDStore_Get_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	fdID := uuid.New()
	uid := uuid.MustParse(testUserID)
	want := stubFixedDeposit(fdID)
	q.EXPECT().GetFixedDeposit(gomock.Any(), sqlcgen.GetFixedDepositParams{ID: fdID, UserID: uid}).Return(want, nil)

	got, err := s.Get(t.Context(), fdID.String(), testUserID)
	require.NoError(t, err)
	assert.Equal(t, &want, got)
}

func TestFDStore_Get_InvalidFDID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	_, err := s.Get(t.Context(), "not-a-uuid", testUserID)
	require.Error(t, err)
}

func TestFDStore_Get_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	_, err := s.Get(t.Context(), uuid.NewString(), "bad-user")
	require.Error(t, err)
}

func TestFDStore_Get_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	q.EXPECT().GetFixedDeposit(gomock.Any(), gomock.Any()).Return(sqlcgen.FixedDeposit{}, errors.New("db error"))

	_, err := s.Get(t.Context(), uuid.NewString(), testUserID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get fixed deposit")
}

// ListByUser

func TestFDStore_ListByUser_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	uid := uuid.MustParse(testUserID)
	active := sqlcgen.FdStatusEnumActive
	want := []sqlcgen.FixedDeposit{stubFixedDeposit(uuid.New())}
	q.EXPECT().ListFixedDeposits(gomock.Any(), sqlcgen.ListFixedDepositsParams{
		UserID: uid,
		Status: &active,
	}).Return(want, nil)

	got, err := s.ListByUser(t.Context(), ListFDsParams{UserID: testUserID, Status: &active})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestFDStore_ListByUser_NoFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	q.EXPECT().ListFixedDeposits(gomock.Any(), gomock.Any()).Return(nil, nil)

	got, err := s.ListByUser(t.Context(), ListFDsParams{UserID: testUserID})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestFDStore_ListByUser_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	_, err := s.ListByUser(t.Context(), ListFDsParams{UserID: "bad"})
	require.Error(t, err)
}

func TestFDStore_ListByUser_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	q.EXPECT().ListFixedDeposits(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	_, err := s.ListByUser(t.Context(), ListFDsParams{UserID: testUserID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list fixed deposits")
}

func TestFDStore_ListByUser_MaturingBeforeFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	cutoff := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	q.EXPECT().ListFixedDeposits(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, p sqlcgen.ListFixedDepositsParams) ([]sqlcgen.FixedDeposit, error) {
			assert.True(t, p.MaturingBefore.Valid)
			assert.Equal(t, cutoff, p.MaturingBefore.Time)
			return nil, nil
		})

	_, err := s.ListByUser(t.Context(), ListFDsParams{UserID: testUserID, MaturingBefore: &cutoff})
	require.NoError(t, err)
}

// UpdateStatus

func TestFDStore_UpdateStatus_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	fdID := uuid.New()
	uid := uuid.MustParse(testUserID)
	want := stubFixedDeposit(fdID)
	want.Status = sqlcgen.FdStatusEnumMatured
	want.ActualPayoutAmount = model.FromRupees(107500)

	q.EXPECT().UpdateFixedDepositStatus(gomock.Any(), sqlcgen.UpdateFixedDepositStatusParams{
		Status:             sqlcgen.FdStatusEnumMatured,
		ActualPayoutAmount: model.FromRupees(107500),
		ID:                 fdID,
		UserID:             uid,
	}).Return(want, nil)

	got, err := s.UpdateStatus(t.Context(), UpdateStatusParams{
		UserID:             testUserID,
		FDID:               fdID.String(),
		Status:             sqlcgen.FdStatusEnumMatured,
		ActualPayoutAmount: model.FromRupees(107500),
	})
	require.NoError(t, err)
	assert.Equal(t, sqlcgen.FdStatusEnumMatured, got.Status)
	assert.Equal(t, model.FromRupees(107500), got.ActualPayoutAmount)
}

func TestFDStore_UpdateStatus_InvalidFDID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	_, err := s.UpdateStatus(t.Context(), UpdateStatusParams{
		UserID: testUserID,
		FDID:   "not-a-uuid",
		Status: sqlcgen.FdStatusEnumMatured,
	})
	require.Error(t, err)
}

func TestFDStore_UpdateStatus_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	_, err := s.UpdateStatus(t.Context(), UpdateStatusParams{
		UserID: "bad",
		FDID:   uuid.NewString(),
		Status: sqlcgen.FdStatusEnumMatured,
	})
	require.Error(t, err)
}

func TestFDStore_UpdateStatus_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newFDStoreForTest(q)

	q.EXPECT().UpdateFixedDepositStatus(gomock.Any(), gomock.Any()).Return(sqlcgen.FixedDeposit{}, errors.New("db error"))

	_, err := s.UpdateStatus(t.Context(), UpdateStatusParams{
		UserID: testUserID,
		FDID:   uuid.NewString(),
		Status: sqlcgen.FdStatusEnumMatured,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update fixed deposit status")
}
