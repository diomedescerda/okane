-- +goose Up
ALTER TABLE users
    ADD COLUMN language text NOT NULL DEFAULT 'en'
    CHECK (language IN ('en', 'es', 'ja'));

-- +goose Down
ALTER TABLE users DROP COLUMN language;
