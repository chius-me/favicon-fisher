<p align="right">
  English | <a href="./README.zh.md">简体中文</a>
</p>

<h1 align="center">favicon-fisher</h1>

<p align="center">
  Discover, preview, and download a website's favicon — via CLI, Web UI, or Cloudflare Worker.
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-MIT-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-Go-00ADD8.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-fisher?color=success">
  <img alt="CI" src="https://github.com/chius-me/favicon-fisher/actions/workflows/ci.yml/badge.svg">
  <img alt="Container" src="https://github.com/chius-me/favicon-fisher/actions/workflows/container.yml/badge.svg">
</p>

## What is this?

`favicon-fisher` is a minimalist tool that discovers a website's favicon, lets you preview the candidates, and downloads the one you want. It ships in three flavours:

| Flavour | Description | Stack |
|---|---|---|
| **`fvf`** | CLI — favicon discovery and download | Go |
| **`fvf-web`** | Browser-based Web UI + API | Go |
| **`favicon-worker`** | Zero-infra Cloudflare Workers deployment | TypeScript |

Given a URL or domain, it normalises `https://`, fetches the HTML, parses favicon candidates from `<link rel="icon">`, `shortcut icon`, `apple-touch-icon`, web manifest icons, and the `/favicon.ico` fallback, ranks them by relevance, and lets you preview and download in your format of choice.

## Quick Start

### CLI

```bash
go build -o fvf ./cmd/fvf
./fvf --out tmp https://github.com
```

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

### Cloudflare Worker

```bash
cd worker
npm install
npx wrangler deploy
```

See the [`worker/`](./worker/) directory for full Worker documentation.

## Features

- **Smart discovery** — parses `<link rel="icon">`, `shortcut icon`, `apple-touch-icon`, `mask-icon`, web manifest, and `/favicon.ico` fallback
- **URL normalisation** — accepts bare domains (e.g. `github.com`) and normalises to `https://`
- **Candidate ranking** — picks the best icon by size, type, and source priority
- **Format conversion** — download as **png**, **jpg**, **webp** or **ico** (Worker) / **png**, **jpg** or **svg** (fvf-web)
- **Zero server-side image processing** (Worker) — all raster conversion runs in-browser via Canvas API
- **Cross-platform releases** — pre-built binaries for Linux, macOS, and Windows on the [Releases page](https://github.com/chius-me/favicon-fisher/releases)

## Web API

Both `fvf-web` and `favicon-worker` expose the same API shape.

### `POST /api/preview`

Discover favicon candidates for a URL.

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
      "source_rel": "icon",
      "content_type": "image/svg+xml",
      "allowed_types": ["svg"]
    }
  ]
}
```

### `POST /api/download` (fvf-web)

Download an icon in a chosen format. Returns the binary file.

```json
{
  "icon_url": "https://github.githubassets.com/favicons/favicon.svg",
  "format": "png"
}
```

### `GET /api/proxy?url=<icon_url>` (Worker)

Proxies an icon download to bypass browser CORS restrictions.

## CLI Usage

```bash
fvf [--out DIR] [--json] <url>
```

| Flag | Description |
|---|---|
| `--out DIR` | Output directory (default: current dir) |
| `--json` | Structured JSON output for scripting |

Examples:

```bash
# Basic usage
./fvf --out tmp https://github.com

# JSON mode for programmatic use
./fvf --json --out tmp https://go.dev
```

## Container Images

Pre-built multi-arch images (`linux/amd64` + `linux/arm64`) are available on GHCR:

```bash
docker run --rm -p 8080:8080 ghcr.io/chius-me/favicon-fisher:latest
docker run --rm -p 8080:8080 ghcr.io/chius-me/favicon-fisher:v1.0.1
```

Tags: `:latest` (default branch), `:main`, `:vX.Y.Z` (release tags), `:sha-<hash>`.

## Tests

```bash
go test ./...
```

## Project Structure

| Path | Description |
|---|---|
| `cmd/fvf/` | CLI entrypoint |
| `cmd/fvf-web/` | Web server entrypoint |
| `internal/fetcher/` | Core discovery, ranking and download logic |
| `internal/convert/` | Format conversion (Go image processing) |
| `internal/web/` | API handlers and embedded static assets |
| `worker/` | Cloudflare Workers variant (TypeScript, zero server-side processing) |

## Notes

- SVG output is passthrough-only — only available when the source icon is SVG.
- The Worker variant handles ICO output by wrapping a PNG inside a minimal ICO container.
- CLI has a 10-second timeout for slow servers.

## License

[MIT](./LICENSE)
