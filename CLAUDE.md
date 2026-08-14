# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Lembas Links is a Lord of the Rings-themed URL shortener. The system assigns memorable LOTR-themed slugs (sourced from movie quotes) to shortened URLs, with Redis caching, API key authentication, rate limiting, and click analytics.

## Commands

All common operations are in the `Makefile`:

```bash
make run        # Start all services via Docker Compose (builds first)
make stop       # Stop all services
make build      # Build Docker images only
make test-models   # Run model tests (cd api && go test ./models/... -v)
make test-rate     # Run middleware tests (cd api && go test ./middleware/... -v)
make test-handlers # Run handler tests (cd api && go test ./handlers/... -v)
make test-all      # Run every Go package (cd api && go test ./... -v)
make migrate    # Run database migrations via golang-migrate
make seed       # Load pre-generated LOTR slug pool into Postgres
make seed-dev   # Insert a test API key for local development
make logs       # Stream Docker Compose logs
make generate   # Re-run the NLP slug generation pipeline
```

The four `test-*` targets run integration cases (`test-models` needs Postgres; `test-rate` and `test-handlers` need both Postgres and Redis) that are skipped per-case unless `TEST_DATABASE_URL`/`TEST_REDIS_URL` are set — so to actually exercise them rather than skip, the dependencies need to be running first (`make run`, or just `docker compose up postgres redis`), with those two env vars pointed at localhost.

Local setup flow: `cp .env.example .env` → fill in values → `cp frontend/.env.example frontend/.env` → `make run` → `make seed` → `make seed-dev`

Frontend dev outside Docker: `cd frontend && npm install && npm run dev` (Vite dev server on `http://localhost:5173`; `npm run build` type-checks and bundles).

To run a single Go test:
```bash
cd api && go test ./... -run TestFunctionName
```

## Architecture

The system has four layers: a Go API, a PostgreSQL database, a React frontend, and a one-time NLP preprocessing pipeline.

### Go API (`api/`)

Organized into four packages:

- **`config/`** — Loads all env vars with defaults (DB URL, Redis URL, port, rate limits, TTL)
- **`db/`** — PostgreSQL pool (25 max open, retry on startup) and Redis client initialization
- **`models/`** — All database operations via a `URLStore` struct wrapping `*sql.DB`. Each file maps to a table: `url.go` (CRUD + slug allocation from the quote pool), `stats.go`/`clicks.go` (analytics), `api_key.go` (auth)
- **`handlers/`** — Gin handlers: `links.go` (create/delete), `redirect.go` (slug resolution + async click tracking), `stats.go` (analytics)
- **`middleware/`** — `auth.go` validates API keys against the DB; `rate.go` enforces IP-based (60/min) and API-key-based (120/min) limits using Redis counters

`main.go` wires everything together: loads config → connects DB/Redis → runs migrations → registers routes → starts server.

### Request Flows

**Redirect (`GET /:slug`):** Redis cache check → cache miss queries `urls` table → validates expiry/active status → caches result → async goroutine records click and increments count → 302 redirect.

**Create link (`POST /links`):** API key auth → rate limit check → select least-used slug from `quotes` table → insert into `urls` → increment `quotes.use_count` → return short URL.

**Delete (`DELETE /links/:slug`):** Soft-delete (sets `is_active = FALSE`) + Redis cache invalidation.

### Frontend (`frontend/`)

Vite + React 19 + TypeScript SPA, served by the Vite dev server (compose service on port 5173), calling the API directly over CORS. No accounts: `src/api.ts` silently mints an anonymous key via `POST /session` on first use, caches it in localStorage (`lembas_api_key`), sends it as `Authorization: Bearer`, and on a 401 discards it, re-mints, and retries once. All fetches go through one typed `request<T>` core; response shapes are mirrored in `src/types.ts` and the frontend never constructs short URLs (always renders `short_url` from responses).

- **Routes** (react-router): `/` → `pages/Dashboard.tsx` (forge form, freshly-forged card, fellowship list); `/stats/:slug` → `pages/Stats.tsx` (metrics, 7-day chart from `daily_clicks`, recent passages table)
- **Theming** — two palettes (Parchment light / Rivendell dark) as CSS custom properties in `src/styles/theme.css`, keyed off `data-theme` on `<html>`: set pre-paint by an inline script in `index.html` (localStorage `lembas_theme` override, else `prefers-color-scheme`), toggled via `src/useTheme.ts`. All colors go through the variables — never hardcode
- **Styles** — plain CSS: `app.css` (shared shell/buttons/cards), `dashboard.css`/`stats.css` (per-page); no CSS framework
- **`src/format.ts`** — presentation helpers (expiry countdowns, date/timestamp formats, user-agent + referrer summarizers)
- `frontend/.env.example` — `VITE_API_BASE_URL` (browser-facing API origin; the API's `CORS_ALLOWED_ORIGINS` must include the frontend origin)

### Database Schema

Four tables:
- **`urls`** — `(id UUID, slug UNIQUE, original, api_key, click_count, expires_at, is_active)`
- **`quotes`** — LOTR slug pool `(slug UNIQUE, quote, character, source, use_count)` — seeded once, never changed by the API except incrementing `use_count`
- **`api_keys`** — `(key UNIQUE, name)`
- **`clicks`** — `(slug, clicked_at, referrer, user_agent, ip_address)` for analytics

Migrations live in `db/migrations/` and run automatically on API startup.

### NLP Service (`nlp-service/`)

A one-time Python pipeline, not part of the live API. Run via `make generate` when the slug pool needs to be regenerated. The pipeline:

1. `nlp_preprocess.py` — Processes LOTR movie script CSV with spaCy (NER, keyword extraction, scoring)
2. `slug_generator.py` — Calls Claude Haiku to generate 2-3 word hyphenated slugs from enriched quote data
3. `generate_slugs.py` — Orchestrates the pipeline and writes `db/seeds/quotes.sql`

The output (`db/seeds/quotes.sql`) is committed to the repo so the NLP pipeline doesn't need to run in production.

## Development Conventions

Before writing a new function, write a 1-2 sentence description of what it does and where it gets called from.

### Commit Messages

- **Subject line**: a single, descriptive partial-sentence (not a full sentence, no trailing period) covering only the core change of the commit.
- **Body**: separated from the subject by a blank line, with two parts:
  1. Specific details on what changed (files, behaviors, edge cases handled).
  2. An overarching "why" — the reasoning or context behind why the change was made this way.

## Environment Variables

See `.env.example` for the full list. Key variables:
- `DATABASE_URL` / `REDIS_URL` — Connection strings
- `BASE_URL` — The domain used to construct short URLs
- `IP_RATE_LIMIT`, `KEY_RATE_LIMIT`, `DEFAULT_TTL_DAYS` — Tunable defaults
- `ANTHROPIC_API_KEY` — Only needed when running the NLP pipeline (`make generate`)
