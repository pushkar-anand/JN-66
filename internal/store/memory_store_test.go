package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// mockEmbedder is a simple test double for Embedder.
type mockEmbedder struct {
	vec []float32
	err error
}

func (m *mockEmbedder) EmbedText(_ context.Context, _ string) ([]float32, error) {
	return m.vec, m.err
}

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

func TestMemoryStore_Save_WithEmbedder(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	emb := &mockEmbedder{vec: make([]float32, 768)}
	s := &MemoryStore{q: q, embedder: emb}

	uid := testUserID
	want := sqlcgen.AgentMemory{ID: uuid.New(), Content: "food budget"}
	q.EXPECT().CreateMemory(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p sqlcgen.CreateMemoryParams) (sqlcgen.AgentMemory, error) {
			assert.NotNil(t, p.Embedding)
			v := pgvector.NewVector(make([]float32, 768))
			assert.Equal(t, v, *p.Embedding)
			return want, nil
		},
	)

	got, err := s.Save(t.Context(), &uid, "food budget", sqlcgen.MemoryTypeEnumPreference, nil)
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}

func TestMemoryStore_Save_EmbedFailContinues(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	emb := &mockEmbedder{err: errors.New("embed unavailable")}
	s := &MemoryStore{q: q, embedder: emb}

	uid := testUserID
	want := sqlcgen.AgentMemory{ID: uuid.New(), Content: "food budget"}
	q.EXPECT().CreateMemory(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p sqlcgen.CreateMemoryParams) (sqlcgen.AgentMemory, error) {
			assert.Nil(t, p.Embedding)
			return want, nil
		},
	)

	got, err := s.Save(t.Context(), &uid, "food budget", sqlcgen.MemoryTypeEnumPreference, nil)
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}

func TestMemoryStore_Recall_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	want := []sqlcgen.AgentMemory{{ID: uuid.New(), Content: "eat out less"}}
	q.EXPECT().RecallMemoriesByTags(gomock.Any(), gomock.Any()).Return(want, nil)

	got, err := s.Recall(t.Context(), testUserID, "food spending", 10)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMemoryStore_Recall_VectorPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	emb := &mockEmbedder{vec: make([]float32, 768)}
	s := &MemoryStore{q: q, embedder: emb}

	want := []sqlcgen.AgentMemory{{ID: uuid.New(), Content: "eat out less"}}
	q.EXPECT().RecallMemoriesByEmbedding(gomock.Any(), gomock.Cond(func(p sqlcgen.RecallMemoriesByEmbeddingParams) bool {
		return p.MaxDistance > 0 && p.MaxDistance < 1.0
	})).Return(want, nil)

	got, err := s.Recall(t.Context(), testUserID, "food spending", 10)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMemoryStore_Recall_FallsBackWhenEmbedFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	emb := &mockEmbedder{err: errors.New("embed unavailable")}
	s := &MemoryStore{q: q, embedder: emb}

	want := []sqlcgen.AgentMemory{{ID: uuid.New(), Content: "eat out less"}}
	q.EXPECT().RecallMemoriesByTags(gomock.Any(), gomock.Any()).Return(want, nil)

	got, err := s.Recall(t.Context(), testUserID, "food spending", 10)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMemoryStore_Recall_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	q.EXPECT().RecallMemoriesByTags(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	_, err := s.Recall(t.Context(), testUserID, "food", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recall memories")
}

func TestMemoryStore_Recall_InvalidUUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newMemoryStoreForTest(q)

	_, err := s.Recall(t.Context(), "bad-uuid", "query", 5)
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
