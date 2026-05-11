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

func TestUserStore_GetByEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newUserStoreForTest(q)

	email := "a@b.com"
	want := sqlcgen.User{ID: uuid.New(), Email: &email, Name: "Alice"}
	q.EXPECT().GetUserByEmail(gomock.Any(), &email).Return(want, nil)

	got, err := s.GetByEmail(t.Context(), "a@b.com")
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}

func TestUserStore_GetByEmail_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newUserStoreForTest(q)

	q.EXPECT().GetUserByEmail(gomock.Any(), gomock.Any()).Return(sqlcgen.User{}, errors.New("not found"))

	_, err := s.GetByEmail(t.Context(), "missing@b.com")
	require.Error(t, err)
}

func TestUserStore_GetByID_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newUserStoreForTest(q)

	_, err := s.GetByID(t.Context(), "not-a-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestUserStore_GetByID_ValidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newUserStoreForTest(q)

	uid := uuid.New()
	want := sqlcgen.User{ID: uid, Name: "Bob"}
	q.EXPECT().GetUserByID(gomock.Any(), uid).Return(want, nil)

	got, err := s.GetByID(t.Context(), uid.String())
	require.NoError(t, err)
	assert.Equal(t, uid, got.ID)
}

func TestUserStore_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newUserStoreForTest(q)

	users := []sqlcgen.User{
		{ID: uuid.New(), Name: "Alice"},
		{ID: uuid.New(), Name: "Bob"},
	}
	q.EXPECT().ListUsers(gomock.Any()).Return(users, nil)

	got, err := s.List(t.Context())
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestUserStore_List_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newUserStoreForTest(q)

	q.EXPECT().ListUsers(gomock.Any()).Return(nil, errors.New("db error"))

	_, err := s.List(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list users")
}

func TestUserStore_GetByID_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newUserStoreForTest(q)

	uid := uuid.New()
	q.EXPECT().GetUserByID(gomock.Any(), uid).Return(sqlcgen.User{}, errors.New("db error"))

	_, err := s.GetByID(t.Context(), uid.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get user by id")
}
