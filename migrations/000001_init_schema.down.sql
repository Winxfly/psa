DROP INDEX IF EXISTS idx_stat_scraped_profession;
DROP INDEX IF EXISTS idx_skill_extracted_scraped_profession;
DROP INDEX IF EXISTS idx_skill_formal_scraped_profession;
DROP INDEX IF EXISTS idx_scraping_scraped_at;
DROP INDEX IF EXISTS idx_refresh_tokens_expires_at;

DROP TABLE IF EXISTS stat;
DROP TABLE IF EXISTS skill_extracted;
DROP TABLE IF EXISTS skill_formal;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS scraping;
DROP TABLE IF EXISTS profession;