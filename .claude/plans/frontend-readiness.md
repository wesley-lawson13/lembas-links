# Prepare Lembas Links API for a React Frontend

## Context

The user wants to add a React frontend to this Go/Gin URL shortener but wants it **lightweight** — no login/signup screen, no user accounts. The current backend can't support any frontend as-is: there's no CORS middleware at all (browser requests would be blocked outright), auth is a single static plaintext API key with no self-serve provisioning (only created via a hardcoded `make seed-dev` SQL insert), and there's no endpoint to list a caller's own links (every read requires an exact known slug).

The agreed approach: the frontend silently mints its own anonymous, short-lived API key on first visit (no user-facing flow at all), stores it in localStorage, and sends it as a Bearer token from then on. The key's expiry **slides forward** on each use so an active visitor never loses access to their links; only abandoned keys age out. This is deliberately simpler than a full accounts system while still fixing the actual security problem in the current code (API keys are stored in plaintext today).

While tracing `POST /links`, the exploration also surfaced a real bug worth fixing as part of this work: link ownership (`urls.api_key`) is currently taken from a client-supplied `body.api_key` JSON field, not from the authenticated `Authorization` header — meaning ownership is currently unenforceable and anyone can claim any link. This must be fixed for the new "list my links" endpoint to mean anything.

Frontend stack: Vite + React + TypeScript SPA, calling the Go API directly over CORS (no BFF/Next.js layer needed since there's no secret worth hiding server-side once keys are self-serve/anonymous anyway).

## Key decisions (final, not open questions)

1. **No login UI.** `POST /session` (public, unauthenticated) mints a random key, stores its SHA-256 hash + `expires_at`, returns the raw key once.
2. **Sliding TTL**: every successful `ValidateKey` call extends `expires_at` by `DEFAULT_TTL_DAYS`. Expired keys are rejected (401).
3. **Fix the Bearer-prefix bug**: `Authorization` header is used raw today despite Swagger documenting `Bearer <key>` — strip it consistently via one shared helper used by both `middleware/auth.go` and `middleware/rate.go`.
4. **New `GET /links`**: lists the caller's own active links, matched by comparing the hashed authenticated key against `urls.api_key` (which must therefore also store the hash, not a client-supplied value — see the ownership bug above).
5. **Separate, stricter rate limit on `POST /session` itself** (IP-based, e.g. 5/hour) — since minting a fresh key trivially resets the existing per-key rate-limit counter, the real abuse backstop has to live on session-minting.
6. **CORS** via `gin-contrib/cors`, explicit origin allow-list from a new `CORS_ALLOWED_ORIGINS` env var — no wildcard.
7. **Frontend**: new `frontend/` dir, Vite+React+TS, added as a `docker-compose.yml` service mirroring the existing `api`/`postgres`/`redis` pattern.

## Git workflow

Create a new branch off the current branch (e.g. `refactor/frontend-readiness`) before starting Phase 1; all work below lands on this branch, one phase at a time, in order:

- Each phase gets its own commit(s) — don't bundle multiple phases into one commit.
- Where a phase's plan includes both implementation and test changes (Phases 2, 3, 4, 6/6c), split into **two commits**: the implementation first, then the tests for that same phase.
- Phases that are testing-only in nature (Phase 8's `scripts/e2e_smoke_test.sh`) are a single commit — the "split if testing is involved" rule is about pairing a feature with its tests, not about isolating a phase whose entire content is tests.
- Phases with no test changes (Phase 1 migration, Phase 5/5b session endpoint + dev seeding, Phase 7 CORS, Phase 9 frontend scaffold) are a single commit each, unless the phase is substantially large enough to warrant splitting the *work itself* into more than one commit (e.g. Phase 6 could reasonably split into "fix CreateLink ownership + ListLinks" and "GetStats ownership fix (6c)" as two work commits, still followed by their test commit(s)) — use judgment on size, not a fixed rule.
- **Swagger regeneration** (`swag init` diffs under `api/docs/`) gets bundled into whichever commit triggered it (Phase 6/6c's handler and type changes), never split into its own commit — it's a large generated diff not worth isolating.

## Implementation sequence

### Phase 1 — Migration
The project isn't deployed anywhere yet, so there's no live data/migration history to preserve — alter the original migration directly instead of bolting on a new one. Edit `db/migrations/003_create_api_keys.up.sql`:
```sql
CREATE TABLE api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key         VARCHAR(255) UNIQUE NOT NULL,
    name        VARCHAR(255),
    expires_at  TIMESTAMP NOT NULL DEFAULT (NOW() + INTERVAL '30 days'),
    created_at  TIMESTAMP DEFAULT NOW()
);
```
No column width change needed — `api_keys.key`/`urls.api_key` are `VARCHAR(255)`, and a SHA-256 hex digest is 64 chars.

### Phase 2 — Shared Bearer-stripping helper
New `api/middleware/bearer.go`, exported so `handlers` can also use it (no import cycle: `middleware` never imports `handlers`):
```go
func ExtractBearerToken(header string) string {
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return header
}
```
Add `api/middleware/bearer_test.go` covering: with prefix, without prefix, empty string, `"Bearer "` with empty token.

### Phase 3 — Key hashing, expiry, sliding renewal
Rewrite `api/models/api_key.go`:
- `hashKey(key string) string` — SHA-256 hex (appropriate here since these are high-entropy random tokens, not passwords — bcrypt/argon2 unnecessary).
- `generateRawKey(n int) (string, error)` — `crypto/rand`, 32 bytes → 64 hex chars.
- `CreateKey(ttl time.Duration) (string, error)` — generates raw key, inserts hash + `expires_at = now+ttl`, returns raw key (never persisted, unrecoverable after this call).
- `ValidateKey(key string, ttl time.Duration) error` — **signature changes** from `(key string) error`. Does an atomic `UPDATE api_keys SET expires_at = NOW() + $2 WHERE key = $1 AND expires_at > NOW() RETURNING id` to check-and-slide in one round trip (avoids a check/extend race). On no match, distinguishes not-found vs. expired for logging only — HTTP layer returns one generic message either way.
- Export `HashKey(rawKey string) string` directly (rename, not a wrapper) for `handlers` to hash the caller's key when matching against `urls.api_key`. (A separate `HashKeyForOwnership` wrapper over an internal `hashKey` was considered so the hashing algorithm stayed a models-internal detail, but with only one real cross-package call path today that indirection doesn't earn its keep — a straight rename is simpler and just as easy to split later if a second, differently-scoped use ever appears.)

Update `api/models/store_test.go`: existing `TestValidateKey` assumes the plaintext `test-api-key-123` row validates directly — rewrite to mint via `CreateKey` first, update all call sites for the new `ttl` param. Add tests for: successful create+validate round trip, expired-key rejection (insert directly with `expires_at` in the past), sliding renewal actually extending `expires_at` on repeated validation. Follow the existing `cleanupURLs`/`t.Cleanup` pattern in this file for teardown.

### Phase 4 — Wire into middleware
`api/middleware/auth.go` — `APIKeyAuth` now takes `(store *models.URLStore, cfg *config.Config)` (needs `cfg` to compute the TTL), strips the Bearer prefix via `ExtractBearerToken`, calls `store.ValidateKey(key, ttl)`. 401 body becomes generic `"invalid or expired api key"`.

`api/middleware/rate.go` — the API-key rate bucket must key off `ExtractBearerToken(authHeader)`, not the raw header, so `"Bearer x"` and `"x"` share one counter.

`api/main.go` — update the one call site: `middleware.APIKeyAuth(store, cfg)`.

Update `api/middleware/rate_test.go`: the existing `TestRateLimit_BlocksRequestsOverKeyThreshold` cleans up Redis key `rate:key:Bearer test-rate-key-1` — after stripping, the real key is `rate:key:test-rate-key-1`, fix the cleanup line. Add a test confirming prefixed/unprefixed headers share a counter.

Add `api/middleware/auth_test.go` (didn't exist before) covering: missing header, unknown key, expired key, valid key — DB-backed, following the existing `setupTestDB`-skip-if-unset convention.

**Checkpoint**: `cd api && go build ./...`, `make test`, `make test-rate` all green before adding new routes. Expect the old `test-api-key-123` dev key to now 401 (it's plaintext, lookups are hash-based) — resolved in Phase 6b.

### Phase 5 — `POST /session`
`api/config/config.go` additions: `SessionRateLimit` (env `SESSION_RATE_LIMIT`, default 5), `SessionRateLimitWindow` (env `SESSION_RATE_LIMIT_WINDOW`, default 3600s), `CORSAllowedOrigins []string` (env `CORS_ALLOWED_ORIGINS`, comma-separated, new `getEnvList` helper).

**Rationale for the defaults (5 per hour)**: under sliding TTL, a legitimate visitor mints a key once and reuses it from localStorage indefinitely — repeat mints from one real user should be rare (cleared storage, incognito, a shared office IP). The cap has to stay low because each mint hands out a fresh 120/min key-based budget, so mint-rate is what actually bounds "how many spam identities one IP can create per hour" now that keys are free — that's the real thing being defended, not raw request volume. The 1-hour window (vs. per-minute) reflects that this is a sustained-abuse concern, not a burst concern. These are reasonable starting defaults, not derived from measured traffic — both are env-tunable and worth revisiting once there's real usage data.

`api/middleware/rate.go` addition — `SessionRateLimit(r *redis.Client, cfg *config.Config) gin.HandlerFunc`, IP-only, Redis key `rate:session:<ip>`, reuses the existing `parseRate` helper. This is the **only** rate limiter on `/session` (stricter than and not stacked with the general `RateLimit`).

New `api/handlers/session.go` — `SessionHandler.CreateSession`: calls `store.CreateKey(ttl)`, returns `201 {"api_key": "<raw>", "expires_at": ...}`. Add `SessionResponse` struct to `types.go`.

`api/main.go`: register `r.POST("/session", middleware.SessionRateLimit(redis, cfg), sessionHandler.CreateSession)` alongside `/health`/`/swagger`.

`.env.example` additions: `SESSION_RATE_LIMIT=5`, `SESSION_RATE_LIMIT_WINDOW=3600`, `CORS_ALLOWED_ORIGINS=http://localhost:5173`.

**Phase 5b — fix local dev seeding**: `Makefile`'s `seed-dev` inserts plaintext `test-api-key-123` — update it to insert `sha256("test-api-key-123")` (`a2e4ab0472c808a1ff2ce147ae4f6cd9ecd8bcc8a49c48350f97e6811ace7464`) plus an `expires_at`, with a comment noting the plaintext value developers still pass as the Bearer token. Note in the PR that `POST /session` is now the primary key-provisioning path; `seed-dev` is a manual-testing convenience only.

### Phase 6 — `GET /links` + fix create-link ownership bug
`api/handlers/links.go` — `CreateLink`: drop `APIKey` from the request body struct entirely (breaking wire change — silently ignored if old clients still send it, worth calling out). Derive the owner from the authenticated header instead:
```go
ownerKey := models.HashKey(middleware.ExtractBearerToken(c.GetHeader("Authorization")))
```
Pass `ownerKey` (a hash) to `store.CreateURL(...)`, so `urls.api_key` stores the same hash form as `api_keys.key`, never a raw secret. Update `CreateLinkRequest` in `types.go` to drop `APIKey`.

`api/models/url.go` addition — `ListURLsByAPIKey(rawAPIKey string) ([]URL, error)`: hashes the input the same way, `SELECT ... FROM urls WHERE api_key = $1 AND is_active = TRUE ORDER BY created_at DESC`.

`api/handlers/links.go` addition — `ListLinks` handler: extracts+hashes the caller's key, calls `ListURLsByAPIKey`, maps to a new `LinkSummary`/`ListLinksResponse` (in `types.go`) with `slug`, `short_url`, `original`, `click_count`, `created_at`, `expires_at`.

`api/main.go`: `protected.GET("", linkHandler.ListLinks)` inside the existing `/links` group.

**Verify routing precedence once**: `GET /:slug` (redirect, root-level) and `GET /links` (exact) both exist — Gin prioritizes static segments over param segments at the same level, so this resolves correctly, but confirm manually: `curl http://localhost:8080/links` (no auth) should 401 from `APIKeyAuth`, not fall through to the redirect handler.

Add `TestListURLsByAPIKey` to `store_test.go`: two keys, two links each under key A / one under key B, assert `ListURLsByAPIKey` for key A returns only its 2, and that soft-deleting one drops it from the list. Follow existing `GetSlug`/`cleanupURLs` patterns already in that file.

**Why `ListLinks` doesn't wrap `GetStats`**: they serve different shapes on purpose. `ListLinks` is one lightweight query returning a summary for *all* of the caller's active links (slug, short_url, original, click_count, created_at, expires_at) — a dashboard overview. `GetStats` is a single-slug detail lookup that does an *extra* query against `clicks` for up to `RecentClicksLimit` recent click events (referrer/user-agent/IP per visit) — a drill-down view. Having `ListLinks` call `GetStats` once per link in a loop would be an N+1 query problem, and a list view has no need for every link's individual click log. They stay separate, each with its own query, matching the standard list-vs-detail split (e.g. GitHub's list-repos vs. get-repo).

**Surfacing stats on the dashboard**: `ListLinks` already carries `click_count` per link, so the overview gets basic stats for free — no backend combining needed there. For deeper detail (recent clicks), the frontend should lazily call `GetStats(slug)` only when a user drills into one specific link (e.g. expanding a row), not upfront for every link in the list. List-then-detail composes naturally without any backend merging.

### Phase 6c — Fix missing ownership check on `GetStats` (found during this work, not optional)
`GetStats` (`api/handlers/stats.go`) currently has **no ownership check at all** — it looks up `slug` and returns full detail (including per-visitor referrer/user-agent/IP from `recent_clicks`) to *any* caller holding *any* valid API key, not just the slug's owner. This was a smaller risk when keys were hand-provisioned to trusted people; it becomes a real info leak once anyone can self-serve a free key via `/session`. Fix as part of this phase, not deferred:
- Add `models.URLStore.GetURLOwner(slug) (string, error)` (or extend `GetStats`'s existing query to also select `api_key`) so the handler can compare the stored hash against the caller's hashed key.
- In `GetStats`, after the existing not-found/expired checks, compare `urlStats`'s owning key hash against `models.HashKey(middleware.ExtractBearerToken(c.GetHeader("Authorization")))`; on mismatch return `404 {"error": "stats not found"}` — the same message as a nonexistent slug, so the endpoint never leaks *which* slugs exist versus which you don't own (avoids an enumeration oracle).
- Add a test: two keys, link created under key A, `GetStats` called with key B's token → 404.

Regenerate Swagger (`cd api && swag init`) after Phases 5–6.

### Phase 7 — CORS
`go get github.com/gin-contrib/cors && go mod tidy` in `api/`.

`api/main.go`, registered as the **first** `r.Use()` call (before any route group, so preflight `OPTIONS` never hits `RateLimit`/`APIKeyAuth`/`SessionRateLimit`):
```go
r.Use(cors.New(cors.Config{
	AllowOrigins:     cfg.CORSAllowedOrigins,
	AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
	AllowHeaders:     []string{"Authorization", "Content-Type"},
	AllowCredentials: false, // bearer header only, no cookies
	MaxAge:           12 * time.Hour,
}))
```
No wildcard — driven entirely by `CORS_ALLOWED_ORIGINS` (empty/unset = no origins allowed).

Verify: `curl -i -X OPTIONS http://localhost:8080/links -H "Origin: http://localhost:5173" -H "Access-Control-Request-Method: POST" -H "Access-Control-Request-Headers: Authorization,Content-Type"` → `204` with the origin echoed back; confirm a disallowed origin gets no `Access-Control-Allow-Origin` header.

### Phase 8 — End-to-end smoke tests (`scripts/`)
Before touching the frontend, prove the whole backend flow works over real HTTP, not just the Go unit/integration suites (which test packages directly, not the running server). `scripts/` already exists in the repo (currently empty) — add:

`scripts/e2e_smoke_test.sh` — bash + curl against a running `docker compose` stack (assumes `make run` is already up, matching how `seed`/`migrate` targets assume the stack is running). Steps, each asserting the expected status code:
1. `GET /health` → 200
2. `POST /session` → 201, capture `api_key` (key A)
3. `POST /links` with key A → 201, capture `slug`
4. `GET /links` with key A → 200, assert `slug` present in the list
5. `GET /links/:slug/stats` with key A → 200
6. `POST /session` again → capture a second key (key B); `GET /links/:slug/stats` with key B → **404** (verifies the Phase 6c ownership fix)
7. `GET /:slug` (no auth) → 302 with a `Location` header
8. `DELETE /links/:slug` with key A → 204
9. `GET /:slug` again → 404 (soft-deleted)
10. `POST /session` 6× rapidly from the same shell (same IP) → 6th call → 429 (verifies Phase 5's session rate limit)
11. `OPTIONS /links` with `Origin: http://localhost:5173` → 204 with `Access-Control-Allow-Origin` echoed back

Add a `make e2e` target running this script. Fail the script (non-zero exit) on the first unexpected status code, printing which step failed.

**Load testing**: deferred until after the EC2 deployment — numbers from a laptop Docker Compose stack mostly measure local machine limits, not anything representative of real capacity, so they're not worth chasing yet. Revisit once there's an actual deployed target.

### Phase 9 — Frontend scaffold + Docker Compose

**Before writing any frontend code, confirm styling approach and layout with the user** (plain CSS vs. a utility framework like Tailwind vs. a component library, single-page dashboard vs. separate create/list views, etc.) — do not assume a design here.
New `frontend/` — minimal Vite+React+TS, just enough to prove the wiring end-to-end (not a real UI):
- Standard Vite React-TS scaffold (`package.json`, `vite.config.ts` with `server: { host: true, port: 5173 }`, `tsconfig.json`, `index.html`).
- `frontend/.env.example`: `VITE_API_BASE_URL=http://localhost:8080`
- `frontend/src/api.ts`: `getOrCreateApiKey()` (checks localStorage, else `POST /session` and stores the result), `listLinks(apiKey)`, `createLink(apiKey, url)` — all plain `fetch` calls with `Authorization: Bearer <key>`.
- `frontend/src/App.tsx`: on mount, mint/reuse key → list links → render as a plain list; one input + button to create a link and refresh the list. Enough to visually confirm CORS + session + auth + list + create work in a real browser.
- `frontend/Dockerfile`: `node:22-alpine`, `npm install`, `CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]`.

`docker-compose.yml` addition:
```yaml
  frontend:
    build: ./frontend
    ports:
      - "5173:5173"
    environment:
      VITE_API_BASE_URL: http://localhost:8080
    volumes:
      - ./frontend:/app
      - /app/node_modules
    depends_on:
      - api
```

### Phase 10 — Docs and final regression
- Regenerate Swagger if not already done.
- Update `CLAUDE.md`: document `POST /session` and `GET /links`, the new `frontend/` directory, new env vars (`SESSION_RATE_LIMIT`, `SESSION_RATE_LIMIT_WINDOW`, `CORS_ALLOWED_ORIGINS`). Leave `API_SECRET_KEY`/`API_KEY_SECRET` alone (confirmed dead/unused, no need to wire it up).
- Update `README.md` with the new endpoints and frontend dev instructions.

## Verification (end-to-end)

1. `cd api && go build ./...` — compiles clean after each phase's signature changes.
2. `make test` (`models/...`) and `make test-rate` (`middleware/...`) — both green.
3. `make run`, then manually:
   - `curl -X POST http://localhost:8080/session` → returns a key; repeat 6× rapidly from the same IP → 6th call gets `429`.
   - `POST /links` with that key → `201`; `GET /links` with that key → shows only that link; a second minted key's `GET /links` → empty list.
   - `curl http://localhost:8080/links` (no auth) → `401`, confirming routing precedence over the `/:slug` redirect handler.
   - CORS preflight check from Phase 7.
4. `make run` → open `http://localhost:5173` in a browser → devtools shows no CORS errors, empty link list loads, creating a link via the form updates the list live.

## Not in scope here

AWS/EC2/Terraform/GitHub Actions/Prometheus — user's plan is to tackle infra after the frontend is working (deploy manually on EC2 first, then codify with Terraform, then automate with GitHub Actions, then add observability last, since there's nothing meaningful to observe until something is deployed and taking traffic). That's a separate future plan.
