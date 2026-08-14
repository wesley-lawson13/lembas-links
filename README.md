# Lembas Links

A Lord of the Rings themed URL shortener built in Go (Gin) with Redis caching, API key authentication middleware, and rate limiting.

> **Note:** The Railway deployment is no longer active as the free trial has ended. See [Getting Started](#getting-started) to run the project locally.

---

## Table of Contents
- [Features](#features)
- [Description, Project Objectives, and Future Plans](#description-project-outcomes-and-future-plans)
- [How it Works](#how-it-works)
- [Usage](#usage)
- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [API Reference](#api-reference)
- [Testing](#testing)
- [NLP Pipeline](#nlp-pipeline)
- [Deployment](#deployment)

---

## Features

- LOTR themed slugs generated from quote keyphrases
- Redis caching on all redirects
- API key authentication middleware on protected routes, with hashed key storage and a sliding TTL (an active key's expiry pushes forward on every authenticated request)
- Anonymous, self-serve session keys (`POST /session`) — no signup required to start creating links
- Per-caller link ownership: callers can only list, view stats for, or delete links created with their own key
- Redis-based rate limiting per IP and per API key, plus a stricter dedicated limit on session-key minting
- Click analytics: timestamp, referrer, user agent, and IP on every redirect asynchronously
- CORS support for browser-based frontends (configurable allowed origins)
- React frontend (Vite + TypeScript) with a LOTR-themed dashboard and per-link analytics — anonymous key minted silently on first visit, light/dark "Parchment/Rivendell" theme
- Interactive Swagger/OpenAPI docs served at `/swagger/index.html`
- Fully containerized with Docker Compose
- Automatic database migrations on startup

---

## Description, Project Objectives, and Future Plans

As a huge Lord of the Rings fan, I've always been looking for ways to incorporate my love for the fantasy franchise into different aspects of my life (seriously, I talk about it way too much). Thus, when I thought of Lembas Links, it felt like the perfect opportunity; Not only to make a LOTR themed project, but also to learn important backend programming principles and apply skills I've learned from my Natural Language Processing coursework at Boston College.

In building this Lembas Links, I gained hands-on, end-to-end experience designing and implementing a REST API in Go using Gin, a relevant backend framework. This experience really helped me understand how each component of the backend architecture interacts with one another, such as how authentication middleware, rate limiting, and the models layer interact, deepening my understanding of backend security and clean API design. Additionally, I gained valuable insight into important caching principles and practices through my use of Redis and how such technologies can improve performance. Lastly, I also developed practical skills in containerization with Docker Compose, building upon my previous experience using the technology.

Currently, my next goal is to learn AWS by redeploying the project there: first a manual EC2 deployment, then codifying that infrastructure with Terraform, then automating deployments with GitHub Actions, and finally adding observability. 

Thanks for checking out my Lembas Links repo! If you have any questions please feel free to get in touch.

---

## How It Works

Incoming requests hit the Go API which is built with Gin. The API first checks Redis and redirects on a cache hit, recording the click asynchronously. On a cache miss it queries Postgres, caches the result with a TTL, and redirects. Rate limiting is handled by Redis middleware that runs before every request, incrementing a count associated with both the user's API key and IP address. All persistent data lives in Postgres.

The NLP preprocessing pipeline is a separate tool that runs once to generate the slug pool. It reads a [LOTR movie script CSV](https://www.kaggle.com/datasets/paultimothymooney/lord-of-the-rings-data?select=lotr_scripts.csv), processes quotes through spaCy for keyword extraction and entity recognition, then calls the Claude Haiku API to generate slugs. The output is a SQL seed file that gets loaded into Postgres at setup time.

---

## Usage

First, follow the [Getting Started](#getting-started) steps to get the services running locally, then open the frontend at `http://localhost:5173`.

### Create a link

Paste a URL into the dashboard form and submit. An anonymous API key is minted for you silently on first visit — nothing to configure. A confirmation card shows your new short link (e.g. `gandalf-shadow-flame`) ready to copy.

### View your links

The dashboard's link list shows every link created with your key, with live click counts. It updates as you create new ones.

### Check a link's stats

Click through to a link's stats page (`/stats/:slug`) for its click count, a 7-day click chart, and a table of recent clicks (referrer, user agent, timestamp per click).

### Delete a link

Remove a link from the dashboard — this soft-deletes it and immediately invalidates the Redis cache, so the short URL stops resolving right away.

Toggle light/dark ("Parchment"/"Rivendell") theme from the header at any time; your choice is remembered in `localStorage`.

For direct API access — scripts, Postman, curl — see [API Reference](#api-reference) below, or the live Swagger UI at `/swagger/index.html`.

## Tech Stack

| Layer | Technology |
|---|---|
| API | Go 1.25, Gin |
| Frontend | React 19, TypeScript, Vite, react-router |
| Database | Postgres 15 |
| Cache + Rate Limiting | Redis 7 |
| NLP Pipeline | Python 3.11, spaCy, pandas, rapidfuzz, Claude Haiku API |
| Containerization | Docker, Docker Compose |
| Migrations | golang-migrate |

---

## Project Structure

Each part of the system has its own README with implementation details beyond what's covered here:

- `api/` — the Go API — see [`api/README.md`](api/README.md)
- `frontend/` — the React SPA — see [`frontend/README.md`](frontend/README.md)
- `nlp-service/` — the offline slug-generation pipeline — see [`nlp-service/README.md`](nlp-service/README.md)

---

## Getting Started

### Prerequisites
- Docker and Docker Compose
- Go 1.25+
- Python 3.11+ *(for NLP pipeline only)*

### 1. Clone the repo
```bash
git clone https://github.com/wesley-lawson13/lembas-links.git
cd lembas-links
```

### 2. Set up environment variables
```bash
cp .env.example .env
```
Fill in the required values — see [Environment Variables](#environment-variables) for details.

### 3. Start the services
```bash
make run
```

### 4. Seed the database
The API auto-loads the pre-generated LOTR themed slug pool (~340 slugs) into an empty `quotes` table on its first startup, so this step is usually optional. Run it explicitly if you want the pool loaded before the API's first request, or to force a reseed of a non-empty table:
```bash
make seed
```

### 5. Create an API key
Not required to try the app — opening the frontend (step 7) mints an anonymous key for you automatically. For manual/API testing:
```bash
make seed-dev
```
Inserts a fixed test API key (`test-api-key-123`) for local development. Alternatively, mint your own via `POST /session` — see [API Reference](#api-reference).

### 6. Verify everything is running
```bash
curl http://localhost:8080/health
```

### 7. Open the frontend

`make run` also starts the React frontend. Set up its env file once:

```bash
cp frontend/.env.example frontend/.env
```

then open `http://localhost:5173`. An anonymous API key is minted for you silently on first visit — no login. Make sure `CORS_ALLOWED_ORIGINS` in the root `.env` includes `http://localhost:5173` so the API accepts the browser's requests.

To run the frontend outside Docker instead: `cd frontend && npm install && npm run dev`. For more on the frontend's structure and dev workflow, see [`frontend/README.md`](frontend/README.md).

---

## API Reference

### Authentication

Protected endpoints require an API key passed in the `Authorization` header, optionally prefixed with `Bearer `:
    `Authorization: Bearer your-api-key`

Get a key via `make seed-dev`, `POST /session` below, or by opening the frontend (which mints one for you automatically). Keys are stored hashed (SHA-256) and their expiry slides forward by `DEFAULT_TTL_DAYS` on every authenticated request.

### Endpoints

#### `GET /health` — Public
Liveness check. Returns a static `200` payload (the field values are constants, not live probes of Postgres/Redis).

**Response `200`:**
```json
{
    "status": "ok",
    "service": "lembas-links",
    "database": "connected",
    "cache": "connected"
}
```

---

#### `POST /session` — Public
Mint a new anonymous API key. No login or signup required.

**Response `201`:**
```json
{
    "api_key": "9f2c...raw-key...",
    "expires_at": "2026-08-20T16:00:00Z"
}
```

Rate limited per IP — see [Rate Limits](#rate-limits).

| Status | Meaning |
|---|---|
| `201` | Key minted |
| `429` | Rate limit exceeded |
| `503` | Redis unavailable — the session limiter fails closed, so key minting is refused rather than left unmetered |

---

#### `POST /links` — Protected
Create a new Lord of the Rings link, owned by the calling API key.

**Request:**
```json
{
    "url": "https://your-long-url.com"
}
```

**Validation:** the target must be an absolute `http` or `https` URL that includes a host and is at most 2048 characters. Other schemes (`javascript:`, `data:`, `file:`, `mailto:`, …) are rejected so the shortener can't be turned into an open redirector.

**Response `201`:**
```json
{
    "slug": "gandalf-shadow-flame",
    "short_url": "http://localhost:8080/gandalf-shadow-flame",
    "original": "https://your-long-url.com"
}
```

| Status | Meaning |
|---|---|
| `201` | Link created |
| `400` | URL missing, malformed, over-length, or a disallowed scheme |
| `401` | Missing or invalid/expired API key |
| `429` | Rate limit exceeded |

---

#### `GET /links` — Protected
List every active link owned by the calling API key.

**Response `200`:**
```json
{
    "links": [
        {
            "slug": "gandalf-shadow-flame",
            "short_url": "http://localhost:8080/gandalf-shadow-flame",
            "original": "https://your-long-url.com",
            "click_count": 42,
            "created_at": "2026-04-06T16:00:00Z",
            "expires_at": "2026-05-06T16:00:00Z"
        }
    ]
}
```

| Status | Meaning |
|---|---|
| `200` | Links returned (empty array if the key owns none) |
| `401` | Missing or invalid/expired API key |
| `429` | Rate limit exceeded |

---

#### `GET /:slug` — Public
Redirect to the original URL. Checks Redis cache first, falls back to Postgres. Records click analytics asynchronously.

**Response:** `302` redirects to original URL

| Status | Meaning |
|---|---|
| `302` | Redirect successful |
| `404` | Slug not found or link inactive |
| `410` | Link has expired |
| `429` | Rate limit exceeded |

---

#### `GET /links/:slug/stats` — Protected
Get analytics for a Lord of the Rings link. Only the API key that created the link can view its stats; any other key (or a nonexistent slug) returns `404`. `daily_clicks` always contains exactly 7 entries (oldest → newest, UTC day buckets, zero-count days included) — it powers the frontend's 7-day chart.

**Response `200`:**
```json
{
    "slug": "gandalf-shadow-flame",
    "short_url": "http://localhost:8080/gandalf-shadow-flame",
    "original": "https://your-long-url.com",
    "click_count": 42,
    "created_at": "2026-04-06T16:00:00Z",
    "expires_at": "2026-05-06T16:00:00Z",
    "is_active": true,
    "daily_clicks": [
        { "date": "2026-04-09", "count": 12 }
    ],
    "recent_clicks": [
        {
            "id": "abc123",
            "slug": "gandalf-shadow-flame",
            "clicked_at": "2026-04-09T18:10:03Z",
            "referrer": "https://twitter.com",
            "user_agent": "Mozilla/5.0...",
            "ip_address": "192.168.1.1"
        }
    ]
}
```

| Status | Meaning |
|---|---|
| `200` | Stats returned |
| `401` | Missing or invalid/expired API key |
| `404` | Slug not found, inactive, expired, or owned by another key |
| `429` | Rate limit exceeded |

---

#### `DELETE /links/:slug` — Protected
Soft delete a Lord of the Rings link. Immediately invalidates Redis cache.

**Response:** `204 No Content`

| Status | Meaning |
|---|---|
| `204` | Link soft-deleted |
| `401` | Missing or invalid/expired API key |
| `404` | Slug not found, or owned by another key (same response, to avoid leaking existence) |
| `429` | Rate limit exceeded |

---

### Rate Limits

| Type | Limit |
|---|---|
| Per IP | 60 requests/minute |
| Per API Key | 120 requests/minute |
| `POST /session` (per IP) | 5 requests/hour |

Exceeding limits returns `429 Too Many Requests`.

---

## Testing

Most tests are unit tests with no external dependencies. A few (in `models`, `middleware`, and `handlers`) are integration tests that talk to a real Postgres/Redis instance and are skipped automatically unless the relevant test connection string is set.

### Running tests

```bash
make test-models    # cd api && go test ./models/... -v
make test-rate      # cd api && go test ./middleware/... -v
make test-handlers  # cd api && go test ./handlers/... -v
```

To run the full suite (all packages):
```bash
make test-all       # cd api && go test ./... -v
```

To run a single test:
```bash
cd api && go test ./... -run TestFunctionName
```

### Enabling integration tests

Integration tests check for `TEST_DATABASE_URL` (Postgres) and `TEST_REDIS_URL` (Redis) and call `t.Skip` per-case if unset — so without Docker running, `make test-models`, `make test-rate`, `make test-handlers`, and `make test-all` will all report as passing while quietly skipping their integration cases. `test-models` needs Postgres; `test-rate` and `test-handlers` need both Postgres and Redis. With `make run` already up (or just `docker compose up postgres redis` if you don't need the API/frontend containers), the compose services are reachable from the host, so you can point both at localhost:

```bash
# in .env
TEST_DATABASE_URL=postgres://<user>:<password>@localhost:5432/<db>?sslmode=disable
TEST_REDIS_URL=redis://localhost:6379
```

### End-to-end smoke test

The Go suites test packages directly, not the running server. `scripts/e2e_smoke_test.sh` covers that gap: it runs curl against a live compose stack (start it with `make run` first) and asserts the expected status code across the full backend flow — health, session key minting, link create/list/stats, ownership isolation between keys, the public redirect, soft-delete, the session-mint rate limit, and the CORS preflight.

```bash
make e2e
```

The script fails fast on the first unexpected status and prints which step broke. It resets the `rate:session:*` counters in Redis before minting steps, so it's safe to rerun back-to-back. Configurable via env vars: `API_URL` (default `http://localhost:8080`), `ORIGIN` (default `http://localhost:5173`, must be in `CORS_ALLOWED_ORIGINS`), and `SESSION_RATE_LIMIT` (default 5, must match the API's setting).

### NLP pipeline tests

The NLP pipeline has its own pytest suite, separate from the Go tests above:

```bash
make test-nlp   # venv activated — cd nlp-service && pytest
```

See [`nlp-service/README.md`](nlp-service/README.md#running-the-tests) for setup details.

---

## NLP Pipeline

The slug generation pipeline runs offline as a one-time preprocessing step and is not part of the running application. For implementation details and its test suite, see [`nlp-service/README.md`](nlp-service/README.md).

### How It Works

1. **Data loading** — reads the [LOTR movie script dataset](https://www.kaggle.com/datasets/paultimothymooney/lord-of-the-rings-data?select=lotr_scripts.csv) (~2,000 quotes) from a csv file
2. **Preprocessing** — cleans text, filters by character relevance and quote quality using spaCy
3. **Scoring** — ranks quotes by keyword richness and named entity density
4. **Famous quote detection** — fuzzy matches against a curated list of 'famous' quotes using rapidfuzz, ensuring they always make it into the pool regardless of score
5. **Slug generation** — sends enriched quote data to Claude Haiku API with extracted keywords and named entities, generating memorable 2-3 word hyphenated slugs
6. **Collision handling** — avoids duplicate slugs using an in-memory set
7. **Output** — writes `db/seeds/quotes.sql` with LOTR themed slugs ready to seed!

### Regenerating the Slug Pool 

```bash
cd nlp-service
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python -m spacy download en_core_web_sm
cp .env.example .env  # add your ANTHROPIC_API_KEY
cd ..
make generate
cp nlp-service/data/quotes.sql db/seeds/quotes.sql
```

---

## Deployment

This project was previously deployed on Railway. The deployment is no longer active as the free trial has ended. See [Getting Started](#getting-started) to run locally.

### Environment Variables

| Variable | Description | Default |
|---|---|---|
| `POSTGRES_USER` | Postgres username (used by the `postgres` container and `make seed`/`seed-dev`/`migrate`) | required |
| `POSTGRES_PASSWORD` | Postgres password | required |
| `POSTGRES_DB` | Postgres database name | required |
| `DATABASE_URL` | Postgres connection string used by the API | required |
| `REDIS_URL` | Redis connection string | required |
| `API_PORT` | Port to run the API on | `8080` |
| `BASE_URL` | Base URL for short links | required |
| `IP_RATE_LIMIT` | Requests per minute per IP | `60` |
| `KEY_RATE_LIMIT` | Requests per minute per API key | `120` |
| `RATE_LIMIT_WINDOW` | Rate limit window, in seconds | `60` |
| `DEFAULT_TTL_DAYS` | Default link expiry in days; also the sliding-TTL window applied to API keys on each authenticated request | `30` |
| `SESSION_RATE_LIMIT` | Max `POST /session` requests per IP per window | `5` |
| `SESSION_RATE_LIMIT_WINDOW` | `POST /session` rate limit window, in seconds | `3600` |
| `CORS_ALLOWED_ORIGINS` | Comma-separated list of allowed browser origins (e.g. `http://localhost:5173`). CORS middleware is skipped entirely if unset | optional |
| `RECENT_CLICKS_LIMIT` | Number of recent clicks returned by the stats endpoint | `10` |
| `TEST_DATABASE_URL` | Postgres connection string used by `go test` for integration tests; tests are skipped if unset | optional |
| `TEST_REDIS_URL` | Redis connection string used by `go test` for integration tests; tests are skipped if unset | optional |

---

## License

MIT
