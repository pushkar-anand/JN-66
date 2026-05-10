package tools

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRecallFacts_UsesBoundUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)
	q.EXPECT().Recall(gomock.Any(), boundUser, []string{"food"}, int32(10)).Return(nil, nil)

	got, err := NewRecallFacts(boundUser, q).Execute(t.Context(), "", `{"tags":["food"]}`)
	require.NoError(t, err)
	assert.Equal(t, "No memories found matching those tags.", got)
}

func TestRecallFacts_NoMemories(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)
	q.EXPECT().Recall(gomock.Any(), boundUser, gomock.Any(), int32(10)).Return(nil, nil)

	got, err := NewRecallFacts(boundUser, q).Execute(t.Context(), "", `{"tags":["xyz"]}`)
	require.NoError(t, err)
	assert.Equal(t, "No memories found matching those tags.", got)
}

func TestRecallFacts_FormatsMemories(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)

	memories := []sqlcgen.AgentMemory{
		{ID: uuid.New(), MemoryType: sqlcgen.MemoryTypeEnumGeneral, Content: "Zomato is food delivery"},
		{ID: uuid.New(), MemoryType: sqlcgen.MemoryTypeEnumTaggingHint, Content: "Tag Uber as transport"},
	}
	q.EXPECT().Recall(gomock.Any(), boundUser, gomock.Any(), int32(10)).Return(memories, nil)

	got, err := NewRecallFacts(boundUser, q).Execute(t.Context(), "", `{"tags":["food"]}`)
	require.NoError(t, err)
	assert.Contains(t, got, "Zomato is food delivery")
	assert.Contains(t, got, "Tag Uber as transport")
	assert.Contains(t, got, "[general]")
	assert.Contains(t, got, "[tagging_hint]")
}

func TestRecallFacts_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)
	q.EXPECT().Recall(gomock.Any(), boundUser, gomock.Any(), int32(10)).Return(nil, errors.New("db down"))

	_, err := NewRecallFacts(boundUser, q).Execute(t.Context(), "", `{"tags":["food"]}`)
	require.Error(t, err)
}

func TestRecallFacts_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)

	_, err := NewRecallFacts(boundUser, q).Execute(t.Context(), "", `{bad`)
	require.Error(t, err)
}

func TestRecallFacts_Definition(t *testing.T) {
	q := NewMockmemoryQuerier(gomock.NewController(t))
	def := NewRecallFacts(boundUser, q).Definition()
	assert.Equal(t, "recall_facts", def.Name)
}
