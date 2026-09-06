# Deploying Cloudflare Worker

Production: **https://icon.chius.dev** (`wrangler.jsonc` binds that custom domain).

```bash
cd worker
npm install
npx wrangler secret put FVF_SIGNING_SECRET   # ≥ 32 characters, required
npx wrangler deploy
```

The Worker **fails closed** if `FVF_SIGNING_SECRET` is missing or shorter than 32 characters.

## GitHub Actions deploy

Push to `main` under `worker/**` (or `workflow_dispatch`). The workflow:

1. `npm ci` / check / build  
2. Verifies `FVF_SIGNING_SECRET` exists via `wrangler secret list`  
3. `wrangler deploy`

Required repo secrets: `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`.

## Optional vars

| Variable | Description |
|----------|-------------|
| `FVF_ALLOW_PRIVATE` | `1` only for trusted private networks — never on public internet |

## SSRF note

Worker SSRF is not fully equivalent to Go dialer checks. See [security.md](./security.md).
