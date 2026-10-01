# okane

Personal finance tracker: expenses and income from your bank accounts, spending
by category, and AI insights — on the web and on iPhone.

> Status: **foundation**. Auth, database schema and deploy plumbing are in
> place; accounts, transactions, bank sync and AI insights are next (see the
> roadmap below).

## Stack

| Piece      | Choice                                                                 |
| ---------- | ---------------------------------------------------------------------- |
| API        | Go 1.27, chi, pgx, Postgres 17, goose migrations (embedded in the binary) |
| Web        | React 19 + Vite + Tailwind CSS 4 + TanStack Query                      |
| iOS        | SwiftUI app talking to the same API (planned)                          |
| Banks      | [Belvo](https://belvo.com) aggregation (Mexico/LatAm), sandbox first   |
| AI         | OpenCode Inference (OpenAI-compatible) with your Console key           |
| Deployment | Docker Compose on a VPS: Postgres + API + Caddy (TLS) + daily backups  |

Money is stored as `numeric(20,4)` in Postgres and handled as decimals, never
floats.

## Layout

```
cmd/okane/                CLI entrypoint: serve, migrate, version
internal/config/          Environment configuration
internal/db/              Pool, migrations (embedded SQL), integration test
internal/domain/          Core data model
internal/money/           Decimal money type (safe arithmetic + JSON)
internal/auth/            Passwords (argon2id), sessions, service, store
internal/httpapi/         Router, middleware, handlers
web/                      React SPA
deploy/                   Production compose, Dockerfiles, Caddyfile
```

## Quickstart (development)

Requirements: Go 1.27+, Node 24+, Docker (or any Postgres 17).

```sh
cp .env.example .env
make db-up          # start Postgres in Docker
make run            # API on :8080, applies migrations on startup

cd web && npm install && npm run dev   # web on :5173, proxies /api to :8080
```

Create an account from the web UI (the first sign-up), or with curl:

```sh
curl -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"me@example.com","password":"password123","name":"Me"}'
```

The API returns a session token (for the iOS app) and sets an HttpOnly cookie
(for the web app).

## API

| Method | Path                        | Description                          |
| ------ | --------------------------- | ------------------------------------ |
| GET    | `/healthz`                  | Liveness                             |
| GET    | `/readyz`                   | Readiness (pings Postgres)           |
| POST   | `/api/v1/auth/register`     | Create account + session             |
| POST   | `/api/v1/auth/login`        | Sign in                              |
| POST   | `/api/v1/auth/logout`       | Revoke current session               |
| GET    | `/api/v1/auth/me`           | Current user                         |
| GET    | `/api/v1/accounts`          | List accounts with live balances     |
| POST   | `/api/v1/accounts`          | Create a manual account              |
| PATCH  | `/api/v1/accounts/{id}`     | Update account                       |
| DELETE | `/api/v1/accounts/{id}`     | Delete account (cascades its transactions) |
| GET    | `/api/v1/categories`        | List system + user categories        |
| GET    | `/api/v1/transactions`      | List/filter (`account_id`, `category_id`, `direction`, `search`, `from`, `to`, `limit`, `offset`) |
| POST   | `/api/v1/transactions`      | Create transaction (amount is positive; direction sets sign) |
| PATCH  | `/api/v1/transactions/{id}` | Update transaction (`null` clears category/merchant/notes) |
| DELETE | `/api/v1/transactions/{id}` | Delete transaction                   |
| GET    | `/api/v1/summary`           | Income/expenses/net/net worth + by category for a period |
| GET    | `/api/v1/summary/months`    | Monthly totals (`?months=6`, 1–24)   |

Authenticate with either `Authorization: Bearer <token>` (iOS) or the
`okane_session` cookie (web). State-changing cookie requests are protected by
an Origin check on top of `SameSite=Lax`.

## Configuration

All configuration is environment-based; see `.env.example` for the full list.
Highlights:

| Variable                      | Default                                             | Notes                              |
| ----------------------------- | --------------------------------------------------- | ---------------------------------- |
| `OKANE_DATABASE_URL`          | —                                                   | Required                           |
| `OKANE_APP_URL`               | `http://localhost:5173`                             | Web origin (CORS + CSRF origin)    |
| `OKANE_SESSION_TTL`           | `720h`                                              | Session lifetime                   |
| `OKANE_SESSION_COOKIE_SECURE` | `false`                                             | Set `true` behind HTTPS            |
| `OKANE_TIMEZONE`              | `America/Mexico_City`                               | IANA timezone for month boundaries |
| `OKANE_BELVO_ENV`             | `sandbox`                                           | `sandbox` or `production`          |
| `OKANE_AI_BASE_URL`           | `https://opencode.ai/inference/openai/v1`           | OpenAI-compatible endpoint         |
| `OKANE_AI_API_KEY`            | —                                                   | OpenCode Console service key       |

## Testing

```sh
make test        # unit tests
make db-up       # then, for the database integration test:
OKANE_DATABASE_URL="postgres://okane:okane@localhost:5432/okane?sslmode=disable" go test ./...
```

## Deployment (VPS)

1. Point a DNS `A` record at the VPS.
2. Copy `deploy/.env.example` to `deploy/.env` and fill it in.
3. On the VPS:

```sh
docker compose -f deploy/compose.yaml --env-file deploy/.env up -d --build
```

Caddy provisions TLS automatically. For a local rehearsal, `OKANE_DOMAIN=localhost`
serves `https://localhost` with a self-signed certificate (curl needs `-k`).

A `backup` service writes a gzipped `pg_dump` to `deploy/backups/` every 24
hours and keeps 14 days.

Migrations run automatically when the API starts (protected by a Postgres
advisory lock, so it is safe to restart or scale).

## Roadmap

- [x] Foundation: repo, schema, migrations, config, auth, web shell, deploy
- [x] Accounts + transactions: CRUD, categories, dashboard (rules TBD)
- [ ] Belvo: link flow (sandbox `erebus` first), sync worker, webhooks, dedupe
- [ ] CSV/OFX import so the app is useful without aggregation
- [ ] AI: categorization fallback, monthly digest, anomalies, Q&A over your data
- [ ] iOS: SwiftUI app (login, dashboard, transactions, bank linking)
- [ ] Budgets and recurring-charge detection
- [ ] Push notifications / weekly email digest

## Notes

- Belvo sandbox is free and includes fake Mexican banks, so the full sync flow
  can be developed without real credentials. Production access requires Belvo
  to enable it for your account.
- On Linux, if `docker` says permission denied, log out and back in (or use
  `newgrp docker`) after being added to the `docker` group.
