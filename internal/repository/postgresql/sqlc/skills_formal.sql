-- name: InsertFormalSkills :copyfrom
INSERT INTO skill_formal (profession_id, skill, count, scraped_at_id)
VALUES ($1, $2, $3, $4);

-- name: GetFormalSkillsByProfessionAndDate :many
SELECT skill, count
FROM skill_formal
WHERE profession_id = $1
  AND scraped_at_id = $2
ORDER BY count DESC;

-- name: GetFormalSkillsWithDatesByProfessionAndDateRange :many
SELECT s.skill, s.count, sc.scraped_at
FROM skill_formal s
         JOIN scraping sc ON s.scraped_at_id = sc.id
WHERE s.profession_id = $1
  AND sc.scraped_at BETWEEN $2 AND $3
ORDER BY sc.scraped_at ASC;

-- name: GetFormalSkillsWithDatesByProfessionsAndDateRange :many
SELECT s.profession_id, s.skill, s.count, sc.scraped_at
FROM skill_formal s
         JOIN scraping sc ON s.scraped_at_id = sc.id
WHERE s.profession_id = ANY ($1::uuid[])
  AND sc.scraped_at BETWEEN $2 AND $3
ORDER BY s.profession_id, sc.scraped_at;