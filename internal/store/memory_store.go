package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// memoryRecallMaxDistance is the cosine distance ceiling for vector recall.
// Cosine distance: 0 = identical, 1 = orthogonal, 2 = opposite.
// Memories beyond this threshold are considered semantically unrelated and excluded.
const memoryRecallMaxDistance = 0.5

// MemoryStore handles agent_memories data access.
type MemoryStore struct {
	DB
	q        sqlcgen.Querier
	embedder Embedder // nil disables vector recall; falls back to tag-based
}

// NewMemoryStore creates a MemoryStore backed by pool.
// Pass a non-nil Embedder to enable vector-similarity recall; nil falls back to tag-based.
func NewMemoryStore(pool *pgxpool.Pool, embedder Embedder) *MemoryStore {
	return &MemoryStore{DB: newDB(pool), q: sqlcgen.New(pool), embedder: embedder}
}

func newMemoryStoreForTest(q sqlcgen.Querier) *MemoryStore {
	return &MemoryStore{q: q}
}

func toPgtypeUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// Save stores a new memory for the given user (nil = household-wide).
// When an Embedder is configured, the embedding is populated automatically.
// An embed failure is logged as a warning and the memory is stored without an embedding.
func (s *MemoryStore) Save(ctx context.Context, userID *string, content string, memType sqlcgen.MemoryTypeEnum, tags []string) (*sqlcgen.AgentMemory, error) {
	var pgUID pgtype.UUID
	if userID != nil {
		id, err := parseUUID(*userID)
		if err != nil {
			return nil, err
		}
		pgUID = toPgtypeUUID(id)
	}

	params := sqlcgen.CreateMemoryParams{
		UserID:          pgUID,
		Content:         content,
		MemoryType:      memType,
		DetectionSource: sqlcgen.DetectionSourceEnumUser,
		Tags:            tags,
	}

	if s.embedder != nil {
		vec, err := s.embedder.EmbedText(ctx, content)
		if err != nil {
			slog.WarnContext(ctx, "memory embedding failed, storing without embedding", "err", err)
		} else {
			v := pgvector.NewVector(vec)
			params.Embedding = &v
		}
	}

	m, err := s.q.CreateMemory(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}
	return &m, nil
}

// Recall returns memories semantically matching query for the given user.
// Uses vector cosine similarity when an Embedder is configured and succeeds;
// falls back to keyword tag overlap otherwise.
func (s *MemoryStore) Recall(ctx context.Context, userID string, query string, limit int32) ([]sqlcgen.AgentMemory, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	pgUID := toPgtypeUUID(uid)

	if s.embedder != nil {
		vec, embedErr := s.embedder.EmbedText(ctx, query)
		if embedErr == nil {
			v := pgvector.NewVector(vec)
			rows, dbErr := s.q.RecallMemoriesByEmbedding(ctx, sqlcgen.RecallMemoriesByEmbeddingParams{
				UserID:      pgUID,
				Embedding:   &v,
				MaxDistance: memoryRecallMaxDistance,
				PageLimit:   limit,
			})
			if dbErr == nil {
				slog.DebugContext(ctx, "memory recall", slog.String("path", "vector"), slog.Int("results", len(rows)))
				return rows, nil
			}
			slog.WarnContext(ctx, "vector recall failed, falling back to tags", "err", dbErr)
		} else {
			slog.WarnContext(ctx, "embed query failed, falling back to tags", "err", embedErr)
		}
	}

	rows, err := s.q.RecallMemoriesByTags(ctx, sqlcgen.RecallMemoriesByTagsParams{
		UserID:    pgUID,
		Tags:      memoryKeywords(query),
		PageLimit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("recall memories: %w", err)
	}
	slog.DebugContext(ctx, "memory recall", slog.String("path", "tags"), slog.Int("results", len(rows)))
	return rows, nil
}

// RecallTaggingHints returns up to limit tagging-hint memory content strings matching query.
// Satisfies the importer.MemoryRecaller interface.
func (s *MemoryStore) RecallTaggingHints(ctx context.Context, userID, query string, limit int) ([]string, error) {
	memories, err := s.Recall(ctx, userID, query, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(memories))
	for _, m := range memories {
		if m.MemoryType == sqlcgen.MemoryTypeEnumTaggingHint {
			out = append(out, m.Content)
		}
	}
	return out, nil
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

// memoryKeywords extracts lowercase keywords (>4 chars) from text for tag-based recall fallback.
func memoryKeywords(text string) []string {
	seen := make(map[string]struct{})
	var tags []string
	word := make([]byte, 0, 16)
	for i := range len(text) {
		c := text[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			if c >= 'A' && c <= 'Z' {
				c += 32
			}
			word = append(word, c)
		} else if len(word) > 0 {
			if len(word) > 4 {
				w := string(word)
				if _, ok := seen[w]; !ok {
					seen[w] = struct{}{}
					tags = append(tags, w)
				}
			}
			word = word[:0]
		}
	}
	if len(word) > 4 {
		w := string(word)
		if _, ok := seen[w]; !ok {
			tags = append(tags, w)
		}
	}
	if len(tags) == 0 {
		return strings.Fields(strings.ToLower(text))
	}
	return tags
}
