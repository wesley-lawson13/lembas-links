# Lembas Links — API

Go 1.25 + Gin REST API for the URL shortener. Redirects check Redis first, fall
back to Postgres on a cache miss, then cache the result and record the click
asynchronously; rate limiting and API key auth run as Gin middleware in front
of the protected routes. This file is the endpoint contract — for project setup
and running the whole stack, see the root [`README.md`](../README.md#getting-started).

### Structure

- `config/` — `Config` struct + `Load()`, reads all env vars with defaults,
  fatals if `DATABASE_URL`/`REDIS_URL` are missing
- `db/` — Postgres pool (`db.go`, 25 max open connections, retries on
  startup) and Redis client (`redis.go`) initialization
- `migrate/` — runs `golang-migrate` migrations and seeds the `quotes` table
  on first startup if it's empty
- `models/` — `URLStore` wrapping `*sql.DB`, the shared data-access layer;
  one file per table (`url.go`, `api_key.go`, `clicks.go`, `stats.go`) plus
  shared struct definitions in `store.go`
- `handlers/` — Gin handlers: `links.go` (create/list/delete), `redirect.go`
  (slug resolution + async click tracking), `session.go` (anonymous key
  minting), `stats.go` (analytics), `validate.go` (target URL validation),
  `types.go` (request/response DTOs)
- `middleware/` — `auth.go` (API key auth), `bearer.go` (shared bearer-token
  extraction), `rate.go` (IP- and key-based rate limiting)
- `docs/` — generated Swagger/OpenAPI output (`swag init`), not hand-maintained
- `main.go` — wires config → DB/Redis → migrations → routes → server

Environment variables for the API are documented in the root
[`README.md`](../README.md#environment-variables) — they live in the root
`.env`, which Docker Compose and the `Makefile` targets share.

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

An interactive Swagger/OpenAPI version of everything above is served at
`/swagger/index.html` while the stack is running.

## Testing

Most tests are unit tests with no external dependencies. A few (in `models`,
`middleware`, and `handlers`) are integration tests that talk to a real
Postgres/Redis instance and are skipped automatically unless the relevant test
connection string is set.

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

Integration tests check for `TEST_DATABASE_URL` (Postgres) and `TEST_REDIS_URL`
(Redis) and call `t.Skip` per-case if unset — so without Docker running,
`make test-models`, `make test-rate`, `make test-handlers`, and `make test-all`
will all report as passing while quietly skipping their integration cases.
`test-models` needs Postgres; `test-rate` and `test-handlers` need both
Postgres and Redis. With `make run` already up (or just
`docker compose up postgres redis` if you don't need the API/frontend
containers), the compose services are reachable from the host, so you can point
both at localhost:

```bash
# in .env
TEST_DATABASE_URL=postgres://<user>:<password>@localhost:5432/<db>?sslmode=disable
TEST_REDIS_URL=redis://localhost:6379
```

### End-to-end smoke test

The Go suites test packages directly, not the running server.
`scripts/e2e_smoke_test.sh` covers that gap: it runs curl against a live compose
stack (start it with `make run` first) and asserts the expected status code
across the full backend flow — health, session key minting, link
create/list/stats, ownership isolation between keys, the public redirect,
soft-delete, the session-mint rate limit, and the CORS preflight.

```bash
make e2e
```

The script fails fast on the first unexpected status and prints which step
broke. It resets the `rate:session:*` counters in Redis before minting steps,
so it's safe to rerun back-to-back. Configurable via env vars: `API_URL`
(default `http://localhost:8080`), `ORIGIN` (default `http://localhost:5173`,
must be in `CORS_ALLOWED_ORIGINS`), and `SESSION_RATE_LIMIT` (default 5, must
match the API's setting).

