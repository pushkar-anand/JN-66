package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pushkar-anand/agentrig/channel"
	"github.com/pushkaranand/finagent/internal/llm"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

const maxToolRounds = 8

// Agent is the core ReAct loop. It receives messages from any channel,
// calls the LLM, dispatches tool calls, and returns a response.
type Agent struct {
	llm         chatProvider
	conv        convStore
	memories    memStore
	users       userStore
	registry    toolRegistry
	router      *Router
	hasZerodha  bool
	channelType sqlcgen.ChannelEnum
}

// New creates an Agent with all dependencies wired in.
// hasZerodha should be true when Zerodha investment tools are registered in registry.
// channelType is stored in conversation_sessions for provenance tracking.
func New(
	provider chatProvider,
	conv convStore,
	memories memStore,
	users userStore,
	registry toolRegistry,
	router *Router,
	hasZerodha bool,
	channelType sqlcgen.ChannelEnum,
) *Agent {
	return &Agent{
		llm:         provider,
		conv:        conv,
		memories:    memories,
		users:       users,
		registry:    registry,
		router:      router,
		hasZerodha:  hasZerodha,
		channelType: channelType,
	}
}

// HandleMessage is the MessageHandler the channel layer calls for each inbound message.
func (a *Agent) HandleMessage(ctx context.Context, msg channel.Message) (channel.Response, error) {
	t0 := time.Now()
	slog.InfoContext(ctx, "agent session start",
		slog.String("user_id", msg.UserID),
		slog.String("session_id", msg.SessionID),
	)

	// Resolve the user's display name for the system prompt.
	userName := msg.UserID
	if u, err := a.users.GetByID(ctx, msg.UserID); err == nil {
		userName = u.Name
	}

	// Get or create a conversation session.
	sess, err := a.conv.GetOrCreateSession(ctx, msg.UserID, msg.SessionID, a.channelType)
	if err != nil {
		return channel.Response{}, fmt.Errorf("get session: %w", err)
	}

	// Persist the incoming user message.
	if err := a.conv.SaveMessage(ctx, sess.ID, sqlcgen.MsgRoleEnumUser, msg.Text); err != nil {
		slog.Warn("failed to save user message", "err", err)
	}

	// Load recent history and recalled memories.
	history, err := a.conv.RecentMessages(ctx, sess.ID, 20)
	if err != nil {
		return channel.Response{}, fmt.Errorf("load history: %w", err)
	}

	recalledMems, _ := a.memories.Recall(ctx, msg.UserID, msg.Text, 5)
	memStrings := make([]string, len(recalledMems))
	for i, m := range recalledMems {
		memStrings[i] = m.Content
	}
	slog.DebugContext(ctx, "memories recalled", slog.Int("count", len(recalledMems)))
	slog.Debug("agent setup", "elapsed_ms", time.Since(t0).Milliseconds())

	// Build the full message list for the LLM.
	messages := buildMessages(systemPrompt(userName, msg.UserID, memStrings, a.hasZerodha), history, msg.Text)
	model := a.router.Select(msg.Text, RouterHintChat)

	var finalText string
	var round int
	for round = range maxToolRounds {
		tLLM := time.Now()
		resp, err := a.llm.Chat(ctx, llm.ChatRequest{
			Model:    model,
			Messages: messages,
			Tools:    a.registry.Definitions(),
		})
		slog.DebugContext(ctx, "llm round",
			slog.String("session_id", msg.SessionID),
			slog.Int("round", round+1),
			slog.String("model", model),
			slog.Int64("elapsed_ms", time.Since(tLLM).Milliseconds()),
		)
		if err != nil {
			return channel.Response{}, fmt.Errorf("llm chat: %w", err)
		}

		messages = append(messages, resp.Message)

		if resp.StopReason == "stop" || len(resp.Message.ToolCalls) == 0 {
			finalText = resp.Message.Content
			break
		}

		// Execute each tool call and append results.
		for _, tc := range resp.Message.ToolCalls {
			tTool := time.Now()
			result, err := a.registry.Execute(ctx, tc.Name, tc.ID, tc.ArgsJSON)
			slog.DebugContext(ctx, "tool call",
				slog.String("session_id", msg.SessionID),
				slog.String("tool", tc.Name),
				slog.Int64("elapsed_ms", time.Since(tTool).Milliseconds()),
			)
			if err != nil {
				result = "error: " + err.Error()
				slog.WarnContext(ctx, "tool error",
					slog.String("session_id", msg.SessionID),
					slog.String("tool", tc.Name),
					slog.Any("error", err),
				)
			}
			messages = append(messages, llm.ToolResultMessage(tc.ID, result, tc.Name))
		}
	}

	if finalText == "" {
		slog.WarnContext(ctx, "agent tool limit reached",
			slog.String("session_id", msg.SessionID),
			slog.Int("max_rounds", maxToolRounds),
		)
		finalText = "I reached the tool call limit without a final answer. Please try a more specific question."
	}

	// Persist the assistant reply.
	if err := a.conv.SaveMessage(ctx, sess.ID, sqlcgen.MsgRoleEnumAssistant, finalText); err != nil {
		slog.Warn("failed to save assistant message", "err", err)
	}

	slog.InfoContext(ctx, "agent session done",
		slog.String("user_id", msg.UserID),
		slog.String("session_id", msg.SessionID),
		slog.Int("rounds", round+1),
		slog.Int64("total_ms", time.Since(t0).Milliseconds()),
	)
	return channel.Response{Text: finalText, Markdown: true}, nil
}

// buildMessages constructs the ordered LLM message list from history and the new user input.
func buildMessages(sysPrompt string, history []sqlcgen.ConversationMessage, userText string) []llm.Message {
	msgs := []llm.Message{llm.SystemMessage(sysPrompt)}
	for _, h := range history {
		// Skip the last user message — we append it separately below to avoid duplication.
		if h.Role == sqlcgen.MsgRoleEnumUser && h.Content == userText {
			continue
		}
		msgs = append(msgs, llm.Message{
			Role:    llm.Role(h.Role),
			Content: h.Content,
		})
	}
	msgs = append(msgs, llm.UserMessage(userText))
	return msgs
}
