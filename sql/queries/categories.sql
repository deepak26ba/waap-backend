-- name: ListCategories :many
SELECT * FROM categories
ORDER BY name ASC;

-- name: GetCategoryByID :one
SELECT * FROM categories
WHERE id = $1 LIMIT 1;

-- name: GetCategoryBySlug :one
SELECT * FROM categories
WHERE slug = $1 LIMIT 1;

-- name: CreateCategory :one
INSERT INTO categories (
    parent_id,
    name,
    slug,
    description,
    image_url
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

