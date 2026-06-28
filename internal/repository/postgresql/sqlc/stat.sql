-- name: InsertStat :one
INSERT INTO stat (profession_id, vacancy_count, scraped_at_id)
VALUES ($1, $2, $3) RETURNING id;

-- name: GetLatestStatByProfessionID :one
SELECT st.profession_id, st.vacancy_count, st.scraped_at_id
FROM stat st
         JOIN scraping sc ON st.scraped_at_id = sc.id
WHERE st.profession_id = $1
ORDER BY sc.scraped_at DESC LIMIT 1;

-- name: GetStatsByProfessionsAndDateRange :many
SELECT profession_id, vacancy_count, scraped_at_id
FROM stat
         JOIN scraping sc ON stat.scraped_at_id = sc.id
WHERE profession_id = ANY ($1::uuid[])
  AND sc.scraped_at BETWEEN $2 AND $3
ORDER BY profession_id, sc.scraped_at;
