package agent

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/pushkaranand/finagent/config"
	"github.com/pushkaranand/finagent/internal/channel"
	"github.com/pushkaranand/finagent/internal/llm"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

const testUserID = "11111111-1111-1111-1111-111111111111"

func testSession() *sqlcgen.ConversationSession {
	return &sqlcgen.ConversationSession{
		ID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		UserID: uuid.MustParse(testUserID),
	}
}

func newTestAgent(
	t *testing.T,
	llmP chatProvider,
	conv convStore,
	mem memStore,
	users userStore,
	reg toolRegistry,
) *Agent {
	t.Helper()
	return New(llmP, conv, mem, users, reg, NewRouter(config.RoutingConfig{
		ChatModel:      "chat-model",
		AnalysisModel:  "analysis-model",
		SummarizeModel: "summarize-model",
	}), false)
}

func TestHandleMessage_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLLM := NewMockchatProvider(ctrl)
	mockConv := NewMockconvStore(ctrl)
	mockMem := NewMockmemStore(ctrl)
	mockUsers := NewMockuserStore(ctrl)
	mockReg := NewMocktoolRegistry(ctrl)

	sess := testSession()
	msg := channel.Message{UserID: testUserID, SessionID: "", Text: "What accounts do I have?"}

	mockUsers.EXPECT().GetByID(gomock.Any(), testUserID).Return(&sqlcgen.User{Name: "Alice"}, nil)
	mockConv.EXPECT().GetOrCreateSession(gomock.Any(), testUserID, "", sqlcgen.ChannelEnumCli).Return(sess, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumUser, msg.Text).Return(nil)
	mockConv.EXPECT().RecentMessages(gomock.Any(), sess.ID, int32(20)).Return(nil, nil)
	mockMem.EXPECT().Recall(gomock.Any(), testUserID, gomock.Any(), int32(5)).Return(nil, nil)
	mockReg.EXPECT().Definitions().Return(nil)
	mockLLM.EXPECT().Chat(gomock.Any(), gomock.Any()).Return(llm.ChatResponse{
		Message:    llm.Message{Role: llm.RoleAssistant, Content: "You have 3 accounts."},
		StopReason: "stop",
	}, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumAssistant, "You have 3 accounts.").Return(nil)

	ag := newTestAgent(t, mockLLM, mockConv, mockMem, mockUsers, mockReg)
	resp, err := ag.HandleMessage(t.Context(), msg)
	require.NoError(t, err)
	assert.Equal(t, "You have 3 accounts.", resp.Text)
	assert.True(t, resp.Markdown)
}

func TestHandleMessage_SessionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLLM := NewMockchatProvider(ctrl)
	mockConv := NewMockconvStore(ctrl)
	mockMem := NewMockmemStore(ctrl)
	mockUsers := NewMockuserStore(ctrl)
	mockReg := NewMocktoolRegistry(ctrl)

	mockUsers.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found"))
	mockConv.EXPECT().GetOrCreateSession(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("invalid uuid"))

	ag := newTestAgent(t, mockLLM, mockConv, mockMem, mockUsers, mockReg)
	_, err := ag.HandleMessage(t.Context(), channel.Message{UserID: "bad-id"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get session")
}

func TestHandleMessage_WithToolCall(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLLM := NewMockchatProvider(ctrl)
	mockConv := NewMockconvStore(ctrl)
	mockMem := NewMockmemStore(ctrl)
	mockUsers := NewMockuserStore(ctrl)
	mockReg := NewMocktoolRegistry(ctrl)

	sess := testSession()
	msg := channel.Message{UserID: testUserID, Text: "Show my accounts"}

	mockUsers.EXPECT().GetByID(gomock.Any(), testUserID).Return(&sqlcgen.User{Name: "Alice"}, nil)
	mockConv.EXPECT().GetOrCreateSession(gomock.Any(), testUserID, "", sqlcgen.ChannelEnumCli).Return(sess, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumUser, msg.Text).Return(nil)
	mockConv.EXPECT().RecentMessages(gomock.Any(), sess.ID, int32(20)).Return(nil, nil)
	mockMem.EXPECT().Recall(gomock.Any(), testUserID, gomock.Any(), int32(5)).Return(nil, nil)
	mockReg.EXPECT().Definitions().Return(nil).AnyTimes()

	// First LLM call returns a tool call.
	toolCallMsg := llm.Message{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{
			{ID: "tc1", Name: "get_account_summary", ArgsJSON: `{}`},
		},
	}
	mockLLM.EXPECT().Chat(gomock.Any(), gomock.Any()).Return(llm.ChatResponse{
		Message:    toolCallMsg,
		StopReason: "tool_calls",
	}, nil)
	mockReg.EXPECT().Execute(gomock.Any(), "get_account_summary", "tc1", `{}`).Return("HDFC Savings", nil)

	// Second LLM call returns final answer.
	mockLLM.EXPECT().Chat(gomock.Any(), gomock.Any()).Return(llm.ChatResponse{
		Message:    llm.Message{Role: llm.RoleAssistant, Content: "You have HDFC Savings."},
		StopReason: "stop",
	}, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumAssistant, "You have HDFC Savings.").Return(nil)

	ag := newTestAgent(t, mockLLM, mockConv, mockMem, mockUsers, mockReg)
	resp, err := ag.HandleMessage(t.Context(), msg)
	require.NoError(t, err)
	assert.Equal(t, "You have HDFC Savings.", resp.Text)
}

func TestHandleMessage_ToolError_ContinuesWithErrorString(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLLM := NewMockchatProvider(ctrl)
	mockConv := NewMockconvStore(ctrl)
	mockMem := NewMockmemStore(ctrl)
	mockUsers := NewMockuserStore(ctrl)
	mockReg := NewMocktoolRegistry(ctrl)

	sess := testSession()
	msg := channel.Message{UserID: testUserID, Text: "query txns"}

	mockUsers.EXPECT().GetByID(gomock.Any(), testUserID).Return(nil, errors.New("not found"))
	mockConv.EXPECT().GetOrCreateSession(gomock.Any(), testUserID, "", sqlcgen.ChannelEnumCli).Return(sess, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumUser, msg.Text).Return(nil)
	mockConv.EXPECT().RecentMessages(gomock.Any(), sess.ID, int32(20)).Return(nil, nil)
	mockMem.EXPECT().Recall(gomock.Any(), testUserID, gomock.Any(), int32(5)).Return(nil, nil)
	mockReg.EXPECT().Definitions().Return(nil).AnyTimes()

	mockLLM.EXPECT().Chat(gomock.Any(), gomock.Any()).Return(llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
			{ID: "tc2", Name: "query_transactions", ArgsJSON: `{}`},
		}},
		StopReason: "tool_calls",
	}, nil)
	mockReg.EXPECT().Execute(gomock.Any(), "query_transactions", "tc2", `{}`).Return("", errors.New("db error"))

	// After the tool error, LLM gets "error: db error" as tool result and replies.
	mockLLM.EXPECT().Chat(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, req llm.ChatRequest) (llm.ChatResponse, error) {
			last := req.Messages[len(req.Messages)-1]
			assert.Equal(t, llm.RoleTool, last.Role)
			assert.Contains(t, last.Content, "error: db error")
			return llm.ChatResponse{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "Sorry, DB error."},
				StopReason: "stop",
			}, nil
		})
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumAssistant, "Sorry, DB error.").Return(nil)

	ag := newTestAgent(t, mockLLM, mockConv, mockMem, mockUsers, mockReg)
	resp, err := ag.HandleMessage(t.Context(), msg)
	require.NoError(t, err)
	assert.Equal(t, "Sorry, DB error.", resp.Text)
}

func TestHandleMessage_MaxRounds(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLLM := NewMockchatProvider(ctrl)
	mockConv := NewMockconvStore(ctrl)
	mockMem := NewMockmemStore(ctrl)
	mockUsers := NewMockuserStore(ctrl)
	mockReg := NewMocktoolRegistry(ctrl)

	sess := testSession()
	msg := channel.Message{UserID: testUserID, Text: "go"}

	mockUsers.EXPECT().GetByID(gomock.Any(), testUserID).Return(nil, errors.New("not found"))
	mockConv.EXPECT().GetOrCreateSession(gomock.Any(), testUserID, "", sqlcgen.ChannelEnumCli).Return(sess, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumUser, msg.Text).Return(nil)
	mockConv.EXPECT().RecentMessages(gomock.Any(), sess.ID, int32(20)).Return(nil, nil)
	mockMem.EXPECT().Recall(gomock.Any(), testUserID, gomock.Any(), int32(5)).Return(nil, nil)
	mockReg.EXPECT().Definitions().Return(nil).Times(maxToolRounds)

	// Every LLM call returns a tool call — forces all 8 rounds.
	toolResp := llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
			{ID: "tcX", Name: "some_tool", ArgsJSON: `{}`},
		}},
		StopReason: "tool_calls",
	}
	mockLLM.EXPECT().Chat(gomock.Any(), gomock.Any()).Return(toolResp, nil).Times(maxToolRounds)
	mockReg.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("ok", nil).Times(maxToolRounds)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumAssistant, gomock.Any()).Return(nil)

	ag := newTestAgent(t, mockLLM, mockConv, mockMem, mockUsers, mockReg)
	resp, err := ag.HandleMessage(t.Context(), msg)
	require.NoError(t, err)
	assert.Contains(t, resp.Text, "tool call limit")
}

func TestHandleMessage_WithRecalledMemories(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLLM := NewMockchatProvider(ctrl)
	mockConv := NewMockconvStore(ctrl)
	mockMem := NewMockmemStore(ctrl)
	mockUsers := NewMockuserStore(ctrl)
	mockReg := NewMocktoolRegistry(ctrl)

	sess := testSession()
	msg := channel.Message{UserID: testUserID, Text: "what's my Netflix subscription?"}

	mockUsers.EXPECT().GetByID(gomock.Any(), testUserID).Return(&sqlcgen.User{Name: "Alice"}, nil)
	mockConv.EXPECT().GetOrCreateSession(gomock.Any(), testUserID, "", sqlcgen.ChannelEnumCli).Return(sess, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumUser, msg.Text).Return(nil)
	mockConv.EXPECT().RecentMessages(gomock.Any(), sess.ID, int32(20)).Return(nil, nil)
	mockMem.EXPECT().Recall(gomock.Any(), testUserID, gomock.Any(), int32(5)).Return([]sqlcgen.AgentMemory{
		{Content: "Netflix ₹649/month subscription"},
	}, nil)
	mockReg.EXPECT().Definitions().Return(nil)
	mockLLM.EXPECT().Chat(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, req llm.ChatRequest) (llm.ChatResponse, error) {
			// The system prompt should include the recalled memory.
			assert.Contains(t, req.Messages[0].Content, "Netflix")
			return llm.ChatResponse{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "Your Netflix is ₹649/month."},
				StopReason: "stop",
			}, nil
		})
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumAssistant, "Your Netflix is ₹649/month.").Return(nil)

	ag := newTestAgent(t, mockLLM, mockConv, mockMem, mockUsers, mockReg)
	resp, err := ag.HandleMessage(t.Context(), msg)
	require.NoError(t, err)
	assert.Equal(t, "Your Netflix is ₹649/month.", resp.Text)
}

func TestHandleMessage_SaveUserMessageFails_ContinuesNormally(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLLM := NewMockchatProvider(ctrl)
	mockConv := NewMockconvStore(ctrl)
	mockMem := NewMockmemStore(ctrl)
	mockUsers := NewMockuserStore(ctrl)
	mockReg := NewMocktoolRegistry(ctrl)

	sess := testSession()
	msg := channel.Message{UserID: testUserID, Text: "hi"}

	mockUsers.EXPECT().GetByID(gomock.Any(), testUserID).Return(&sqlcgen.User{Name: "Alice"}, nil)
	mockConv.EXPECT().GetOrCreateSession(gomock.Any(), testUserID, "", sqlcgen.ChannelEnumCli).Return(sess, nil)
	// SaveMessage for user message fails — handler should warn but continue.
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumUser, msg.Text).Return(errors.New("write error"))
	mockConv.EXPECT().RecentMessages(gomock.Any(), sess.ID, int32(20)).Return(nil, nil)
	mockMem.EXPECT().Recall(gomock.Any(), testUserID, gomock.Any(), int32(5)).Return(nil, nil)
	mockReg.EXPECT().Definitions().Return(nil)
	mockLLM.EXPECT().Chat(gomock.Any(), gomock.Any()).Return(llm.ChatResponse{
		Message:    llm.Message{Role: llm.RoleAssistant, Content: "Hi there!"},
		StopReason: "stop",
	}, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumAssistant, "Hi there!").Return(nil)

	ag := newTestAgent(t, mockLLM, mockConv, mockMem, mockUsers, mockReg)
	resp, err := ag.HandleMessage(t.Context(), msg)
	require.NoError(t, err)
	assert.Equal(t, "Hi there!", resp.Text)
}

func TestHandleMessage_HistoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLLM := NewMockchatProvider(ctrl)
	mockConv := NewMockconvStore(ctrl)
	mockMem := NewMockmemStore(ctrl)
	mockUsers := NewMockuserStore(ctrl)
	mockReg := NewMocktoolRegistry(ctrl)

	sess := testSession()
	msg := channel.Message{UserID: testUserID, Text: "hi"}

	mockUsers.EXPECT().GetByID(gomock.Any(), testUserID).Return(nil, errors.New("not found"))
	mockConv.EXPECT().GetOrCreateSession(gomock.Any(), testUserID, "", sqlcgen.ChannelEnumCli).Return(sess, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumUser, msg.Text).Return(nil)
	mockConv.EXPECT().RecentMessages(gomock.Any(), sess.ID, int32(20)).Return(nil, errors.New("db error"))

	ag := newTestAgent(t, mockLLM, mockConv, mockMem, mockUsers, mockReg)
	_, err := ag.HandleMessage(t.Context(), msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load history")
}

func TestHandleMessage_LLMError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLLM := NewMockchatProvider(ctrl)
	mockConv := NewMockconvStore(ctrl)
	mockMem := NewMockmemStore(ctrl)
	mockUsers := NewMockuserStore(ctrl)
	mockReg := NewMocktoolRegistry(ctrl)

	sess := testSession()
	msg := channel.Message{UserID: testUserID, Text: "hi"}

	mockUsers.EXPECT().GetByID(gomock.Any(), testUserID).Return(nil, errors.New("not found"))
	mockConv.EXPECT().GetOrCreateSession(gomock.Any(), testUserID, "", sqlcgen.ChannelEnumCli).Return(sess, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumUser, msg.Text).Return(nil)
	mockConv.EXPECT().RecentMessages(gomock.Any(), sess.ID, int32(20)).Return(nil, nil)
	mockMem.EXPECT().Recall(gomock.Any(), testUserID, gomock.Any(), int32(5)).Return(nil, nil)
	mockReg.EXPECT().Definitions().Return(nil)
	mockLLM.EXPECT().Chat(gomock.Any(), gomock.Any()).Return(llm.ChatResponse{}, errors.New("llm down"))

	ag := newTestAgent(t, mockLLM, mockConv, mockMem, mockUsers, mockReg)
	_, err := ag.HandleMessage(t.Context(), msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "llm chat")
}

func TestHandleMessage_SaveAssistantMessageFails_ReturnsResponseAnyway(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLLM := NewMockchatProvider(ctrl)
	mockConv := NewMockconvStore(ctrl)
	mockMem := NewMockmemStore(ctrl)
	mockUsers := NewMockuserStore(ctrl)
	mockReg := NewMocktoolRegistry(ctrl)

	sess := testSession()
	msg := channel.Message{UserID: testUserID, Text: "hi"}

	mockUsers.EXPECT().GetByID(gomock.Any(), testUserID).Return(&sqlcgen.User{Name: "Alice"}, nil)
	mockConv.EXPECT().GetOrCreateSession(gomock.Any(), testUserID, "", sqlcgen.ChannelEnumCli).Return(sess, nil)
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumUser, msg.Text).Return(nil)
	mockConv.EXPECT().RecentMessages(gomock.Any(), sess.ID, int32(20)).Return(nil, nil)
	mockMem.EXPECT().Recall(gomock.Any(), testUserID, gomock.Any(), int32(5)).Return(nil, nil)
	mockReg.EXPECT().Definitions().Return(nil)
	mockLLM.EXPECT().Chat(gomock.Any(), gomock.Any()).Return(llm.ChatResponse{
		Message:    llm.Message{Role: llm.RoleAssistant, Content: "Done."},
		StopReason: "stop",
	}, nil)
	// Assistant message save fails — handler should warn but still return response.
	mockConv.EXPECT().SaveMessage(gomock.Any(), sess.ID, sqlcgen.MsgRoleEnumAssistant, "Done.").Return(errors.New("write error"))

	ag := newTestAgent(t, mockLLM, mockConv, mockMem, mockUsers, mockReg)
	resp, err := ag.HandleMessage(t.Context(), msg)
	require.NoError(t, err)
	assert.Equal(t, "Done.", resp.Text)
}

func TestBuildMessages_SkipsDuplicateUserMessage(t *testing.T) {
	history := []sqlcgen.ConversationMessage{
		{Role: sqlcgen.MsgRoleEnumAssistant, Content: "Hi"},
		{Role: sqlcgen.MsgRoleEnumUser, Content: "hello"},
	}
	msgs := buildMessages("sys", history, "hello")

	// system + assistant + deduplicated user = 3 (not 4)
	assert.Len(t, msgs, 3)
	assert.Equal(t, llm.RoleSystem, msgs[0].Role)
	assert.Equal(t, llm.RoleAssistant, msgs[1].Role)
	assert.Equal(t, llm.RoleUser, msgs[2].Role)
	assert.Equal(t, "hello", msgs[2].Content)
}

func TestBuildMessages_EmptyHistory(t *testing.T) {
	msgs := buildMessages("sys", nil, "hi")
	assert.Len(t, msgs, 2)
	assert.Equal(t, llm.RoleSystem, msgs[0].Role)
	assert.Equal(t, llm.RoleUser, msgs[1].Role)
}

func TestExtractTags_FiltersShortWords(t *testing.T) {
	tags := extractTags("Show me spending on food and drinks")
	assert.Contains(t, tags, "spending")
	assert.Contains(t, tags, "drinks")
	// "Show","me","on","and" are ≤4 chars — excluded (case-folded)
	for _, tag := range tags {
		assert.Greater(t, len(tag), 4, "tag %q should be >4 chars", tag)
	}
}

func TestExtractTags_Deduplicated(t *testing.T) {
	tags := extractTags("total total total")
	count := 0
	for _, t := range tags {
		if t == "total" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestSplitWords_EarlyStop(t *testing.T) {
	// Consumer stops after the first word; splitWords must not panic or loop.
	var got []string
	for w := range splitWords("hello world foo") {
		got = append(got, w)
		break
	}
	assert.Equal(t, []string{"hello"}, got)
}
