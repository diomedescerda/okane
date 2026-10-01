-- +goose Up

-- Colombia defaults: accounts are in Colombian pesos.
ALTER TABLE accounts ALTER COLUMN currency SET DEFAULT 'COP';
UPDATE accounts SET currency = 'COP' WHERE currency = 'MXN';

-- Colombian bank fee on transactions (Gravamen a los Movimientos Financieros).
INSERT INTO categories (id, parent_id, slug, name, kind, icon, sort_order, is_system)
VALUES ('c0000000-0000-4000-8000-000000000076', 'c0000000-0000-4000-8000-000000000060',
        'gmf', 'GMF / 4x1000', 'expense', '🏛️', 204, true);

-- Institutions are banks or wallets that accounts can be grouped under. System
-- rows (user_id IS NULL) ship with the app; users can add their own.
CREATE TABLE institutions (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 uuid REFERENCES users(id) ON DELETE CASCADE,
    provider                text NOT NULL DEFAULT 'manual',
    provider_institution_id text,
    slug                    text NOT NULL DEFAULT '',
    name                    text NOT NULL DEFAULT '',
    display_name            text NOT NULL,
    country_code            char(2) NOT NULL DEFAULT 'CO',
    type                    text NOT NULL DEFAULT 'bank' CHECK (type IN ('bank', 'wallet', 'other')),
    website                 text NOT NULL DEFAULT '',
    primary_color           text NOT NULL DEFAULT '',
    logo_url                text NOT NULL DEFAULT '',
    icon                    text NOT NULL DEFAULT '',
    sort_order              int NOT NULL DEFAULT 100,
    is_system               boolean NOT NULL DEFAULT false,
    is_active               boolean NOT NULL DEFAULT true,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX institutions_system_slug_idx ON institutions (slug) WHERE is_system;
CREATE UNIQUE INDEX institutions_user_name_idx ON institutions (user_id, lower(display_name))
    WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX institutions_provider_idx ON institutions (provider, provider_institution_id)
    WHERE provider_institution_id IS NOT NULL;
CREATE INDEX institutions_country_idx ON institutions (country_code, type) WHERE is_active;

CREATE TRIGGER institutions_set_updated_at BEFORE UPDATE ON institutions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Connections can now point at an institution row (bank name, logo, color).
ALTER TABLE links ADD COLUMN institution_id uuid REFERENCES institutions(id) ON DELETE SET NULL;
CREATE INDEX links_institution_idx ON links (institution_id);

-- +goose Down

DROP INDEX IF EXISTS links_institution_idx;
ALTER TABLE links DROP COLUMN IF EXISTS institution_id;
DROP TABLE IF EXISTS institutions;
DELETE FROM categories WHERE id = 'c0000000-0000-4000-8000-000000000076';
ALTER TABLE accounts ALTER COLUMN currency SET DEFAULT 'MXN';
UPDATE accounts SET currency = 'MXN' WHERE currency = 'COP';
