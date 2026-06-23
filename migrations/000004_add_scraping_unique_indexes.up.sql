DROP INDEX IF EXISTS idx_stat_scraped_profession;
DROP INDEX IF EXISTS idx_skill_formal_scraped_profession;
DROP INDEX IF EXISTS idx_skill_extracted_scraped_profession;

CREATE UNIQUE INDEX idx_stat_scraped_profession_unique
ON stat (scraped_at_id, profession_id);

CREATE UNIQUE INDEX idx_skill_formal_scraped_profession_skill_unique
ON skill_formal (scraped_at_id, profession_id, skill);

CREATE UNIQUE INDEX idx_skill_extracted_scraped_profession_skill_unique
ON skill_extracted (scraped_at_id, profession_id, skill);