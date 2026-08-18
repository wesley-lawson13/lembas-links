# Lembas Links — Frontend

Vite + React + TypeScript SPA for the Lembas Links URL shortener. Talks to the
Go API over CORS; an anonymous API key is minted silently via `POST /session`
on first visit and kept in localStorage (see `src/api.ts`).

Or via Docker Compose from the repo root: `make run`.

### Structure

- `src/api.ts` — typed API client + anonymous-key lifecycle (mint, cache, retry-on-401)
- `src/types.ts` — TS mirrors of the API's JSON contract
- `src/format.ts` — date/expiry/user-agent presentation helpers
- `src/useTheme.ts` + `src/styles/theme.css` — Parchment/Rivendell theme system
  (CSS custom properties keyed off `data-theme` on `<html>`)
- `src/pages/` — `Dashboard` (`/`) and `Stats` (`/stats/:slug`)
- `src/components/` — screen building blocks
