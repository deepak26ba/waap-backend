-- name: GetCartByUserID :one
SELECT * FROM carts
WHERE user_id = $1
ORDER BY created_at DESC LIMIT 1;

-- name: GetCartByGuestToken :one
SELECT * FROM carts
WHERE guest_token = $1
ORDER BY created_at DESC LIMIT 1;

-- name: CreateCart :one
INSERT INTO carts (
    user_id,
    guest_token
) VALUES (
    $1, $2
)
RETURNING *;

-- name: GetCartItems :many
SELECT * FROM cart_items
WHERE cart_id = $1
ORDER BY created_at ASC;
