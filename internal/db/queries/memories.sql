-- name: CreateMemory :one
INSERT INTO agent_memories (user_id, content, memory_type, detection_source, tags, embedding, expires_at)
VALUES (sqlc.narg(user_id), @content, @memory_type, @detection_source, @tags, sqlc.narg(embedding), sqlc.narg(expires_at))
RETURNING *;

-- name: RecallMemoriesByTags :many
SELECT * FROM agent_memories
WHERE is_active = TRUE
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (user_id = sqlc.narg(user_id) OR user_id IS NULL)
  AND (tags = '{}' OR tags && @tags)
ORDER BY created_at DESC
LIMIT sqlc.arg(page_limit);

-- name: RecallMemoriesByEmbedding :many
SELECT * FROM agent_memories
WHERE is_active = TRUE
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (user_id = sqlc.narg(user_id) OR user_id IS NULL)
  AND embedding IS NOT NULL
  AND embedding <=> @embedding < sqlc.arg(max_distance)::float8
ORDER BY embedding <=> @embedding
LIMIT sqlc.arg(page_limit);

-- name: RecallTaggingHintsByEmbedding :many
SELECT * FROM agent_memories
WHERE is_active = TRUE
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (user_id = sqlc.narg(user_id) OR user_id IS NULL)
  AND memory_type = 'tagging_hint'
  AND embedding IS NOT NULL
  AND embedding <=> @embedding < sqlc.arg(max_distance)::float8
ORDER BY embedding <=> @embedding
LIMIT sqlc.arg(page_limit);

-- name: RecallTaggingHintsByTags :many
SELECT * FROM agent_memories
WHERE is_active = TRUE
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (user_id = sqlc.narg(user_id) OR user_id IS NULL)
  AND memory_type = 'tagging_hint'
  AND (tags = '{}' OR tags && @tags)
ORDER BY created_at DESC
LIMIT sqlc.arg(page_limit);

-- name: DeactivateMemory :exec
UPDATE agent_memories SET is_active = FALSE, updated_at = NOW() WHERE id = @id;

-- name: ListMemories :many
SELECT * FROM agent_memories
WHERE is_active = TRUE
  AND (expires_at IS NULL OR expires_at > NOW())
  AND (user_id = sqlc.narg(user_id) OR user_id IS NULL)
ORDER BY created_at DESC
LIMIT sqlc.arg(page_limit);
