CREATE TABLE skill_corpus
(
    profession_id UUID         NOT NULL REFERENCES profession (id) ON DELETE CASCADE,
    skill         VARCHAR(255) NOT NULL,
    mention_count INTEGER      NOT NULL,
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (profession_id, skill)
);