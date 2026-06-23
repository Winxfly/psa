DROP INDEX IF EXISTS idx_skill_extracted_scraped_profession_skill_unique;
DROP INDEX IF EXISTS idx_skill_formal_scraped_profession_skill_unique;
DROP INDEX IF EXISTS idx_stat_scraped_profession_unique;

CREATE INDEX idx_stat_scraped_profession
ON stat (scraped_at_id, profession_id);

CREATE INDEX idx_skill_formal_scraped_profession
ON skill_formal (scraped_at_id, profession_id);

CREATE INDEX idx_skill_extracted_scraped_profession
ON skill_extracted (scraped_at_id, profession_id);