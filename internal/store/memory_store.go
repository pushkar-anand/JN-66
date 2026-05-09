package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// MemoryStore handles agent_memories data access.
type MemoryStore struct {
	DB
	q *sqlcgen.Queries
}

// NewMemoryStore creates a MemoryStore backed by pool.
func NewMemoryStore(pool *pgxpool.Pool) *MemoryStore {
	return &MemoryStore{DB: newDB(pool), q: sqlcgen.New(pool)}
}

func toPgtypeUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// Save stores a new memory for the given user (nil = household-wide).
func (s *MemoryStore) Save(ctx context.Context, userID *string, content string, memType sqlcgen.MemoryTypeEnum, tags []string) (*sqlcgen.AgentMemory, error) {
	var pgUID pgtype.UUID
	if userID != nil {
		id, err := parseUUID(*userID)
		if err != nil {
			return nil, err
		}
		pgUID = toPgtypeUUID(id)
	}

	m, err := s.q.CreateMemory(ctx, sqlcgen.CreateMemoryParams{
		UserID:          pgUID,
		Content:         content,
		MemoryType:      memType,
		DetectionSource: sqlcgen.DetectionSourceEnumUser,
		Tags:            tags,
	})
	if err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}
	return &m, nil
}

// Recall retrieves memories whose tags overlap with queryTags for the given user.
func (s *MemoryStore) Recall(ctx context.Context, userID string, queryTags []string, limit int32) ([]sqlcgen.AgentMemory, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.RecallMemoriesByTags(ctx, sqlcgen.RecallMemoriesByTagsParams{
		UserID:    toPgtypeUUID(uid),
		Tags:      queryTags,
		PageLimit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("recall memories: %w", err)
	}
	return rows, nil
}

// List returns recent active memories for a user.
func (s *MemoryStore) List(ctx context.Context, userID string, limit int32) ([]sqlcgen.AgentMemory, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListMemories(ctx, sqlcgen.ListMemoriesParams{
		UserID:    toPgtypeUUID(uid),
		PageLimit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	return rows, nil
}
