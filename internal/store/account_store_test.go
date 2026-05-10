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

func TestAccountStore_ListByUser_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	uid := uuid.MustParse(testUserID)
	want := []sqlcgen.Account{{ID: uuid.New(), Name: "Savings"}}
	q.EXPECT().ListAccountsByUser(gomock.Any(), uid).Return(want, nil)

	got, err := s.ListByUser(t.Context(), testUserID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAccountStore_ListByUser_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	_, err := s.ListByUser(t.Context(), "not-a-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestAccountStore_ListByUser_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	uid := uuid.MustParse(testUserID)
	q.EXPECT().ListAccountsByUser(gomock.Any(), uid).Return(nil, errors.New("db error"))

	_, err := s.ListByUser(t.Context(), testUserID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list accounts by user")
}

func TestAccountStore_GetByID_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	aid := uuid.New()
	want := sqlcgen.Account{ID: aid, Name: "Salary"}
	q.EXPECT().GetAccountByID(gomock.Any(), aid).Return(want, nil)

	got, err := s.GetByID(t.Context(), aid.String())
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}

func TestAccountStore_GetByID_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	_, err := s.GetByID(t.Context(), "bad-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestAccountStore_GetByID_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	aid := uuid.New()
	q.EXPECT().GetAccountByID(gomock.Any(), aid).Return(sqlcgen.Account{}, errors.New("not found"))

	_, err := s.GetByID(t.Context(), aid.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get account by id")
}

func TestAccountStore_Create_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	created := sqlcgen.Account{ID: uuid.New(), Name: "Current"}
	q.EXPECT().CreateAccount(gomock.Any(), gomock.Any()).Return(created, nil)
	q.EXPECT().AddAccountMember(gomock.Any(), gomock.Any()).Return(nil)

	p := CreateAccountParams{
		Institution: "HDFC",
		Name:        "Current",
		AccountType: sqlcgen.AccountTypeEnumBankCurrent,
		Currency:    "INR",
		IsActive:    true,
	}
	got, err := s.Create(t.Context(), p, testUserID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestAccountStore_Create_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	_, err := s.Create(t.Context(), CreateAccountParams{}, "bad-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestAccountStore_Create_CreateAccountError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	q.EXPECT().CreateAccount(gomock.Any(), gomock.Any()).Return(sqlcgen.Account{}, errors.New("insert failed"))

	_, err := s.Create(t.Context(), CreateAccountParams{}, testUserID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create account")
}

func TestAccountStore_Create_AddMemberError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	q.EXPECT().CreateAccount(gomock.Any(), gomock.Any()).Return(sqlcgen.Account{ID: uuid.New()}, nil)
	q.EXPECT().AddAccountMember(gomock.Any(), gomock.Any()).Return(errors.New("member insert failed"))

	_, err := s.Create(t.Context(), CreateAccountParams{}, testUserID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "add account member")
}

func TestAccountStore_AddMember_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	aid := uuid.New()
	uid := uuid.MustParse(testUserID)
	q.EXPECT().AddAccountMember(gomock.Any(), sqlcgen.AddAccountMemberParams{
		AccountID: aid,
		UserID:    uid,
		Role:      sqlcgen.MemberRoleEnumJoint,
	}).Return(nil)

	err := s.AddMember(t.Context(), aid.String(), testUserID, sqlcgen.MemberRoleEnumJoint)
	require.NoError(t, err)
}

func TestAccountStore_AddMember_InvalidAccountID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	err := s.AddMember(t.Context(), "bad-aid", testUserID, sqlcgen.MemberRoleEnumJoint)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestAccountStore_AddMember_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	aid := uuid.New()
	err := s.AddMember(t.Context(), aid.String(), "bad-uid", sqlcgen.MemberRoleEnumJoint)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestAccountStore_GetDetails_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	aid := uuid.New()
	want := sqlcgen.AccountDetail{AccountID: aid}
	q.EXPECT().GetAccountDetails(gomock.Any(), aid).Return(want, nil)

	got, err := s.GetDetails(t.Context(), aid)
	require.NoError(t, err)
	assert.Equal(t, want.AccountID, got.AccountID)
}

func TestAccountStore_GetDetails_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newAccountStoreForTest(q)

	aid := uuid.New()
	q.EXPECT().GetAccountDetails(gomock.Any(), aid).Return(sqlcgen.AccountDetail{}, errors.New("not found"))

	_, err := s.GetDetails(t.Context(), aid)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get account details")
}
