<p align="right">
  English | <a href="./README.zh.md">简体中文</a>
</p>

# favicon-worker

Cloudflare Workers build of [favicon-fisher](../README.md) — same product, edge-hosted UI, browser-side format conversion.

```bash
npm install
npx wrangler secret put FVF_SIGNING_SECRET   # once, for production
npx wrangler deploy

# local
npx wrangler dev   # http://localhost:8787
```

Product overview, features, and other run modes: **[root README](../README.md)**.
