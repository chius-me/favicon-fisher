<p align="right">
  English | <a href="./README.zh.md">简体中文</a>
</p>

<h1 align="center">favicon-fisher</h1>

<p align="center">
  A small Go toolset that discovers a website's favicon, previews candidates in a Web UI, and downloads the chosen icon in a supported format.
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-MIT-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-Go-00ADD8.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-fisher?color=success">
  <img alt="CI" src="https://github.com/chius-me/favicon-fisher/actions/workflows/ci.yml/badge.svg">
</p>

## Overview

`favicon-fisher` now ships two entrypoints:

- `fvf`: the existing CLI for favicon discovery and download
- `fvf-web`: a lightweight Web UI/API for previewing discovered icons and downloading them as another format

For a given URL or domain, it normalizes `https://`, fetches the HTML, parses favicon candidates from `<link rel="icon">`, `shortcut icon`, `apple-touch-icon`, and fallback `/favicon.ico`, ranks the candidates, and lets the user preview the result before downloading.

## v1.0.1 Web MVP Features

- **Web UI preview:** enter a URL and preview discovered favicon candidates in the browser
- **Format download:** download the selected icon as `png` or `jpg`
- **SVG passthrough:** if the source icon is SVG, `svg` download is also available
- **Shared Go core:** CLI and Web server reuse the same discovery/fetcher logic
- **Container-ready:** includes `Dockerfile` and `docker-compose.yml`

## Quick Start

Build the CLI:

```bash
go build -o fvf ./cmd/fvf
```

Build the Web server:

```bash
go build -o fvf-web ./cmd/fvf-web
```

## Run the Web UI

```bash
PORT=8080 ./fvf-web
```

Then open:

```text
http://localhost:8080
```

## Web API

### `POST /api/preview`

Request:

```json
{
  "url": "https://github.com"
}
```

Response shape:

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

### `POST /api/download`

Request:

```json
{
  "icon_url": "https://github.githubassets.com/favicons/favicon.svg",
  "format": "png"
}
```

Response:
- binary file payload
- `Content-Disposition: attachment; filename="..."`

## CLI Usage

Basic run, saving to a `tmp` directory:

```bash
./fvf --out tmp https://github.com
```

JSON mode:

```bash
./fvf --json --out tmp https://go.dev
```

## Docker

Build and run with Docker Compose:

```bash
docker compose up --build
```

Then open `http://localhost:8080`.

## Tests

```bash
go test ./...
```

## Project Structure

- `cmd/fvf/`: CLI entrypoint
- `cmd/fvf-web/`: Web server entrypoint
- `internal/fetcher/`: favicon discovery and download logic
- `internal/convert/`: basic format conversion logic for Web downloads
- `internal/web/`: API handlers and embedded static assets
- `docs/plans/`: historical implementation plans

## Notes

- Current Web download conversion supports `png` and `jpg` for raster outputs.
- SVG output is passthrough-only and only available when the source icon is SVG.
- The CLI behavior remains available and unchanged for script usage.
