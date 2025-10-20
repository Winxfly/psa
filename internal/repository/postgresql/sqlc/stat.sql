-- name: InsertStat :one
INSERT INTO stat (profession_id, vacancy_count, scraped_at_id)
VALUES ($1, $2, $3) RETURNING id;

-- name: GetLatestStatByProfessionID :one
SELECT profession_id, vacancy_count, scraped_at_id
FROM stat
WHERE profession_id = $1
ORDER BY scraped_at_id DESC LIMIT 1;

-- name: GetStatsByProfessionsAndDateRange :many
SELECT profession_id, vacancy_count, scraped_at_id
FROM stat
         JOIN scraping sc ON stat.scraped_at_id = sc.id
WHERE profession_id = ANY ($1::uuid[])
  AND sc.scraped_at BETWEEN $2 AND $3
ORDER BY profession_id, sc.scraped_at;