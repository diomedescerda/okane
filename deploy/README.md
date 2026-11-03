# Deploying Okane for free (Render + Neon)

The app ships as a single container with the React front end embedded in the
Go binary, so one free web service + one free Postgres is a complete deployment.
Verified vendor terms (Oct 2026):

- **Render free web service**: 750 instance-hours/month (enough to run one
  service around the clock), spins down after 15 minutes without traffic and
  wakes in about one minute on the next request.
- **Neon free Postgres**: permanent free plan, no card. 1 GB storage, compute
  scales to zero after 5 minutes of inactivity and wakes in milliseconds;
  100 CU-hours of compute per project per month.

## 1. Neon — create the database

1. Sign up at <https://neon.com> (no card needed) and create a project
   (`okane`, cloud region near you, Postgres 17 or newer).
2. Copy the **connection string** from the project dashboard. Use the direct
   (non-pooled) endpoint for the API; it looks like:
   `postgresql://okane:<PASSWORD>@ep-xxxx.<region>.aws.neon.tech/neondb?sslmode=require`
3. Optional: create the local **demo data / your local data** now (see
   "Migrating local data" below) or rely on `okane seed` later.

## 2. Render — create the web service

1. Sign up at <https://render.com> and use **New → Web Service**, deploying
   from this GitHub repository.
2. Settings:
   - **Runtime**: Docker
   - **Dockerfile path**: `deploy/Dockerfile.app`
   - **Health check path**: `/readyz`
   - **Instance type**: Free
3. Environment variables:

   | Key | Value |
   | --- | --- |
   | `OKANE_ENV` | `prod` |
   | `OKANE_ADDR` | `:$PORT` |
   | `OKANE_APP_URL` | `https://<your-service>.onrender.com` (set after the first deploy gives you the URL) |
   | `OKANE_DATABASE_URL` | the Neon connection string |
   | `OKANE_TIMEZONE` | `America/Bogota` |
   | `OKANE_JOBS_POLL` | `15m` — keeps Neon's free compute mostly asleep between queue checks; import-triggered jobs still run instantly via the in-process kick |
   | `OKANE_AI_API_KEY` | optional; enables AI categorization |

   `OKANE_SESSION_COOKIE_SECURE` defaults to `true` in prod — don't set it.

4. First deploy builds the web front end inside `deploy/Dockerfile.app`, embeds
   it in the binary, and `okane serve` applies migrations on boot.
5. Once the service is live, copy its URL into `OKANE_APP_URL` and redeploy.

## 3. Seed or migrate data

From your machine, pointing at Neon:

```sh
# demo dataset (idempotent)
OKANE_DATABASE_URL='<neon dsn>' go run ./cmd/okane seed

# or restore your local data: dump from the local container...
docker exec okane-postgres-1 pg_dump -U okane -d okane --clean --if-exists > /tmp/opencode/okane-dump.sql

# ...and replay it into Neon (psql runs inside a throwaway container)
docker run --rm -i postgres:17-alpine \
  psql '<neon dsn>' < /tmp/opencode/okane-dump.sql
```

## Notes and gotchas

- **Render cold starts**: after 15 minutes idle the first page load waits
  ~1 minute. Personal use tolerates it; upgrade ($7/mo) or add a cron ping to
  avoid it.
- **Neon compute budget**: with `OKANE_JOBS_POLL=15m` the database stays awake
  roughly a third of the month — well under the 100 CU-hours allowance. Do not
  set the poll below `5m` on the Neon free tier.
- **Why one service**: the SPA is embedded (`internal/webui/webui.go`), so the
  API and the front end share one origin. Cookies and CSRF need no extra
  configuration; the front end calls `/api/v1/...` relative to its own origin,
  no build-time URL variable required.
- **VITE_API_BASE**: leave it unset — the front end calls `/api/v1/...` on its
  own origin.
