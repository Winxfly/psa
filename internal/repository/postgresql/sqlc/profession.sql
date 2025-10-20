-- name: GetAllProfessions :many
SELECT id, name, vacancy_query, is_active
FROM profession
ORDER BY id;

-- name: GetActiveProfessions :many
SELECT id, name, vacancy_query
FROM profession
WHERE is_active = true
ORDER BY id;

-- name: GetProfessionByID :one
SELECT id, name, vacancy_query, is_active
FROM profession
WHERE id = $1;

-- name: GetProfessionByName :one
SELECT id, name, vacancy_query, is_active
FROM profession
WHERE name = $1;

-- name: InsertProfession :one
INSERT INTO profession (name, vacancy_query, is_active)
VALUES ($1, $2, $3) RETURNING id;

-- name: UpdateProfession :exec
UPDATE profession
SET name          = $2,
    vacancy_query = $3,
    is_active     = $4
WHERE id = $1;