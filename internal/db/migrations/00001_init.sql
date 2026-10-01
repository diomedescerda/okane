-- +goose Up

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL,
    password_hash text NOT NULL,
    name          text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));

CREATE TABLE sessions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   bytea NOT NULL UNIQUE,
    user_agent   text NOT NULL DEFAULT '',
    ip           text NOT NULL DEFAULT '',
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

-- A connection to a bank institution, created through an aggregator (Belvo).
CREATE TABLE links (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        text NOT NULL DEFAULT 'belvo',
    provider_link_id text,
    institution     text NOT NULL DEFAULT '',
    institution_name text NOT NULL DEFAULT '',
    access_mode     text NOT NULL DEFAULT 'recurrent',
    status          text NOT NULL DEFAULT 'pending',
    last_sync_at    timestamptz,
    last_sync_error text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX links_provider_link_idx ON links (provider, provider_link_id)
    WHERE provider_link_id IS NOT NULL;
CREATE INDEX links_user_idx ON links (user_id);

CREATE TABLE accounts (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    link_id            uuid REFERENCES links(id) ON DELETE SET NULL,
    provider           text NOT NULL DEFAULT 'manual',
    provider_account_id text,
    name               text NOT NULL DEFAULT '',
    type               text NOT NULL DEFAULT 'other',
    subtype            text NOT NULL DEFAULT '',
    currency           char(3) NOT NULL DEFAULT 'MXN',
    balance_current    numeric(20,4) NOT NULL DEFAULT 0,
    balance_available  numeric(20,4),
    credit_limit       numeric(20,4),
    institution_name   text NOT NULL DEFAULT '',
    is_active          boolean NOT NULL DEFAULT true,
    last_synced_at     timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX accounts_provider_idx ON accounts (user_id, provider, provider_account_id)
    WHERE provider_account_id IS NOT NULL;
CREATE INDEX accounts_user_idx ON accounts (user_id);

-- Categories form a two-level tree. user_id NULL means a system category.
CREATE TABLE categories (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid REFERENCES users(id) ON DELETE CASCADE,
    parent_id  uuid REFERENCES categories(id) ON DELETE CASCADE,
    slug       text NOT NULL,
    name       text NOT NULL,
    kind       text NOT NULL DEFAULT 'expense' CHECK (kind IN ('expense', 'income', 'transfer')),
    icon       text NOT NULL DEFAULT '',
    color      text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 100,
    is_system  boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX categories_system_slug_idx ON categories (slug) WHERE user_id IS NULL;
CREATE UNIQUE INDEX categories_user_slug_idx ON categories (user_id, slug) WHERE user_id IS NOT NULL;

CREATE TABLE transactions (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id             uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    category_id            uuid REFERENCES categories(id) ON DELETE SET NULL,
    provider               text NOT NULL DEFAULT 'manual',
    provider_transaction_id text,
    description            text NOT NULL DEFAULT '',
    merchant               text,
    -- Signed amount: negative = money out, positive = money in.
    amount                 numeric(20,4) NOT NULL,
    direction              text NOT NULL CHECK (direction IN ('in', 'out')),
    currency               char(3) NOT NULL DEFAULT 'MXN',
    status                 text NOT NULL DEFAULT 'posted' CHECK (status IN ('posted', 'pending')),
    category_source        text NOT NULL DEFAULT '',
    occurred_at            timestamptz NOT NULL,
    value_date             date,
    reference              text,
    notes                  text,
    tags                   text[] NOT NULL DEFAULT '{}',
    source                 text NOT NULL DEFAULT 'manual',
    raw                    jsonb,
    dedupe_hash            text,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX transactions_provider_idx ON transactions (user_id, provider, provider_transaction_id)
    WHERE provider_transaction_id IS NOT NULL;
CREATE UNIQUE INDEX transactions_dedupe_idx ON transactions (account_id, dedupe_hash)
    WHERE dedupe_hash IS NOT NULL;
CREATE INDEX transactions_user_date_idx ON transactions (user_id, occurred_at DESC);
CREATE INDEX transactions_account_date_idx ON transactions (account_id, occurred_at DESC);
CREATE INDEX transactions_category_idx ON transactions (category_id);

CREATE TABLE rules (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    priority    integer NOT NULL DEFAULT 100,
    category_id uuid NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    field       text NOT NULL DEFAULT 'description' CHECK (field IN ('description', 'merchant', 'amount', 'reference')),
    match_type  text NOT NULL DEFAULT 'contains' CHECK (match_type IN ('contains', 'equals', 'regex', 'gt', 'lt')),
    pattern     text NOT NULL,
    enabled     boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX rules_user_priority_idx ON rules (user_id, priority);

CREATE TABLE budgets (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id uuid REFERENCES categories(id) ON DELETE CASCADE,
    period      text NOT NULL DEFAULT 'monthly' CHECK (period IN ('monthly', 'weekly', 'yearly')),
    amount      numeric(20,4) NOT NULL CHECK (amount >= 0),
    currency    char(3) NOT NULL DEFAULT 'MXN',
    starts_on   date NOT NULL DEFAULT CURRENT_DATE,
    enabled     boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX budgets_user_overall_idx ON budgets (user_id, period) WHERE category_id IS NULL;
CREATE UNIQUE INDEX budgets_user_category_idx ON budgets (user_id, category_id, period) WHERE category_id IS NOT NULL;

CREATE TABLE ai_reports (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind         text NOT NULL,
    status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'error')),
    period_start date,
    period_end   date,
    model        text NOT NULL DEFAULT '',
    title        text NOT NULL DEFAULT '',
    content      text NOT NULL DEFAULT '',
    data         jsonb,
    error        text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX ai_reports_user_kind_idx ON ai_reports (user_id, kind, created_at DESC);

-- Postgres-backed job queue for background work (syncs, categorization, AI).
CREATE TABLE jobs (
    id           bigserial PRIMARY KEY,
    kind         text NOT NULL,
    payload      jsonb NOT NULL DEFAULT '{}',
    status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'done', 'failed', 'dead')),
    priority     integer NOT NULL DEFAULT 100,
    run_at       timestamptz NOT NULL DEFAULT now(),
    attempts     integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 5,
    locked_at    timestamptz,
    locked_by    text,
    last_error   text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz
);
CREATE INDEX jobs_pending_idx ON jobs (status, run_at) WHERE status = 'pending';
CREATE INDEX jobs_running_idx ON jobs (status, locked_at) WHERE status = 'running';

CREATE TABLE sync_runs (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    link_id            uuid REFERENCES links(id) ON DELETE CASCADE,
    status             text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'ok', 'error')),
    transactions_seen  integer NOT NULL DEFAULT 0,
    transactions_new   integer NOT NULL DEFAULT 0,
    error              text NOT NULL DEFAULT '',
    started_at         timestamptz NOT NULL DEFAULT now(),
    finished_at        timestamptz
);
CREATE INDEX sync_runs_link_idx ON sync_runs (link_id, started_at DESC);

-- Raw events received from providers (webhooks) for auditing and replays.
CREATE TABLE provider_events (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider     text NOT NULL,
    event_type   text NOT NULL DEFAULT '',
    payload      jsonb NOT NULL DEFAULT '{}',
    error        text NOT NULL DEFAULT '',
    received_at  timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz
);

-- Keep updated_at fresh on every update.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER users_set_updated_at      BEFORE UPDATE ON users      FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER links_set_updated_at      BEFORE UPDATE ON links      FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER accounts_set_updated_at   BEFORE UPDATE ON accounts   FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER categories_set_updated_at BEFORE UPDATE ON categories FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER transactions_set_updated_at BEFORE UPDATE ON transactions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER rules_set_updated_at      BEFORE UPDATE ON rules      FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER budgets_set_updated_at    BEFORE UPDATE ON budgets    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down

DROP TABLE IF EXISTS provider_events;
DROP TABLE IF EXISTS sync_runs;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS ai_reports;
DROP TABLE IF EXISTS budgets;
DROP TABLE IF EXISTS rules;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS links;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();
