-- name: ListCategories :many
SELECT * FROM categories ORDER BY depth, slug;

-- name: ListTopLevelCategories :many
SELECT * FROM categories WHERE depth = 0 ORDER BY slug;

-- name: ListSubCategories :many
SELECT * FROM categories WHERE parent_id = $1 ORDER BY slug;

-- name: GetCategoryBySlug :one
SELECT * FROM categories WHERE slug = $1;

-- name: GetCategoryByID :one
SELECT * FROM categories WHERE id = $1;
