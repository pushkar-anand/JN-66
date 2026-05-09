-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY name;

-- name: CreateUser :one
INSERT INTO users (name, email, phone, timezone, preferences)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateUserPreferences :one
UPDATE users SET preferences = $2, updated_at = NOW() WHERE id = $1 RETURNING *;
