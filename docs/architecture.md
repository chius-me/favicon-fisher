# Architecture

```text
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│  CLI (fvf)  │     │ Go Web UI    │     │ CF Worker       │
│  cmd/fvf    │     │ cmd/fvf-web  │     │ worker/src/*    │
└──────┬──────┘     └──────┬───────┘     └────────┬────────┘
       │                   │                      │
       ▼                   ▼                      ▼
┌──────────────────────────────────────────────────────────┐
│ Discovery → rank → download / proxy / convert            │
│ Go: internal/fetcher, convert, security                  │
│ Worker: discovery/, security/, routes/ (TypeScript)      │
└──────────────────────────────────────────────────────────┘
```

## Components

| Layer | Responsibility |
|-------|----------------|
| `cmd/fvf` | CLI: fetch best or `--all`, JSON output |
| `cmd/fvf-web` | HTTP server, embeds static UI |
| `internal/fetcher` | URL normalize, HTML/manifest discovery, download |
| `internal/convert` | Format allowlist, magic bytes, pixel limits |
| `internal/security` | SSRF, limits, signing, rate limit, observability |
| `internal/web` | REST handlers + static assets |
| `worker/src` | Edge-equivalent API + browser conversion |

## Request flow (Web / Worker)

1. `POST /api/preview` — fetch page (SSRF-safe), parse icons, optional manifest, probe CTs (capped), sign tokens (`fetch` purpose).
2. Browser loads thumbs via `GET /api/proxy?url&token` (same-origin; never direct remote `img src`).
3. Download: Go `POST /api/download` re-encodes; Worker uses canvas conversion client-side.

## Parity

Shared golden fixtures live in `testdata/golden/`:

- Go: `DiscoverFromHTML` + contract JSON tests  
- Worker: Vitest discovery tests + `npm run test:golden`

Known intentional differences: Worker SSRF is hostname/literal-based; Go re-validates DNS at dial. See [security.md](./security.md).

## Observability

- **Go:** JSON `slog` to stdout; `X-Request-ID` on every response; access log for `/api/*` and HTTP ≥400 (`request_id`, `method`, `path`, `status`, `duration_ms`).
- **Worker:** JSON `console.log` / `console.error` with the same fields for API and errors.
