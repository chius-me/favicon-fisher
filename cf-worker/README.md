<p align="right">
  English | <a href="./README.zh.md">简体中文</a>
</p>

<h1 align="center">favicon-worker</h1>

<p align="center">
  A Cloudflare Worker that discovers a website's favicon, previews candidates in a Web UI, and downloads the chosen icon in a supported format — with zero server-side image processing.
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-GPL--3.0-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-JavaScript-F7DF1E.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-worker?color=success">
  <img alt="Deploy" src="https://github.com/chius-me/favicon-worker/actions/workflows/deploy.yml/badge.svg">
</p>

## Overview

`favicon-worker` is the Cloudflare Workers rewrite of [favicon-fisher](https://github.com/chius-me/favicon-fisher). For a given URL or domain, it normalizes `https://`, fetches the HTML, parses favicon candidates from `<link rel="icon">`, `shortcut icon`, `apple-touch-icon`, `mask-icon`, web manifest icons, and fallback `/favicon.ico`, ranks the candidates, and lets the user preview and download in a chosen format.

All image format conversion is done in the browser via Canvas API — the Worker never processes pixel data.

## Features

- **Web UI preview:** enter a URL and preview discovered favicon candidates in the browser
- **Format download:** download the selected icon as `png`, `jpg`, `webp`, or `ico`
- **SVG passthrough:** if the source icon is SVG, `svg` download is available
- **Web manifest discovery:** asynchronously fetches and parses `manifest.json` for additional icons
- **CORS proxy:** Worker proxies icon downloads to bypass browser CORS restrictions
- **Canvas-based conversion:** all raster conversion happens client-side (no server-side image library needed)
- **ICO output:** generates a valid ICO file by wrapping a PNG inside a minimal ICO header
- **Zero-dependency deployment:** Workers Static Assets — single deploy for API + frontend

## Quick Start

Install dependencies:

```bash
npm install
```

Run locally:

```bash
npx wrangler dev
```

Then open:

```text
http://localhost:8787
```

Deploy to Cloudflare:

```bash
npx wrangler deploy
```

## API

### `POST /api/preview`

Discover favicon candidates for a URL.

Request:

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
      "sizes": null,
      "content_type": "image/svg+xml",
      "allowed_types": ["svg"]
    }
  ]
}
```

### `GET /api/proxy?url=<icon_url>`

Proxy an icon download to bypass CORS. Returns the raw binary with appropriate `Content-Type`.

## CI/CD

Pushing to `main` triggers automatic deployment via GitHub Actions.

Required GitHub Secrets:

- `CLOUDFLARE_API_TOKEN` — Cloudflare API Token with Workers Scripts Edit permission
- `CLOUDFLARE_ACCOUNT_ID` — Cloudflare Account ID

## Project Structure

- `src/worker.js`: Worker entry — API routes and favicon discovery logic
- `public/index.html`: Frontend HTML
- `public/app.js`: Frontend JS — preview rendering, Canvas format conversion, download
- `public/style.css`: Frontend styles
- `.github/workflows/deploy.yml`: GitHub Actions deploy workflow

## Notes

- SVG output is passthrough-only and only available when the source icon is SVG.
- ICO output works by converting the source image to PNG via Canvas and wrapping it in a minimal ICO container.
- For the CLI and Docker-based version, see [favicon-fisher](https://github.com/chius-me/favicon-fisher).

## License

This project is licensed under the [GNU General Public License v3.0](./LICENSE).
