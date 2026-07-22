<p align="right">
  English | <a href="./README.zh.md">简体中文</a>
</p>

<h1 align="center">favicon-fisher</h1>

<p align="center">
  Find the best favicon for any website — CLI, local Web UI, or Cloudflare Worker.
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-GPL--3.0-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-Go-00ADD8.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-fisher?color=success">
  <img alt="CI" src="https://github.com/chius-me/favicon-fisher/actions/workflows/ci.yml/badge.svg">
</p>

## What it is

Paste a URL (or just a domain). **favicon-fisher** discovers icon candidates from the page, ranks them, and lets you download the one you want.

It looks at common sources: `<link rel="icon">`, apple-touch icons, mask icons, web manifests, and the classic `/favicon.ico` fallback.

## Ways to use it

| | | |
|---|---|---|
| **CLI** (`fvf`) | Terminal / scripts | Fastest path for power users |
| **Web UI** (`fvf-web`) | Browser on your machine or Docker | Preview candidates, pick format, download |
| **Worker** | Cloudflare edge | Same idea, zero server to maintain |

## Get started

### CLI

```bash
# From source
go build -o fvf ./cmd/fvf
./fvf --out tmp https://github.com

# Or grab a binary from Releases
```

Useful flags: `--all` (every candidate), `--json` (machine-readable), `--proxy`, `-o` (output dir).  
Run `./fvf --help` for the full list.

### Web UI

```bash
go build -o fvf-web ./cmd/fvf-web
./fvf-web
# open http://localhost:8080
```

Or with Docker:

```bash
docker compose up --build
# open http://localhost:8080
```

Published images: `ghcr.io/chius-me/favicon-fisher:latest` (also `:vX.Y.Z`).

### Cloudflare Worker

```bash
cd worker
npm install
npx wrangler secret put FVF_SIGNING_SECRET   # once
npx wrangler deploy
```

## Highlights

- **Smart discovery** — HTML link icons, manifests, and favicon fallback  
- **Sensible ranking** — prefers standard icons and larger sizes  
- **Format options** — download as PNG / JPG / SVG / ICO (and WebP in Worker browser conversion)  
- **Script-friendly CLI** — JSON output and batch download  
- **Ready to ship** — multi-platform releases, Docker image, one-command Worker deploy  

## Requirements

| You want to… | You need… |
|---|---|
| Build CLI or Web UI | Go 1.26+ |
| Run with Docker | Docker Compose v2 |
| Deploy the Worker | Node.js 20+ and Wrangler |

## Notes

- Intended as a **local tool or carefully deployed service**. The Web/Worker modes fetch remote pages on your behalf — set `FVF_SIGNING_SECRET` in production (Worker **requires** it; deploy fails closed without a ≥32-char secret) and do not expose private-network access on the public internet.  
- Icon previews always go through a same-origin signed `/api/proxy` so the browser never loads remote (or private) icon URLs directly.  
- Go SSRF checks DNS at dial time; Worker SSRF blocks private IP literals / hostnames and relies on Cloudflare’s egress limits (not full DNS rebinding parity).  
- SVG (and ICO in the Go Web UI) are passthrough only when the payload matches the format (not just the file extension).  
- Behind a reverse proxy, set `FVF_TRUSTED_PROXIES` so rate limits use real client IPs.  
- Security policy: [SECURITY.md](./SECURITY.md). API / deploy: [docs/](./docs/).  
- More detail: `./fvf --help`, source under `cmd/` / `internal/` / `worker/`.

## License

[GNU GPL v3.0](./LICENSE)
