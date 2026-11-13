-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (
    email,
    password_hash,
    full_name,
    role,
    phone_number
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET full_name = COALESCE($2, full_name),
    phone_number = COALESCE($3, phone_number),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: MarkUserEmailVerified :one
UPDATE users
SET email_verified = TRUE,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;



