# API reference

Applies to **Go Web** (`fvf-web`) and **Cloudflare Worker** (same paths/shapes unless noted).

All `/api/*` responses are JSON unless downloading binary content. Security headers are set on every response.

## `POST /api/preview`

Discover icon candidates for a page.

### Request

```json
{ "url": "https://example.com" }
```

| Field | Required | Notes |
|-------|----------|--------|
| `url` | yes | Domain or full URL; scheme defaults to `https` when omitted |

Body size limit: 64 KiB.

### Response `200`

```json
{
  "input_url": "https://example.com",
  "page_url": "https://example.com/",
  "recommended_icon_url": "https://example.com/favicon.png",
  "icons": [
    {
      "icon_url": "https://example.com/favicon.png",
      "token": "<expiry>.<hmac>",
      "source_rel": "icon",
      "sizes": "32x32",
      "content_type": "image/png",
      "allowed_types": ["png", "jpg"]
    }
  ]
}
```

| Field | Notes |
|-------|--------|
| `token` | Short-lived HMAC bound to `fetch` purpose + `icon_url` (default TTL 15m) |
| `allowed_types` | Formats the client may request; empty means not previewable |
| `recommended_icon_url` | Best ranked candidate still present in `icons` |

### Errors

| Status | Typical `error` |
|--------|------------------|
| 400 | `url is required`, `URL is not allowed` |
| 429 | `rate limit exceeded` |
| 502 | `no favicon candidates found`, `upstream request failed` |
| 503 | Worker only: `service misconfigured` (missing signing secret) |

---

## `GET /api/proxy` (preview bytes)

Same-origin icon preview. Browser must not load `icon_url` directly.

### Query

| Param | Required |
|-------|----------|
| `url` | yes — remote icon URL |
| `token` | yes — token from preview |

### Response `200`

Raw icon bytes with upstream (or inferred) `Content-Type`.  
`Cache-Control: private, max-age=300`.

### Errors

| Status | Typical `error` |
|--------|------------------|
| 400 | `url parameter is required` |
| 403 | `token is required`, `invalid token`, `token expired` |
| 502 | `upstream request failed` |

---

## `POST /api/download` (Go Web only)

Worker converts in the browser via canvas; Go converts server-side.

### Request

```json
{
  "icon_url": "https://example.com/favicon.png",
  "format": "png",
  "token": "<expiry>.<hmac>"
}
```

| Field | Required | Notes |
|-------|----------|--------|
| `icon_url` | yes | Must match the signed URL |
| `format` | yes | Go: `png` \| `jpg` \| `svg` \| `ico` |
| `token` | yes | Same token as proxy |

### Response `200`

Binary body with `Content-Disposition: attachment; filename="..."`.

SVG/ICO are passthrough only when magic/structure matches; raster outputs re-encode with pixel limits (max dimension 4096, max 16M pixels).

---

## Rate limits

| Mode | Default | Identity |
|------|---------|----------|
| Go | 60 / minute | `RemoteAddr`, or proxy headers if peer ∈ `FVF_TRUSTED_PROXIES` |
| Worker | 60 / minute | `CF-Connecting-IP` (per isolate, best-effort) |

`429` includes `Retry-After` (Go: remaining window seconds).

## Env (Go)

See [deployment-go.md](./deployment-go.md) for `FVF_SIGNING_SECRET`, `FVF_TRUSTED_PROXIES`, `FVF_RATE_LIMIT`, `FVF_RATE_WINDOW`, `FVF_ALLOW_PRIVATE`.
