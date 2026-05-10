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

func TestCategoryStore_List_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newCategoryStoreForTest(q)

	want := []sqlcgen.Category{
		{ID: uuid.New(), Slug: "food", Name: "Food"},
		{ID: uuid.New(), Slug: "travel", Name: "Travel"},
	}
	q.EXPECT().ListCategories(gomock.Any()).Return(want, nil)

	got, err := s.List(t.Context())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestCategoryStore_List_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newCategoryStoreForTest(q)

	q.EXPECT().ListCategories(gomock.Any()).Return(nil, errors.New("db error"))

	_, err := s.List(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list categories")
}

func TestCategoryStore_GetBySlug_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newCategoryStoreForTest(q)

	want := sqlcgen.Category{ID: uuid.New(), Slug: "food", Name: "Food"}
	q.EXPECT().GetCategoryBySlug(gomock.Any(), "food").Return(want, nil)

	got, err := s.GetBySlug(t.Context(), "food")
	require.NoError(t, err)
	assert.Equal(t, want.Slug, got.Slug)
}

func TestCategoryStore_GetBySlug_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newCategoryStoreForTest(q)

	q.EXPECT().GetCategoryBySlug(gomock.Any(), "unknown").Return(sqlcgen.Category{}, errors.New("not found"))

	_, err := s.GetBySlug(t.Context(), "unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get category by slug")
}
