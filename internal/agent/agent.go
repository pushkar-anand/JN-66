package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/pushkaranand/finagent/internal/channel"
	"github.com/pushkaranand/finagent/internal/llm"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/pushkaranand/finagent/internal/store"
	"github.com/pushkaranand/finagent/internal/tools"
)

const maxToolRounds = 8

// Agent is the core ReAct loop. It receives messages from any channel,
// calls the LLM, dispatches tool calls, and returns a response.
type Agent struct {
	llm      llm.Provider
	conv     *store.ConversationStore
	memories *store.MemoryStore
	users    *store.UserStore
	registry *tools.Registry
	router   *Router
}

// New creates an Agent with all dependencies wired in.
func New(
	provider llm.Provider,
	conv *store.ConversationStore,
	memories *store.MemoryStore,
	users *store.UserStore,
	registry *tools.Registry,
	router *Router,
) *Agent {
	return &Agent{
		llm:      provider,
		conv:     conv,
		memories: memories,
		users:    users,
		registry: registry,
		router:   router,
	}
}

// HandleMessage is the MessageHandler the channel layer calls for each inbound message.
func (a *Agent) HandleMessage(ctx context.Context, msg channel.Message) (channel.Response, error) {
	// Resolve the user's display name for the system prompt.
	userName := msg.UserID
	if u, err := a.users.GetByID(ctx, msg.UserID); err == nil {
		userName = u.Name
	}

	// Get or create a conversation session.
	sess, err := a.conv.GetOrCreateSession(ctx, msg.UserID, msg.SessionID, sqlcgen.ChannelEnumCli)
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

	recalledMems, _ := a.memories.Recall(ctx, msg.UserID, extractTags(msg.Text), 5)
	memStrings := make([]string, len(recalledMems))
	for i, m := range recalledMems {
		memStrings[i] = m.Content
	}

	// Build the full message list for the LLM.
	messages := buildMessages(systemPrompt(userName, msg.UserID, memStrings), history, msg.Text)
	model := a.router.Select(msg.Text, RouterHintChat)

	var finalText string
	for range maxToolRounds {
		resp, err := a.llm.Chat(ctx, llm.ChatRequest{
			Model:    model,
			Messages: messages,
			Tools:    a.registry.Definitions(),
		})
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
			slog.Debug("tool call", "name", tc.Name, "id", tc.ID)
			result, err := a.registry.Execute(ctx, tc.Name, tc.ID, tc.ArgsJSON)
			if err != nil {
				result = "error: " + err.Error()
				slog.Warn("tool error", "tool", tc.Name, "err", err)
			}
			messages = append(messages, llm.ToolResultMessage(tc.ID, result, tc.Name))
		}
	}

	if finalText == "" {
		finalText = "I reached the tool call limit without a final answer. Please try a more specific question."
	}

	// Persist the assistant reply.
	if err := a.conv.SaveMessage(ctx, sess.ID, sqlcgen.MsgRoleEnumAssistant, finalText); err != nil {
		slog.Warn("failed to save assistant message", "err", err)
	}

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

// extractTags derives simple keyword tags from the user's message for memory recall.
func extractTags(text string) []string {
	// Naive word extraction; Phase 2 will replace with embedding-based retrieval.
	words := make(map[string]struct{})
	for word := range splitWords(text) {
		if len(word) > 4 {
			words[word] = struct{}{}
		}
	}
	tags := make([]string, 0, len(words))
	for w := range words {
		tags = append(tags, w)
	}
	return tags
}

func splitWords(s string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		word := make([]byte, 0, 16)
		for i := range len(s) {
			c := s[i]
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
				if c >= 'A' && c <= 'Z' {
					c += 32
				}
				word = append(word, c)
			} else if len(word) > 0 {
				if !yield(string(word)) {
					return
				}
				word = word[:0]
			}
		}
		if len(word) > 0 {
			yield(string(word))
		}
	}
}

// ensure uuid import is used (sess.ID is uuid.UUID).
var _ = uuid.UUID{}
