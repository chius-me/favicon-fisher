<p align="right">
  English | <a href="./README.zh.md">简体中文</a>
</p>

<h1 align="center">favicon-fisher</h1>

<p align="center">
  Discover, preview, and download a website's favicon with a CLI, a Go Web UI, or a Cloudflare Worker.
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-GPL--3.0-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-Go-00ADD8.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-fisher?color=success">
  <img alt="CI" src="https://github.com/chius-me/favicon-fisher/actions/workflows/ci.yml/badge.svg">
  <img alt="Container" src="https://github.com/chius-me/favicon-fisher/actions/workflows/container.yml/badge.svg">
</p>

## What It Does

`favicon-fisher` finds favicon candidates for a website, ranks them, and downloads the icon you choose. Use it from the terminal, run the local web app, or deploy the Worker version to Cloudflare.

| Mode | Description | Stack |
|---|---|---|
| **`fvf`** | CLI for favicon discovery and download | Go |
| **`fvf-web`** | Browser-based Web UI + API | Go |
| **`favicon-worker`** | Zero-infra Cloudflare Workers deployment | TypeScript |

Given a URL or bare domain, the app adds `https://` when needed, fetches the page, parses favicon candidates from `<link rel="icon">`, `shortcut icon`, `apple-touch-icon`, `mask-icon`, web manifest icons, and `/favicon.ico`, then ranks the candidates by source and size.

## Requirements

| Task | Requirement |
|---|---|
| Build CLI or Go Web UI | Go 1.26+ |
| Run with Docker | Docker and Docker Compose v2 |
| Deploy Worker | Node.js 20+ and Wrangler |

## Quick Start

### CLI

```bash
go build -o fvf ./cmd/fvf
./fvf --out tmp https://github.com
```

Run `./fvf --help` to see every flag.

### Web UI

```bash
go build -o fvf-web ./cmd/fvf-web
PORT=8080 ./fvf-web
# Open http://localhost:8080
```

### Docker

```bash
docker compose up --build
# Open http://localhost:8080
```

The Compose file is named `docker-compose.yaml`. Docker Compose detects it by default.

### Cloudflare Worker

```bash
cd worker
npm install
npx wrangler deploy
```

See the [`worker/`](./worker/) directory for full Worker documentation.

## Features

- **Smart discovery**: parses `<link rel="icon">`, `shortcut icon`, `apple-touch-icon`, `mask-icon`, web manifest, and `/favicon.ico` fallback
- **URL normalisation**: accepts bare domains (e.g. `github.com`) and normalises to `https://`
- **Candidate ranking**: picks the best icon by size, type, and source priority
- **Batch downloads**: the CLI can download every discovered candidate with `--all`
- **Proxy support**: the CLI accepts `--proxy` and also respects `HTTP_PROXY` / `HTTPS_PROXY`
- **Format conversion**: `fvf-web` downloads **png**, **jpg**, **svg**, or **ico** when the source supports it; Worker browser code supports **png**, **jpg**, **webp**, **ico**, and **svg** passthrough
- **Zero server-side image processing in Worker mode**: raster conversion runs in the browser through Canvas API
- **Cross-platform releases**: pre-built binaries for Linux, macOS, and Windows on the [Releases page](https://github.com/chius-me/favicon-fisher/releases)

## CLI Usage

```bash
fvf [flags] <url>
```

| Flag | Description |
|---|---|
| `-o, --out DIR` | Output directory. Default: `./out` |
| `--json` | Print structured JSON metadata for scripts |
| `--all` | Download every discovered favicon candidate instead of only the best one |
| `--proxy URL` | Use an HTTP proxy, for example `http://127.0.0.1:8080` |

Examples:

```bash
# Download the best favicon
./fvf --out tmp https://github.com

# Download all discovered candidates
./fvf --all --out tmp https://go.dev

# JSON output for scripts
./fvf --json --out tmp https://go.dev

# Use an explicit proxy
./fvf --proxy http://127.0.0.1:8080 https://example.com
```

When you omit `<url>`, `fvf` starts an interactive prompt. JSON mode requires a URL argument.

## Go Web UI

Run the local server:

```bash
go build -o fvf-web ./cmd/fvf-web
PORT=8080 ./fvf-web
```

Open `http://localhost:8080`, enter a URL, preview the candidates, and download the selected icon. The server reads `PORT`; if you omit it, the app listens on `8080`.

## Docker

Build and run the local image:

```bash
docker compose up --build
```

Use another host port without changing the app port inside the container:

```bash
PORT=9090 docker compose up --build
# Open http://localhost:9090
```

Stop the service:

```bash
docker compose down
```

Run the published image directly:

```bash
docker run --rm -p 8080:8080 ghcr.io/chius-me/favicon-fisher:latest
docker run --rm -p 8080:8080 ghcr.io/chius-me/favicon-fisher:v1.0.1
```

Published tags: `:latest` for the default branch, `:main`, release tags such as `:vX.Y.Z`, and commit tags such as `:sha-<hash>`.

## Web API

The Go Web UI and Worker share the preview endpoint. Download behavior differs because the Worker performs conversion in the browser.

Download and proxy endpoints require a short-lived **HMAC token** returned by preview (prevents open proxy abuse).

### `POST /api/preview`

Discover favicon candidates for a URL (HTML icons, web manifest icons, `/favicon.ico`). Extensionless CDN URLs are probed for `Content-Type` when needed.

```json
{
  "url": "https://github.com"
}
```

Response:

```json
{
  "input_url": "https://github.com",
  "page_url": "https://github.com",
  "recommended_icon_url": "https://github.githubassets.com/favicons/favicon.svg",
  "icons": [
    {
      "icon_url": "https://github.githubassets.com/favicons/favicon.svg",
      "token": "<hmac-token>",
      "source_rel": "icon",
      "content_type": "image/svg+xml",
      "allowed_types": ["svg"]
    }
  ]
}
```

### `POST /api/download` (fvf-web)

Download an icon in a chosen format. Requires the `token` from preview. The response body is the binary file.

```json
{
  "icon_url": "https://github.githubassets.com/favicons/favicon.svg",
  "format": "png",
  "token": "<hmac-token>"
}
```

### `GET /api/proxy?url=<icon_url>&token=<hmac-token>` (Worker)

Proxy an icon download through the Worker so browser code can bypass remote CORS restrictions. The token is required.

## Security

`fvf-web` and the Worker act as server-side fetchers. Public deployments apply these controls by default:

| Control | Behavior |
|---|---|
| **SSRF guards** | Only `http`/`https`; blocks private, loopback, link-local, CGNAT, and metadata-style targets; re-validates redirects |
| **Signed tokens** | `/api/download` and `/api/proxy` require a preview-issued HMAC token (TTL 15 minutes) |
| **Body limits** | HTML ≤ 5 MiB, icons ≤ 5 MiB, manifests ≤ 1 MiB, JSON requests ≤ 64 KiB |
| **Rate limit** | 60 API requests / minute / client IP (best-effort) |
| **Security headers** | `X-Content-Type-Options`, `CSP`, `X-Frame-Options`, `Referrer-Policy` |
| **Error hygiene** | Upstream dial/DNS details are not leaked to clients |

Environment variables:

| Variable | Applies to | Description |
|---|---|---|
| `FVF_SIGNING_SECRET` | Web + Worker | HMAC secret for download/proxy tokens. Set a stable value in production. |
| `FVF_ALLOW_PRIVATE` | Web + Worker | Set to `1` only on trusted private networks. **Never** enable on the public internet. |
| `PORT` | Web | Listen port (default `8080`) |

The **CLI** allows private/local targets (developer servers) but still enforces `http`/`https`, User-Agent, timeouts, and body size limits.

## Cloudflare Worker

The Worker variant serves the same UI style with static assets and a TypeScript Worker API. It does not run Go image conversion on the server. Browser code downloads icon bytes through `/api/proxy` and converts raster formats with Canvas.

```bash
cd worker
npm install
npx wrangler secret put FVF_SIGNING_SECRET
npx wrangler deploy
```

See [`worker/README.md`](./worker/README.md) for Worker-specific commands and deployment notes.

## Development

Build both Go entrypoints:

```bash
go build ./cmd/fvf ./cmd/fvf-web
```

Run tests:

```bash
go test ./...
go test -race ./...
```

Run Worker checks:

```bash
cd worker
npm install
npm run check
npm run build
```

Shared discovery fixtures live under `testdata/golden/`.

## Project Structure

| Path | Description |
|---|---|
| `cmd/fvf/` | CLI entrypoint |
| `cmd/fvf-web/` | Web server entrypoint |
| `internal/fetcher/` | Core discovery, ranking and download logic |
| `internal/convert/` | Format conversion (Go image processing) |
| `internal/security/` | SSRF, signing, rate limits, headers, body limits |
| `internal/web/` | API handlers and embedded static assets |
| `worker/` | Cloudflare Workers variant (TypeScript, zero server-side processing) |
| `testdata/golden/` | Shared HTML/manifest fixtures for discovery tests |

## Notes

- SVG output is passthrough-only and requires an SVG source.
- ICO output in `fvf-web` is passthrough-only and requires an ICO source.
- The Worker variant creates ICO output in the browser by wrapping PNG bytes in a minimal ICO container.
- CLI and web requests use a 15-second timeout for slow servers.
- Docker image runs as non-root (`65532`). Compose enables `read_only` root filesystem with a `/tmp` tmpfs.

## License

[GNU GPL v3.0](./LICENSE)
