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

const (
	testUserID    = "11111111-1111-1111-1111-111111111111"
	testSessionID = "22222222-2222-2222-2222-222222222222"
)

func TestConversationStore_GetOrCreateSession_EmptySessionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	want := sqlcgen.ConversationSession{ID: uuid.MustParse(testSessionID)}
	q.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(want, nil)

	got, err := s.GetOrCreateSession(t.Context(), testUserID, "", sqlcgen.ChannelEnumCli)
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}

func TestConversationStore_GetOrCreateSession_InvalidSessionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	want := sqlcgen.ConversationSession{ID: uuid.MustParse(testSessionID)}
	q.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(want, nil)

	// Malformed session ID — falls through to create.
	got, err := s.GetOrCreateSession(t.Context(), testUserID, "not-a-uuid", sqlcgen.ChannelEnumCli)
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}

func TestConversationStore_GetOrCreateSession_ExistingSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	sid := uuid.MustParse(testSessionID)
	want := sqlcgen.ConversationSession{ID: sid}
	q.EXPECT().GetSession(gomock.Any(), sid).Return(want, nil)
	q.EXPECT().TouchSession(gomock.Any(), sid).Return(nil)

	got, err := s.GetOrCreateSession(t.Context(), testUserID, testSessionID, sqlcgen.ChannelEnumCli)
	require.NoError(t, err)
	assert.Equal(t, sid, got.ID)
}

func TestConversationStore_GetOrCreateSession_SessionNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	sid := uuid.MustParse(testSessionID)
	want := sqlcgen.ConversationSession{ID: uuid.New()}
	q.EXPECT().GetSession(gomock.Any(), sid).Return(sqlcgen.ConversationSession{}, errors.New("not found"))
	q.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(want, nil)

	got, err := s.GetOrCreateSession(t.Context(), testUserID, testSessionID, sqlcgen.ChannelEnumCli)
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}

func TestConversationStore_GetOrCreateSession_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	_, err := s.GetOrCreateSession(t.Context(), "not-a-uuid", "", sqlcgen.ChannelEnumCli)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid uuid")
}

func TestConversationStore_SaveMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	sid := uuid.MustParse(testSessionID)
	q.EXPECT().SaveMessage(gomock.Any(), gomock.Any()).Return(sqlcgen.ConversationMessage{}, nil)

	err := s.SaveMessage(t.Context(), sid, sqlcgen.MsgRoleEnumUser, "hello")
	require.NoError(t, err)
}

func TestConversationStore_SaveMessage_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	sid := uuid.MustParse(testSessionID)
	q.EXPECT().SaveMessage(gomock.Any(), gomock.Any()).Return(sqlcgen.ConversationMessage{}, errors.New("db error"))

	err := s.SaveMessage(t.Context(), sid, sqlcgen.MsgRoleEnumUser, "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save message")
}

func TestConversationStore_RecentMessages_ReversedOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	sid := uuid.MustParse(testSessionID)
	// The query returns newest-first; the store reverses to oldest-first.
	q.EXPECT().ListRecentMessages(gomock.Any(), gomock.Any()).Return([]sqlcgen.ConversationMessage{
		{Content: "newest"},
		{Content: "middle"},
		{Content: "oldest"},
	}, nil)

	msgs, err := s.RecentMessages(t.Context(), sid, 3)
	require.NoError(t, err)
	require.Len(t, msgs, 3)
	assert.Equal(t, "oldest", msgs[0].Content)
	assert.Equal(t, "newest", msgs[2].Content)
}

func TestConversationStore_RecentMessages_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	sid := uuid.MustParse(testSessionID)
	q.EXPECT().ListRecentMessages(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	_, err := s.RecentMessages(t.Context(), sid, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list recent messages")
}

func TestConversationStore_GetOrCreateSession_TouchSessionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	sid := uuid.MustParse(testSessionID)
	sess := sqlcgen.ConversationSession{ID: sid}
	q.EXPECT().GetSession(gomock.Any(), sid).Return(sess, nil)
	q.EXPECT().TouchSession(gomock.Any(), sid).Return(errors.New("db error"))

	_, err := s.GetOrCreateSession(t.Context(), testUserID, testSessionID, sqlcgen.ChannelEnumCli)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "touch session")
}

func TestConversationStore_GetOrCreateSession_CreateSessionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	q := NewMockQuerier(ctrl)
	s := newConversationStoreForTest(q)

	q.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(sqlcgen.ConversationSession{}, errors.New("db error"))

	_, err := s.GetOrCreateSession(t.Context(), testUserID, "", sqlcgen.ChannelEnumCli)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create session")
}
