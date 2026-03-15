-- name: InsertStatDaily :one
INSERT INTO stat_daily (profession_id, vacancy_count, scraped_at)
VALUES ($1, $2, $3) RETURNING id;

-- name: GetStatDailyByProfessionID :many
SELECT DISTINCT ON (DATE(scraped_at))
    profession_id, vacancy_count, scraped_at
FROM stat_daily
WHERE profession_id = $1
ORDER BY DATE(scraped_at), scraped_at DESC;

-- name: GetStatDailyByProfessionIDs :many
SELECT profession_id, vacancy_count, scraped_at
FROM stat_daily
WHERE profession_id = ANY ($1::uuid[])
ORDER BY profession_id, scraped_at;
