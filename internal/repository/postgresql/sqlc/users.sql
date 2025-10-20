-- name: GetUserByEmail :one
SELECT id, email, hashed_password, is_admin, created_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, hashed_password, is_admin, created_at
FROM users
WHERE id = $1;

-- name: InsertUser :one
INSERT INTO users (email, hashed_password, is_admin)
VALUES ($1, $2, $3)
    RETURNING id;

-- name: IsAdmin :one
SELECT is_admin
FROM users
WHERE id = $1;