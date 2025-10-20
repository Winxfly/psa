-- name: InsertExtractedSkills :copyfrom
INSERT INTO skill_extracted (profession_id, skill, count, scraped_at_id)
VALUES ($1, $2, $3, $4);

-- name: GetExtractedSkillsByProfessionAndDate :many
SELECT skill, count
FROM skill_extracted
WHERE profession_id = $1
  AND scraped_at_id = $2
ORDER BY count DESC;

-- name: GetExtractedSkillsWithDatesByProfessionAndDateRange :many
SELECT s.skill, s.count, sc.scraped_at
FROM skill_extracted s
         JOIN scraping sc ON s.scraped_at_id = sc.id
WHERE s.profession_id = $1
  AND sc.scraped_at BETWEEN $2 AND $3
ORDER BY sc.scraped_at ASC;

-- name: GetExtractedSkillsWithDatesByProfessionsAndDateRange :many
SELECT s.profession_id, s.skill, s.count, sc.scraped_at
FROM skill_extracted s
         JOIN scraping sc ON s.scraped_at_id = sc.id
WHERE s.profession_id = ANY ($1::uuid[])
  AND sc.scraped_at BETWEEN $2 AND $3
ORDER BY s.profession_id, sc.scraped_at;