# Lembas Links

A Lord of the Rings themed URL shortener built in Go (Gin) with Redis caching, API key authentication middleware, and rate limiting.

![Lembas Links Dashboard](images/Lembas-Links.png)

Thanks for checking out my Lembas Links repo! If you have any questions please feel free to get in touch.

---

## Table of Contents
- [Project Description](#project-description)
  - [Features](#features)
  - [How it Works](#how-it-works)
  - [Project Structure](#project-structure)
- [Learning Objectives and Future Plans](#learning-objectives-and-future-plans)
- [Tech Stack](#tech-stack)
- [Getting Started](#getting-started)
- [Usage](#usage)
- [API Reference](#api-reference)
- [Testing](#testing)
- [NLP Pipeline](#nlp-pipeline)
- [Environment Variables](#environment-variables)
- [License](#license)

---

## Project Description

As a huge Lord of the Rings fan, I've always been looking for ways to incorporate my love for the fantasy franchise into different aspects of my life (seriously, I talk about it way too much). Thus, when I thought of Lembas Links, it felt like the perfect opportunity; Not only to make a LOTR themed project, but also to learn important backend programming principles and apply skills I've learned from my Natural Language Processing coursework at Boston College.

### Features

- LOTR themed slugs generated from quote keyphrases
- Redis caching on all redirects, plus per-IP and per-API-key rate limiting
- API key authentication middleware, with anonymous self-serve keys (`POST /session`) so there's no signup to get started
- Per-caller link ownership — callers only see and manage links created with their own key
- Click analytics recorded asynchronously on every redirect (timestamp, referrer, user agent, IP)
- React frontend (Vite + TypeScript) with a LOTR-themed dashboard, per-link analytics, and a light/dark "Parchment/Rivendell" theme
- Interactive Swagger/OpenAPI docs at `/swagger/index.html`
- Fully containerized with Docker Compose, with database migrations applied automatically on startup

### How It Works

Incoming requests hit the Go API which is built with Gin. The API first checks Redis and redirects on a cache hit, recording the click asynchronously. On a cache miss it queries Postgres, caches the result with a TTL, and redirects. Rate limiting is handled by Redis middleware that runs before every request, incrementing a count associated with both the user's API key and IP address. All persistent data lives in Postgres.

The slug pool itself is produced ahead of time by a separate NLP pipeline — see [NLP Pipeline](#nlp-pipeline).

### Project Structure

Each part of the system has its own README with implementation details beyond what's covered here:

- `api/` — the Go API, including the full endpoint reference — see [`api/README.md`](api/README.md)
- `frontend/` — the React SPA — see [`frontend/README.md`](frontend/README.md)
- `nlp-service/` — the offline slug-generation pipeline — see [`nlp-service/README.md`](nlp-service/README.md)

---

## Learning Objectives and Future Plans

In building this Lembas Links, I set out to gain hands-on, end-to-end experience designing and implementing a REST API in Go using Gin, a relevant backend framework. This experience really helped me understand how each component of the backend architecture interacts with one another, such as how authentication middleware, rate limiting, and the models layer interact, deepening my understanding of backend security and clean API design. Additionally, I gained valuable insight into important caching principles and practices through my use of Redis and how such technologies can improve performance. I also developed practical skills in containerization with Docker Compose, building upon my previous experience using the technology.

Lembas Links also acted as a learning ground for me to develop strong agentic coding principles and practices. Though it was initially designed as a standalone API, I restructured Lembas Links with the help of Claude Code, utilizing it as a collaborator that I had to direct rather than follow blindly. This meant drafting highly-descriptive, actionable plans that matched my vision for the project, scoping work before writing it, keeping a project-level `CLAUDE.md` as shared context, and reviewing every change as carefully as I would a teammate's pull request.

Thus, over the course of developing this project, I feel I've been able to foster the following skills:

- **AI-assisted development** — planning, `CLAUDE.md` creation and customization, and close code reviews
- **Parallel feature development** across Git worktrees, plus browser-driven debugging through the Chrome MCP server
- **Issue-to-PR workflow** — scoped branches and detailed commit messages, matching standard industry practice
- **Testing at every layer** — Go unit tests, integration tests against real Postgres and Redis, and an end-to-end smoke test over live HTTP
- **System design** — utilizing caching effectively, rate limiting, schema and index modeling, and sliding window authentication expiration
- **Practical NLP** — spaCy NER and keyword extraction feeding an LLM to generate the slug pool ahead of time, regenerable in a single command
- **Full-stack delivery** — extending a backend-only service into a typed React SPA, including CORS, anonymous session keys, and a well-defined API contract

Currently, my next goal is to learn AWS by redeploying the project there: first a manual EC2 deployment, then codifying that infrastructure with Terraform, then automating deployments with GitHub Actions, and finally adding observability.

---

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

`.env.example` already ships working defaults for everything else, so there are
only **three values to fill in** — the Postgres credentials. Any values you
like; they create the database on first boot:

```bash
POSTGRES_USER=lembas
POSTGRES_PASSWORD=your-password-here
POSTGRES_DB=lembas_links
```

Then substitute those same three credentials into the `DATABASE_URL` placeholders,
leaving the `postgres:5432` host **and the `?sslmode=disable` suffix** as-is:

```bash
DATABASE_URL=postgres://lembas:your-password-here@postgres:5432/lembas_links?sslmode=disable
```

Leave the rest as they are.

Two things worth knowing:
- `DATABASE_URL` and `REDIS_URL` are the only variables the API refuses to
  start without. `BASE_URL` isn't validated, so if you blank it out the API
  boots fine but returns malformed short links like `/gandalf-shadow-flame`
  instead of `http://localhost:8080/gandalf-shadow-flame`. Deploying anywhere
  other than localhost means updating it to the API's public origin.
- `TEST_DATABASE_URL`/`TEST_REDIS_URL` stay empty unless you're running the
  integration tests — see [Testing](#testing). These also need
  `?sslmode=disable`, and point at `localhost` rather than the Compose
  hostnames.
- If `make run` logs `Attempt N/10 failed: pq: SSL is not enabled on the
  server` and the API exits, `?sslmode=disable` is missing from your
  `DATABASE_URL`. The `lib/pq` driver defaults to requiring SSL; the Postgres
  container doesn't serve it.

Full descriptions and defaults for all 17 variables are in
[Environment Variables](#environment-variables). Regenerating the slug pool
needs a separate `ANTHROPIC_API_KEY` in `nlp-service/.env`, but that isn't part
of normal setup — see [NLP Pipeline](#nlp-pipeline).

### 3. Start the services
```bash
make run
```

### 4. Seed the database (Optional)
On startup the API runs migrations, then loads the pre-generated LOTR themed
slug pool (~340 slugs) into the `quotes` table if that table is empty — so a
normal `make run` needs no seeding step. Check the logs for `Quotes table seeded successfully`,
and if it's missing, seed manually against the running Postgres container:
```bash
make seed
```

### 5. Create an API key (Optional)
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

---

## API Reference

Protected endpoints require an API key in the `Authorization` header, optionally prefixed with `Bearer `. Get one via `make seed-dev`, `POST /session`, or by opening the frontend (which mints one for you automatically).

| Type | Rate Limit |
|---|---|
| Per IP | 60 requests/minute |
| Per API Key | 120 requests/minute |
| `POST /session` (per IP) | 5 requests/hour |

For the full endpoint contract — every route with its request/response shapes and status codes — see [`api/README.md`](api/README.md#api-reference), or browse the interactive Swagger UI at `/swagger/index.html` while the stack is running.

---

## Testing

```bash
make test-all   # every Go package
make e2e        # end-to-end smoke test with curl against a running stack
```

The `models`, `middleware`, and `handlers` packages contain integration tests that talk to a real Postgres/Redis instance. These skip themselves per-case unless `TEST_DATABASE_URL`/`TEST_REDIS_URL` are set — so without those, `make test-all` reports as passing while quietly skipping them.

For per-package targets, integration test setup, and the e2e script's options, see [`api/README.md`](api/README.md#testing). The NLP pipeline has its own pytest suite — see [`nlp-service/README.md`](nlp-service/README.md#running-the-tests).

---

## NLP Pipeline

The slug generation pipeline runs offline as a one-time preprocessing step and is not part of the running application. It reads a [LOTR movie script CSV](https://www.kaggle.com/datasets/paultimothymooney/lord-of-the-rings-data?select=lotr_scripts.csv), processes quotes through spaCy for keyword extraction and entity recognition, then calls the Claude Haiku API to generate slugs. The output is a SQL seed file that gets loaded into Postgres at setup time.

For the full pipeline breakdown, regeneration steps, and its test suite, see [`nlp-service/README.md`](nlp-service/README.md).

---

## Environment Variables

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

The `frontend/` and `nlp-service/` directories each have their own `.env` — see [`frontend/README.md`](frontend/README.md) and [`nlp-service/README.md`](nlp-service/README.md#environment-variables).

---

## License

MIT
