-- name: CreateUser :one
INSERT INTO users (email, display_name, avatar_url)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: ListUsers :many
SELECT *
FROM users
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: UpdateUser :one
UPDATE users
SET email = $2,
    display_name = $3,
    avatar_url = $4
WHERE id = $1
RETURNING *;

-- name: DeleteUser :execrows
DELETE FROM users
WHERE id = $1;
