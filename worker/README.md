<p align="right">
  English | <a href="./README.zh.md">简体中文</a>
</p>

<h1 align="center">favicon-worker</h1>

A Cloudflare Workers implementation of favicon-fisher — zero server-side image processing, all format conversion happens in-browser via the Canvas API.

## Deploy

```bash
npm install
npx wrangler deploy
```

## Dev

```bash
npx wrangler dev
# Open http://localhost:8787
```

## Required Secrets

Set these in your Cloudflare dashboard or GitHub Actions secrets:

- `CLOUDFLARE_API_TOKEN` — API Token with Workers Scripts Edit permission
- `CLOUDFLARE_ACCOUNT_ID` — Your Cloudflare Account ID

## Features

- **Canvas-based conversion** — download as `png`, `jpg`, `webp`, or `ico`
- **SVG passthrough** — when the source is SVG
- **Web manifest discovery** — fetches `manifest.json` icons
- **CORS proxy** — `GET /api/proxy?url=...` bypasses browser CORS
- **Workers Static Assets** — single deploy for API + frontend

## API

Same as the Go version: `POST /api/preview` for discovery, `GET /api/proxy` for proxied downloads.

See the [root README](../README.md) for API details.
