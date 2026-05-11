-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY name;

-- name: CreateUser :one
INSERT INTO users (username, name, email, phone, timezone, preferences, api_key_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpsertUser :one
INSERT INTO users (username, name, email, timezone, preferences, api_key_hash)
VALUES ($1, $2, $3, $4, '{}', $5)
ON CONFLICT (username) DO UPDATE SET
  name         = EXCLUDED.name,
  email        = EXCLUDED.email,
  timezone     = EXCLUDED.timezone,
  api_key_hash = EXCLUDED.api_key_hash,
  updated_at   = NOW()
RETURNING *;

-- name: GetUserByAPIKeyHash :one
SELECT * FROM users WHERE api_key_hash = $1 LIMIT 1;

-- name: UpdateUserPreferences :one
UPDATE users SET preferences = $2, updated_at = NOW() WHERE id = $1 RETURNING *;

-- name: UpdateUserDOB :one
UPDATE users SET date_of_birth = $2, updated_at = NOW() WHERE id = $1 RETURNING *;
