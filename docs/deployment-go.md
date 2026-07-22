# Deploying Go Web (`fvf-web`)

## Quick start

```bash
go build -o fvf-web ./cmd/fvf-web
export FVF_SIGNING_SECRET="$(openssl rand -hex 32)"
./fvf-web   # :8080
```

Docker:

```bash
docker compose up --build
```

Image: `ghcr.io/chius-me/favicon-fisher:latest` (also version tags).

## Environment

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | no | Listen port (default `8080`) |
| `FVF_SIGNING_SECRET` | recommended | HMAC secret; ephemeral if unset |
| `FVF_ALLOW_PRIVATE` | no | `1` allows private IPs (never on public internet) |
| `FVF_RATE_LIMIT` | no | API requests per window (default 60) |
| `FVF_RATE_WINDOW` | no | e.g. `1m`, `30s` |
| `FVF_TRUSTED_PROXIES` | if behind proxy | e.g. `10.0.0.0/8,172.16.0.0/12` |

## Reverse proxy

Example: only trust the proxy network so rate limits use real client IPs:

```bash
export FVF_TRUSTED_PROXIES=10.0.0.0/8
export FVF_SIGNING_SECRET=...
```

Ensure the proxy sets `X-Real-IP` or `X-Forwarded-For` (or `CF-Connecting-IP` on Cloudflare).
