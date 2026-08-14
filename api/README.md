# Lembas Links — API

Go 1.25 + Gin REST API for the URL shortener. Redirects check Redis first, fall
back to Postgres on a cache miss, then cache the result and record the click
asynchronously; rate limiting and API key auth run as Gin middleware in front
of the protected routes. For the full endpoint contract (requests, responses,
status codes) see the root [`README.md`](../README.md#api-reference).

## Local development

Outside Docker you need `DATABASE_URL` and `REDIS_URL` pointed at reachable
instances:

```bash
docker compose up postgres redis   # from repo root
cd api
go run main.go                     # :8080 by default
```

`migrate/migrate.go` reads migrations and seed data from hardcoded container
paths (`/db/migrations`, `/db/seeds/quotes.sql`) that only resolve inside
Docker via the `./db:/db` volume mount in `docker-compose.yml`. Running
`go run` directly on the host means those paths won't exist, so migrations
won't auto-apply the way they do under `make run`. Use `make migrate` /
`make seed` from the repo root against a running Postgres container instead.

## Structure

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
