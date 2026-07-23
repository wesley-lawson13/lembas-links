# Implement the Lembas Links Frontend Design

## Context

The `refactor/frontend-readiness` work (merged PR #20) made the Go API fully frontend-ready: anonymous key minting via `POST /session`, Bearer auth with sliding TTL, `GET /links` scoped to the caller, ownership-checked stats, CORS allow-list. Its Phase 9 frontend scaffold, however, was **never built** — `frontend/` does not exist; this is greenfield on the `feat/frontend` branch.

The user has an approved visual design (claude.ai/design project `ee597686…`, `Lembas Links.dc.html` + `LembasScreen.dc.html`, fetched and analyzed): two screens — **Dashboard** (forge form, "Freshly forged" result card, "Your fellowship of links" list) and **Slug Stats** (metrics, 7-day click chart, "Recent passages" table) — with a live theme toggle between **Parchment light** and **Rivendell cool-dark**, all driven by CSS custom properties, set in Cinzel + EB Garamond. The mockup's fake browser-chrome strip (dots + URL bar) is framing only and is not built; the theme toggle relocates to the real page header (top-right).

**Decisions made with the user:** ① real backend per-day click aggregation for the chart, ② plain CSS + CSS custom properties (no Tailwind), ③ react-router-dom, ④ theme defaults to `prefers-color-scheme`, toggle persists override in localStorage.

**Gap found during exploration:** the stats endpoint returns only lifetime `click_count` + 10 recent clicks — no per-day data — hence the backend phase below. While touching that endpoint, also add `short_url` to its response (handler already has `cfg.BaseURL`) so deep-linked stats pages never need to reconstruct or look up the short URL.

**First implementation step:** copy this plan into the repo at `.claude/plans/frontend-implementation.md` (per user request, alongside the existing `frontend-readiness.md`).

## Concurrent execution

The two tracks touch **disjoint file sets** and run in parallel:

| | Track A — Backend | Track B — Frontend |
|---|---|---|
| **Who** | Background subagent in an isolated **git worktree** (branch `feat/daily-clicks` off `feat/frontend`) | Main session, directly on `feat/frontend` |
| **Files** | `api/models/clicks.go`, `api/handlers/stats.go`, `api/handlers/types.go`, `api/docs/` (swag), test files, `scripts/e2e_smoke_test.sh` | `frontend/**` (new), `docker-compose.yml`, `CLAUDE.md`, `README.md`, `.gitignore` |
| **Commits** | Commit 1 (impl + swagger), Commit 2 (tests) | Commit 3 (scaffold), 4 (Dashboard), 5 (Stats), 6 (compose + docs) |

**Why this is safe:** zero file overlap → guaranteed clean merge. Track B codes against the *agreed contract* (`daily_clicks: [{date, count}]`, `short_url` in `StatsResponse`) — defined in this plan, not discovered from Track A's output — so it never blocks on Track A. The subagent prompt carries the full contract + SQL + test spec below verbatim.

**Why Dashboard/Stats are NOT further split into parallel agents:** they share `format.ts` / `app.css` / `CopyButton` (conflict surface), and the design fidelity lives in this session's context — delegating the screens means lossy re-transmission of the design HTML. Instead, Commit 3 pre-creates *all* shared files (every `format.ts` helper, `CopyButton`, base styles) so the two screen commits are small and self-contained. Screens are built sequentially in the main session while Track A runs in the background.

**Integration (after both tracks finish):** cherry-pick Track A's two commits onto `feat/frontend` (keeps linear history; expected conflict-free), remove the worktree, then run the full verification section — e2e and live stats-page checks are the only steps that genuinely require both tracks.

**Sync points:**
1. Start: copy plan into repo → spawn Track A agent → immediately begin Track B scaffold.
2. If Track A finishes first: cherry-pick as soon as convenient; stats-page live verification unblocks.
3. If Track B finishes first: continue with dashboard-only browser verification (list/create/delete/theme need no new backend); hold e2e + stats verification for the cherry-pick.

## Design tokens (from the approved design, exact)

- **Parchment light**: `--bg:#ece1c4 --card:#efe6cd --card2:#efe5cb --ink:#33291a --mut:#7a6b4f --line:#ddd1b0 --dot:#cdbc94 --acc:#9c7a34 --acc-ink:#fbf5e4 --title:#4a381c --ok:#5c7348 --ok-bg:rgba(92,115,72,.12) --warn:#a5522f --shadow:0 14px 36px rgba(60,45,20,.16)`
- **Rivendell dark**: `--bg:#141a1e --card:#1a2127 --card2:#1b232a --ink:#dde7ec --mut:#8a9ba6 --line:#28323a --dot:#3a464e --acc:#93b7c9 --acc-ink:#0f1518 --title:#bcd6e2 --ok:#7fb0a0 --ok-bg:rgba(127,176,160,.14) --warn:#c98f7a --shadow:0 18px 44px rgba(0,0,0,.55)`
- Fonts: Cinzel 600/700 (headings/labels/buttons), EB Garamond 400/500/600 + italic (body), `ui-monospace` stack (slugs/IPs/short URLs). Google Fonts `<link>` in `index.html`.
- Theme flip: everything transitions `background-color/color/border-color/box-shadow .32s ease`.

## Track A — Backend: `daily_clicks` + `short_url` on `GET /links/:slug/stats` *(background subagent, worktree)*

### Commit 1 (implementation + swagger)

`api/models/clicks.go` — add (following existing `Click`/`GetClicks` shape):

```go
// DailyClickCount is one day's click total for a slug, Date formatted "2006-01-02".
type DailyClickCount struct {
    Date  string `json:"date"`
    Count int    `json:"count"`
}
```

`GetDailyClicks(slug string, days int) ([]DailyClickCount, error)` — called from `handlers.GetStats`. Single query, zero-fill in SQL so Go is a dumb row scanner:

```sql
SELECT d.day::date, COALESCE(c.count, 0)
FROM generate_series(CURRENT_DATE - ($2 - 1) * INTERVAL '1 day', CURRENT_DATE, '1 day') AS d(day)
LEFT JOIN (
    SELECT clicked_at::date AS day, COUNT(*) AS count
    FROM clicks
    WHERE slug = $1 AND clicked_at >= CURRENT_DATE - ($2 - 1) * INTERVAL '1 day'
    GROUP BY 1
) c ON c.day = d.day::date
ORDER BY d.day ASC;
```

Always exactly `days` entries, oldest → newest, zeros included. **Buckets are UTC** — `clicks.clicked_at` is naive `TIMESTAMP` written by Postgres `NOW()` in a UTC container, so `CURRENT_DATE`/`::date` bucket consistently with how rows are written; a client-timezone param is not worth it for a 7-bar mini chart (flagged as accepted trade-off). Existing `idx_clicks_slug` covers the filter; no migration.

`api/handlers/stats.go` (`GetStats`, after the `GetClicks` call at `stats.go:51`):
- `dailyClicks, err := lh.store.GetDailyClicks(slug, dailyClickDays)` (package const `= 7`); on error log + fall back to `[]models.DailyClickCount{}` — same degraded-analytics posture as the existing `GetClicks` fallback.
- Add to the `gin.H`: `"daily_clicks": dailyClicks` and `"short_url": lh.cfg.BaseURL + "/" + urlStats.Slug` (matches `CreateLink`/`ListLinks` construction).

`api/handlers/types.go` — extend `StatsResponse` (swagger doc struct) with `ShortURL string \`json:"short_url"\`` and `DailyClicks []DailyClickResponse \`json:"daily_clicks"\``; add `DailyClickResponse{Date, Count}` with examples. Regenerate: `cd api && swag init` (bundled in this commit per convention).

### Commit 2 (tests)

- Models test (in `api/models/store_test.go`, following `setupTestDB`-skip / `t.Cleanup` / `cleanupURLs` patterns): insert clicks with **explicit** `clicked_at` via direct `db.Exec` (can't use `RecordClick` — it relies on the `NOW()` default): 2 today, 3 three days ago, 1 eight days ago → assert 7 entries, ascending ending today, correct counts, zero days present, 8-day-old excluded. Second case: slug with zero clicks → 7 zeros.
- Handler test (`api/handlers/stats_test.go`, following the existing ownership-mismatch test): owner request decodes `daily_clicks` (len 7) and `short_url`.
- `scripts/e2e_smoke_test.sh`: extend the stats step to `grep -q '"daily_clicks":\['` (presence check, matching the script's grep style).

Track A checkpoint (inside the worktree): `cd api && go build ./...` + `make test`. The `make e2e` / manual-curl checks move to post-integration (they need the compose stack, which the main session owns).

## Track B.1 — Frontend scaffold (Commit 3, single commit) *(main session, starts immediately)*

`npm create vite@latest frontend -- --template react-ts`, add `react-router-dom`, prune Vite boilerplate.

```
frontend/
├── Dockerfile                  # node:22-alpine, npm install, CMD npm run dev -- --host 0.0.0.0, EXPOSE 5173
├── .env.example                # VITE_API_BASE_URL=http://localhost:8080  (browser-origin URL, not compose hostname)
├── index.html                  # Google Fonts links + inline pre-paint script setting data-theme (no FOUC)
└── src/
    ├── main.tsx                # createRoot + BrowserRouter: "/" → Dashboard, "/stats/:slug" → Stats
    ├── App.tsx                 # shell: header (Cinzel title + ThemeToggle top-right), <Outlet/>
    ├── api.ts                  # typed client + key lifecycle (below)
    ├── types.ts                # exact TS mirrors of API contract (below)
    ├── format.ts               # daysUntil / isExpired / date & "Jul 23 · 14:02" formatters / summarizeUserAgent / referrerHost
    ├── useTheme.ts             # read/toggle data-theme + localStorage persist
    ├── pages/Dashboard.tsx     # state owner: list, freshly-forged, create/delete
    ├── pages/Stats.tsx         # fetch by :slug, 404 state, composes chart + table
    ├── components/ThemeToggle.tsx   # ☀/☾ pill, active side filled --acc
    ├── components/CopyButton.tsx    # outlined accent, clipboard.writeText, brief "Copied" state
    ├── components/CreateLinkForm.tsx
    ├── components/FreshlyForged.tsx
    ├── components/LinkRow.tsx
    ├── components/ClicksChart.tsx
    ├── components/RecentClicksTable.tsx
    └── styles/theme.css + styles/app.css
```

No context providers, no state libraries — two pages, one fetch layer.

**`types.ts`** (timestamps stay RFC3339 strings; `format.ts` owns Date conversion):
`SessionResponse{api_key, expires_at}` · `CreateLinkResponse{slug, short_url, original}` · `LinkSummary{slug, short_url, original, click_count, created_at, expires_at}` · `ListLinksResponse{links: LinkSummary[]}` · `ClickEntry{id, slug, clicked_at, referrer, user_agent, ip_address}` · `DailyClick{date, count}` · `StatsResponse{slug, short_url, original, click_count, created_at, expires_at, is_active, recent_clicks, daily_clicks}` · `ApiError extends Error {status}`.

**`api.ts`**: `API_BASE = import.meta.env.VITE_API_BASE_URL`; localStorage key `lembas_api_key`. `getOrCreateApiKey()` returns cached key else `POST /session` → store → return. Private `request<T>(path, init?)`: attach `Authorization: Bearer`; **on 401 remove cached key, mint fresh, retry exactly once** (flag, not loop — old key's links orphan, accepted); 204 → undefined; non-ok → throw `ApiError` with parsed `{"error"}`. Public: `createLink(url)`, `listLinks()`, `getStats(slug)`, `deleteLink(slug)`. Frontend **never constructs short URLs** — always renders `short_url` from responses.

**Theme**: inline script in `index.html` reads `localStorage.lembas_theme` else `matchMedia('(prefers-color-scheme: dark)')` → sets `document.documentElement.dataset.theme` pre-paint. `useTheme.ts` toggles the attribute + persists only explicit overrides (un-toggled visitors keep following OS). `theme.css`: `:root[data-theme="light"]{…}` / `:root[data-theme="dark"]{…}` with exact palettes + the universal `.32s` transition rule.

Scaffold pre-creates **all shared surface** used by both screens: every `format.ts` helper (incl. `summarizeUserAgent`, `referrerHost`), `CopyButton`, and base styles in `app.css` — so Commits 4 and 5 don't touch common files.

## Track B.2 — Dashboard (Commit 4)

State in `Dashboard.tsx`: `links: LinkSummary[] | null` (null = loading), `fresh: CreateLinkResponse | null`, `error`.
- Mount: `listLinks()` (server-sorted `created_at DESC`, `[]` when empty).
- Create: `createLink(url)` → set `fresh` (renders "Freshly forged" card above list: accent caption, mono 18px short_url, outlined Copy filling accent on hover, "→ original" truncated, "View stats →" link) → re-fetch list (authoritative, no optimistic prepend). 400/429 message inline under input, input preserved.
- Delete: `window.confirm("Cast «slug» into the fire?")` (native confirm — one call site, no modal to build) → `deleteLink` → re-fetch; 404 race treated as done; clear `fresh` if it was that slug.
- Empty state (`links.length === 0 && !fresh`): card2 panel — Cinzel caption "The road goes ever on", muted line "No links yet forged. Paste a URL above to begin the journey."

`LinkRow.tsx`: mono short_url (--title) + meta `{click_count} clicks · expires in {N} days` (`daysUntil` = ceil of ms diff / 86 400 000; singular day; 0 → "expires today"; warn color when ≤ 3). Expired (`expires_at < now`): row opacity .62, line-through slug, italic "expired", buttons reduced to Stats/Delete. Buttons: Copy (short_url), Stats (`<Link to={"/stats/"+slug}>`), Open (`target="_blank" rel="noreferrer"`), Delete (warn hover). Header: "Your fellowship of links" + right-aligned "{N} forged".

## Track B.3 — Stats screen (Commit 5)

`Stats.tsx`: `useParams().slug` → `getStats(slug)` on mount/param change. 404 → themed not-found state ("This passage is not recorded in the Red Book…") + "← Return to dashboard" link (covers nonexistent/not-owned/deleted uniformly, as the API intends). Header per design: return link, Cinzel 26px slug h1, mono accent `short_url` (now from the stats response) + CopyButton, right pill badge — **Active** (--ok/--ok-bg) vs **Expired** (warn) from `expires_at < now` (ignore `is_active`; always true on a 200). "Points to" card2 with truncated original + "Open ↗". Three metric cards: Cinzel 30px `click_count` / "Forged on" / "Expires at" (`toLocaleDateString {month:'short', day:'numeric', year:'numeric'}`).

`ClicksChart.tsx` — plain divs, no library: 7 flex columns from `daily_clicks` (already zero-filled/ordered by backend; component only computes `max = Math.max(...counts, 1)`). Bar `height:(count/max)*100%` in fixed 104px track, `var(--acc)` at .35 opacity, max day 1.0, today .72 (max wins). Count label above, day label below via `new Date(d.date+"T00:00:00Z").toLocaleDateString("en-US",{weekday:"short",timeZone:"UTC"})` — UTC on both ends so labels can't drift off the backend's UTC buckets.

`RecentClicksTable.tsx` — 4-col CSS grid (When / Came from / User agent / Origin), uppercase Cinzel headers. `format.ts` helpers: timestamp "Jul 23 · 14:02"; `referrerHost` — empty → italic "— direct —", else `new URL(ref).hostname` in try/catch (fallback raw); `summarizeUserAgent` → "Chrome · macOS" via two ordered regex ladders (browser: `Edg/`→Edge, `OPR/`→Opera, `Firefox/`, `Chrome/`, `Safari/`, else Unknown; OS: `Windows NT`, `iPhone|iPad`→iOS **before** Mac, `Mac OS X|Macintosh`→macOS, `Android`, `Linux`, else Unknown) — **no ua-parser-js**: ~60 KB to classify ≤10 coarse rows; ~15 lines suffices. Empty clicks → muted italic "No travellers have passed this way yet."; footer caption "Showing the 10 most recent passages" only when rows exist.

## Track B.4 — Compose + docs (Commit 6)

`docker-compose.yml`:
```yaml
frontend:
  build: ./frontend
  ports: ["5173:5173"]
  volumes:
    - ./frontend:/app
    - /app/node_modules   # anonymous volume so host mount doesn't shadow the image's install
  depends_on: [api]
```
Vite reads mounted `frontend/.env`; no compose `environment` needed. Update `CLAUDE.md` (frontend/ architecture, api.ts key lifecycle, theme system, routes, `frontend/.env.example`) and `README.md` (`cp frontend/.env.example frontend/.env` → `make run` → `http://localhost:5173`). Ensure `.gitignore` covers `frontend/node_modules` / `frontend/.env`.

## Conventions

- CLAUDE.md rule: 1–2 sentence doc comment on every new function (what + where called from).
- Commits: subject = descriptive partial sentence; body = what + why. Implementation/tests split per phase (Commits 1↔2); scaffold/compose phases single commits; swagger regen bundled with Commit 1.

## Verification *(post-integration: after Track A's commits are cherry-picked onto `feat/frontend`)*

1. `cd api && go build ./...`; `make test` (incl. new daily-clicks cases); `make test-rate` still green.
2. `make run` → `make e2e` (now asserting `daily_clicks`); manual `curl` of stats shows `daily_clicks` + `short_url`.
3. Browser at `localhost:5173`:
   - Cleared localStorage → silent `POST /session` (Network tab), empty state, theme follows OS.
   - Forge link → Freshly forged card, Copy works, list shows "1 forged".
   - Visit short link a few times → stats: count, today's bar, passages rows (UA summary, referrer, IP).
   - Deep-link/refresh `/stats/:slug` → renders (short_url from stats response); bogus slug → themed 404.
   - Delete `lembas_api_key` from localStorage, act → one transparent re-mint + retry (empty list, orphaned as expected).
   - Toggle theme on both screens → .32s crossfade; refresh persists.
   - Delete link → confirmed → gone; its stats page 404s.

## Accepted trade-offs

- **UTC chart buckets**: a UTC-8 user clicking 5 pm local lands in the next day's bar. Fix (client tz param) is additive later; `clicked_at` as naive TIMESTAMP is pre-existing.
- **Session mint 5/hr/IP** can bite during dev when clearing localStorage repeatedly — flush the Redis counter as the escape hatch.
- **Universal transition rule** may animate initial paint in some browsers; if visible, scope it to a class added after first frame.
- Compose frontend service is **dev-only** (Vite dev server, no prod build) — deployment is the user's stated later phase.
- Vite HMR in Docker on macOS may need `server.watch.usePolling: true` — add only if hot reload fails.
- **Worktree agent + DB-backed tests**: the models tests follow the skip-if-env-unset convention; if the compose Postgres isn't up while Track A runs, its tests compile but skip. They're re-run for real in post-integration verification either way, so this only delays feedback, not coverage.
