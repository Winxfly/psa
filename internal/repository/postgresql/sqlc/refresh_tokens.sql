-- name: InsertRefreshToken :one
INSERT INTO refresh_tokens (user_id, hashed_token, expires_at)
VALUES ($1, $2, $3)
    RETURNING user_id, hashed_token, created_at, expires_at;

-- name: GetRefreshToken :one
SELECT user_id, hashed_token, created_at, expires_at
FROM refresh_tokens
WHERE user_id = $1 AND hashed_token = $2;

-- name: DeleteRefreshToken :exec
DELETE FROM refresh_tokens
WHERE user_id = $1 AND hashed_token = $2;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE expires_at < NOW();