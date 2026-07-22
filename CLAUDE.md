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
make test       # Run model tests (cd api && go test ./models/... -v)
make test-rate  # Run middleware tests (cd api && go test ./middleware/... -v)
make migrate    # Run database migrations via golang-migrate
make seed       # Load pre-generated LOTR slug pool into Postgres
make seed-dev   # Insert a test API key for local development
make logs       # Stream Docker Compose logs
make generate   # Re-run the NLP slug generation pipeline
```

Local setup flow: `cp .env.example .env` → fill in values → `make run` → `make seed` → `make seed-dev`

To run a single Go test:
```bash
cd api && go test ./... -run TestFunctionName
```

## Architecture

The system has three layers: a Go API, a PostgreSQL database, and a one-time NLP preprocessing pipeline.

### Go API (`api/`)

Organized into four packages:

- **`config/`** — Loads all env vars with defaults (DB URL, Redis URL, port, rate limits, TTL)
- **`db/`** — PostgreSQL pool (25 max open, retry on startup) and Redis client initialization
- **`models/`** — All database operations via a `URLStore` struct wrapping `*sql.DB`. Each file maps to a table: `url.go` (CRUD), `quote.go` (slug pool), `stats.go`/`clicks.go` (analytics), `api_key.go` (auth)
- **`handlers/`** — Gin handlers: `links.go` (create/delete), `redirect.go` (slug resolution + async click tracking), `stats.go` (analytics)
- **`middleware/`** — `auth.go` validates API keys against the DB; `rate.go` enforces IP-based (60/min) and API-key-based (120/min) limits using Redis counters

`main.go` wires everything together: loads config → connects DB/Redis → runs migrations → registers routes → starts server.

### Request Flows

**Redirect (`GET /:slug`):** Redis cache check → cache miss queries `urls` table → validates expiry/active status → caches result → async goroutine records click and increments count → 302 redirect.

**Create link (`POST /links`):** API key auth → rate limit check → select least-used slug from `quotes` table → insert into `urls` → increment `quotes.use_count` → return short URL.

**Delete (`DELETE /links/:slug`):** Soft-delete (sets `is_active = FALSE`) + Redis cache invalidation.

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
- `API_SECRET_KEY` — Reserved for secure key generation (currently unused by the API)
- `IP_RATE_LIMIT`, `KEY_RATE_LIMIT`, `DEFAULT_TTL_DAYS` — Tunable defaults
- `ANTHROPIC_API_KEY` — Only needed when running the NLP pipeline (`make generate`)
