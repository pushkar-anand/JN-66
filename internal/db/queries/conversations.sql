-- name: CreateSession :one
INSERT INTO conversation_sessions (user_id, channel, title)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSession :one
SELECT * FROM conversation_sessions WHERE id = $1;

-- name: TouchSession :exec
UPDATE conversation_sessions SET last_active = NOW() WHERE id = $1;

-- name: UpdateSessionTitle :exec
UPDATE conversation_sessions SET title = $2 WHERE id = $1;

-- name: SaveMessage :one
INSERT INTO conversation_messages (session_id, role, content, model_used, tool_name, tool_call_id, token_count, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListRecentMessages :many
SELECT * FROM conversation_messages
WHERE session_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ListUserSessions :many
SELECT * FROM conversation_sessions
WHERE user_id = $1
ORDER BY last_active DESC
LIMIT $2;
