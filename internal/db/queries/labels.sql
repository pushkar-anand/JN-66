-- name: ListLabels :many
SELECT * FROM labels WHERE user_id = sqlc.narg(user_id) OR user_id IS NULL ORDER BY name;

-- name: GetLabelBySlug :one
SELECT * FROM labels WHERE slug = @slug;

-- name: CreateLabel :one
INSERT INTO labels (user_id, name, slug, color)
VALUES (sqlc.narg(user_id), @name, @slug, sqlc.narg(color))
RETURNING *;

-- name: AddTransactionLabel :exec
INSERT INTO transaction_labels (transaction_id, label_id, source)
VALUES (@transaction_id, @label_id, @source)
ON CONFLICT DO NOTHING;

-- name: RemoveTransactionLabel :exec
DELETE FROM transaction_labels WHERE transaction_id = @transaction_id AND label_id = @label_id;

-- name: ListTransactionLabels :many
SELECT l.* FROM labels l
JOIN transaction_labels tl ON tl.label_id = l.id
WHERE tl.transaction_id = @transaction_id;
