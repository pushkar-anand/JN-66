package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func savedMemory(content string) *sqlcgen.AgentMemory {
	return &sqlcgen.AgentMemory{ID: uuid.New(), Content: content, MemoryType: sqlcgen.MemoryTypeEnumGeneral}
}

func TestRememberFact_UsesBoundUserWhenNoOverride(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)

	var gotUserID string
	q.EXPECT().Save(gomock.Any(), gomock.Any(), "some fact", sqlcgen.MemoryTypeEnumGeneral, gomock.Any()).
		DoAndReturn(func(_ context.Context, uid *string, content string, _ sqlcgen.MemoryTypeEnum, _ []string) (*sqlcgen.AgentMemory, error) {
			if uid != nil {
				gotUserID = *uid
			}
			return savedMemory(content), nil
		})

	_, err := NewRememberFact(boundUser, q).Execute(t.Context(), "", `{"content":"some fact"}`)
	require.NoError(t, err)
	assert.Equal(t, boundUser, gotUserID)
}

func TestRememberFact_ExplicitUserIDOverrides(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)

	var gotUserID string
	q.EXPECT().Save(gomock.Any(), gomock.Any(), "fact", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, uid *string, content string, _ sqlcgen.MemoryTypeEnum, _ []string) (*sqlcgen.AgentMemory, error) {
			if uid != nil {
				gotUserID = *uid
			}
			return savedMemory(content), nil
		})

	_, err := NewRememberFact(boundUser, q).Execute(t.Context(), "", `{"content":"fact","user_id":"other-user"}`)
	require.NoError(t, err)
	assert.Equal(t, "other-user", gotUserID)
}

func TestRememberFact_MemoryTypeTaggingHint(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)
	q.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any(), sqlcgen.MemoryTypeEnumTaggingHint, gomock.Any()).
		Return(savedMemory("hint"), nil)

	_, err := NewRememberFact(boundUser, q).Execute(t.Context(), "", `{"content":"hint","memory_type":"tagging_hint"}`)
	require.NoError(t, err)
}

func TestRememberFact_UnknownMemoryTypeDefaultsToGeneral(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)
	q.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any(), sqlcgen.MemoryTypeEnumGeneral, gomock.Any()).
		Return(savedMemory("thing"), nil)

	_, err := NewRememberFact(boundUser, q).Execute(t.Context(), "", `{"content":"thing","memory_type":"nonexistent"}`)
	require.NoError(t, err)
}

func TestRememberFact_TagsPassedThrough(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)
	q.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), []string{"food", "zomato"}).
		Return(savedMemory("delivery"), nil)

	_, err := NewRememberFact(boundUser, q).Execute(t.Context(), "", `{"content":"delivery","tags":["food","zomato"]}`)
	require.NoError(t, err)
}

func TestRememberFact_ResultContainsID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)

	mem := savedMemory("fact")
	q.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mem, nil)

	got, err := NewRememberFact(boundUser, q).Execute(t.Context(), "", `{"content":"fact"}`)
	require.NoError(t, err)
	assert.Contains(t, got, mem.ID.String())
}

func TestRememberFact_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)
	q.EXPECT().Save(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("db down"))

	_, err := NewRememberFact(boundUser, q).Execute(t.Context(), "", `{"content":"fact"}`)
	require.Error(t, err)
}

func TestRememberFact_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockmemoryQuerier(ctrl)

	_, err := NewRememberFact(boundUser, q).Execute(t.Context(), "", `{bad`)
	require.Error(t, err)
}
