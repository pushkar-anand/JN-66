package agent

//go:generate go tool mockgen -source=deps.go -destination=mock_deps_test.go -package=agent

import (
	"context"

	"github.com/google/uuid"

	"github.com/pushkaranand/finagent/internal/llm"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

type chatProvider interface {
	Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error)
}

type convStore interface {
	GetOrCreateSession(ctx context.Context, userID, sessionID string, ch sqlcgen.ChannelEnum) (*sqlcgen.ConversationSession, error)
	SaveMessage(ctx context.Context, sessionID uuid.UUID, role sqlcgen.MsgRoleEnum, content string) error
	RecentMessages(ctx context.Context, sessionID uuid.UUID, limit int32) ([]sqlcgen.ConversationMessage, error)
}

type memStore interface {
	Recall(ctx context.Context, userID string, queryTags []string, limit int32) ([]sqlcgen.AgentMemory, error)
}

type userStore interface {
	GetByID(ctx context.Context, id string) (*sqlcgen.User, error)
}

type toolRegistry interface {
	Definitions() []llm.ToolDefinition
	Execute(ctx context.Context, name, callID, argsJSON string) (string, error)
}
