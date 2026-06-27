-- name: GetActiveSkillCorpus :many
SELECT sc.profession_id, sc.skill, sc.mention_count
FROM skill_corpus sc
JOIN profession p ON p.id = sc.profession_id
WHERE p.is_active = true
ORDER BY sc.profession_id, sc.skill;

-- name: DeleteSkillCorpusByProfession :exec
DELETE FROM skill_corpus
WHERE profession_id = $1;

-- name: InsertSkillCorpus :copyfrom
INSERT INTO skill_corpus (profession_id, skill, mention_count)
VALUES ($1, $2, $3);