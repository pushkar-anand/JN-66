package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// ConversationStore handles session and message persistence.
type ConversationStore struct {
	DB
	q sqlcgen.Querier
}

// NewConversationStore creates a ConversationStore backed by pool.
func NewConversationStore(pool *pgxpool.Pool) *ConversationStore {
	return &ConversationStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

func newConversationStoreForTest(q sqlcgen.Querier) *ConversationStore {
	return &ConversationStore{q: q}
}

// GetOrCreateSession returns the existing session by ID, or creates a new one.
func (s *ConversationStore) GetOrCreateSession(ctx context.Context, userID, sessionID string, ch sqlcgen.ChannelEnum) (*sqlcgen.ConversationSession, error) {
	if sessionID != "" {
		sid, err := uuid.Parse(sessionID)
		if err == nil {
			sess, err := s.q.GetSession(ctx, sid)
			if err == nil {
				if err := s.q.TouchSession(ctx, sess.ID); err != nil {
					return nil, fmt.Errorf("touch session: %w", err)
				}
				return &sess, nil
			}
		}
	}

	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	sess, err := s.q.CreateSession(ctx, sqlcgen.CreateSessionParams{
		UserID:  uid,
		Channel: ch,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return &sess, nil
}

// SaveMessage appends a message to the session.
func (s *ConversationStore) SaveMessage(ctx context.Context, sessionID uuid.UUID, role sqlcgen.MsgRoleEnum, content string) error {
	_, err := s.q.SaveMessage(ctx, sqlcgen.SaveMessageParams{
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Metadata:  []byte("{}"),
	})
	if err != nil {
		return fmt.Errorf("save message: %w", err)
	}
	return nil
}

// RecentMessages returns up to limit messages for the session, oldest first.
func (s *ConversationStore) RecentMessages(ctx context.Context, sessionID uuid.UUID, limit int32) ([]sqlcgen.ConversationMessage, error) {
	rows, err := s.q.ListRecentMessages(ctx, sqlcgen.ListRecentMessagesParams{
		SessionID: sessionID,
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list recent messages: %w", err)
	}
	// rows are newest-first from the query; reverse to oldest-first for the LLM context.
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}
