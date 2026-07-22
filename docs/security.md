# Security notes

## SSRF

### Go (`fvf-web`, CLI with `DefaultPolicy`)

- Only `http` / `https`.
- Blocks private, loopback, link-local, metadata, and other reserved ranges after DNS resolution.
- Custom `DialContext` re-resolves and re-checks the peer IP (reduces DNS rebinding / TOCTOU risk).
- Redirects are validated hop-by-hop.

### Cloudflare Worker

- Application checks: scheme, userinfo, localhost / `.local` / `.internal`, and **IP literals** for private/reserved ranges.
- Does **not** resolve public hostnames to A/AAAA and re-check (Workers `fetch` uses Cloudflare’s resolver and platform egress rules).
- Treat Worker SSRF protection as: **literal + hostname denylist + platform network limits**, not full parity with Go.

## Tokens

Preview issues HMAC-SHA256 tokens bound to:

```text
fetch\n<icon_url>\n<expiry_unix>
```

Format: `<expiry>.<base64url(mac)>`. Used by `/api/proxy` and Go `/api/download`. Purpose binding prevents future endpoints from reusing the same token for a different privilege.

## Rate limiting (Go)

| Variable | Meaning |
|----------|---------|
| `FVF_RATE_LIMIT` | Max API requests per window (default 60) |
| `FVF_RATE_WINDOW` | Duration string, e.g. `1m` (default) |
| `FVF_TRUSTED_PROXIES` | Comma-separated CIDRs/IPs; only then trust `CF-Connecting-IP`, `X-Real-IP`, `X-Forwarded-For` |

Without trusted proxies, only `RemoteAddr` is used (correct for direct exposure; wrong if every client appears as the reverse proxy).

## Image handling

- Compressed body cap: 5 MiB.
- Decode config cap: 4096 per side, 16M pixels.
- Output formats allowlist: `png`, `jpg`, `svg`, `ico`.
- SVG/ICO passthrough requires magic-byte / structure checks, not just file extension.
