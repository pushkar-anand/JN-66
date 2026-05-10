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

func TestMemoryStore_Save_WithUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	uid := testUserID
	want := sqlcgen.AgentMemory{ID: uuid.New(), Content: "spend less on food"}
	q.EXPECT().CreateMemory(gomock.Any(), gomock.Any()).Return(want, nil)

	got, err := s.Save(t.Context(), &uid, "spend less on food", sqlcgen.MemoryTypeEnumPreference, []string{"food"})
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}

func TestMemoryStore_Save_WithUserID_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	uid := testUserID
	q.EXPECT().CreateMemory(gomock.Any(), gomock.Any()).Return(sqlcgen.AgentMemory{}, errors.New("db error"))

	_, err := s.Save(t.Context(), &uid, "spend less on food", sqlcgen.MemoryTypeEnumPreference, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save memory")
}

func TestMemoryStore_Save_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	bad := "not-a-uuid"
	_, err := s.Save(t.Context(), &bad, "content", sqlcgen.MemoryTypeEnumGeneral, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestMemoryStore_Save_NilUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	want := sqlcgen.AgentMemory{ID: uuid.New(), Content: "household tip"}
	q.EXPECT().CreateMemory(gomock.Any(), gomock.Any()).Return(want, nil)

	got, err := s.Save(t.Context(), nil, "household tip", sqlcgen.MemoryTypeEnumGeneral, nil)
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}

func TestMemoryStore_Recall_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	want := []sqlcgen.AgentMemory{{ID: uuid.New(), Content: "eat out less"}}
	q.EXPECT().RecallMemoriesByTags(gomock.Any(), gomock.Any()).Return(want, nil)

	got, err := s.Recall(t.Context(), testUserID, []string{"food"}, 10)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMemoryStore_Recall_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	q.EXPECT().RecallMemoriesByTags(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	_, err := s.Recall(t.Context(), testUserID, nil, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recall memories")
}

func TestMemoryStore_Recall_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	_, err := s.Recall(t.Context(), "bad-uuid", nil, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestMemoryStore_List_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	want := []sqlcgen.AgentMemory{
		{ID: uuid.New(), Content: "mem1"},
		{ID: uuid.New(), Content: "mem2"},
	}
	q.EXPECT().ListMemories(gomock.Any(), gomock.Any()).Return(want, nil)

	got, err := s.List(t.Context(), testUserID, 20)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestMemoryStore_List_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	q.EXPECT().ListMemories(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	_, err := s.List(t.Context(), testUserID, 20)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list memories")
}

func TestMemoryStore_List_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	_, err := s.List(t.Context(), "bad-uuid", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}
