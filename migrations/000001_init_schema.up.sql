-- Расширение для генерации UUID
CREATE
EXTENSION IF NOT EXISTS "pgcrypto";

-- Таблица профессий
CREATE TABLE profession
(
    id            UUID PRIMARY KEY      DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL UNIQUE,
    vacancy_query VARCHAR(255) NOT NULL,
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE
);

-- Таблица сессий сбора данных
CREATE TABLE scraping
(
    id         UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    scraped_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица формальных навыков из поля "ключевые навыки"
CREATE TABLE skill_formal
(
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profession_id UUID         NOT NULL REFERENCES profession (id) ON DELETE CASCADE,
    skill         VARCHAR(255) NOT NULL,
    count         INTEGER      NOT NULL,
    scraped_at_id UUID         NOT NULL REFERENCES scraping (id) ON DELETE CASCADE
);

-- Таблица извлечённых навыков из текста описания вакансии
CREATE TABLE skill_extracted
(
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profession_id UUID         NOT NULL REFERENCES profession (id) ON DELETE CASCADE,
    skill         VARCHAR(255) NOT NULL,
    count         INTEGER      NOT NULL,
    scraped_at_id UUID         NOT NULL REFERENCES scraping (id) ON DELETE CASCADE
);

-- Таблица агрегированной статистики
CREATE TABLE stat
(
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profession_id UUID    NOT NULL REFERENCES profession (id) ON DELETE CASCADE,
    vacancy_count INTEGER NOT NULL,
    scraped_at_id UUID    NOT NULL REFERENCES scraping (id) ON DELETE CASCADE
);

CREATE TABLE users
(
    id              UUID PRIMARY KEY             DEFAULT gen_random_uuid(),
    email           VARCHAR(320) UNIQUE NOT NULL,
    hashed_password VARCHAR(96)         NOT NULL,
    is_admin        BOOLEAN             NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ                  DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE refresh_tokens
(
    user_id      UUID REFERENCES users (id) ON DELETE CASCADE,
    hashed_token VARCHAR(500) NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at   TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP + INTERVAL '1 days',
    PRIMARY KEY (user_id, hashed_token)
);

-- Часто выбираем по дате + профессии
CREATE INDEX idx_skill_formal_scraped_profession ON skill_formal (scraped_at_id, profession_id);
CREATE INDEX idx_skill_extracted_scraped_profession ON skill_extracted (scraped_at_id, profession_id);
CREATE INDEX idx_stat_scraped_profession ON stat (scraped_at_id, profession_id);
CREATE INDEX idx_scraping_scraped_at ON scraping (scraped_at);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens (expires_at);