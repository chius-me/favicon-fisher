# Security Policy

## Supported versions

Security fixes are applied to the latest release on `main` and the most recent tagged release.

## Reporting a vulnerability

Please **do not** open a public GitHub Issue for security vulnerabilities.

Report privately via one of:

- GitHub Security Advisories for this repository (preferred)
- Email the maintainer listed on the GitHub profile

Include steps to reproduce, impact, and any proposed fix if available. You should receive an acknowledgement within a few days.

## Threat model (summary)

favicon-fisher (Web / Worker modes) fetches remote HTML and icons on behalf of the operator. It is intended as a **local tool or carefully deployed service**, not an open anonymous internet proxy.

Mitigations include:

| Control | Go Web | Worker |
|--------|--------|--------|
| SSRF: private/reserved IP block | DNS pre-check + dial re-check | Hostname / IP-literal checks; relies on Cloudflare egress limits |
| Signed short-lived tokens for icon fetch | HMAC (`fetch` purpose) | HMAC (`fetch` purpose), fail-closed without `FVF_SIGNING_SECRET` |
| Same-origin icon preview proxy | `/api/proxy` | `/api/proxy` |
| Body size limits | Yes | Yes |
| Image pixel limits | Yes | Canvas size cap (browser conversion) |
| Candidate / probe caps | Yes | Yes |
| Rate limiting | Per-IP fixed window; optional trusted proxies | Per-isolate CF-Connecting-IP |

Worker SSRF is **not** fully equivalent to Go’s dialer DNS re-validation. See `docs/security.md`.

## Production checklist

1. Set a strong `FVF_SIGNING_SECRET` (≥ 32 characters). Worker refuses to run without it.
2. Do **not** set `FVF_ALLOW_PRIVATE=1` on the public internet.
3. Behind a reverse proxy, set `FVF_TRUSTED_PROXIES` to the proxy CIDRs so rate limits use real client IPs (`CF-Connecting-IP` / `X-Real-IP` / `X-Forwarded-For`).
4. Optionally tune `FVF_RATE_LIMIT` and `FVF_RATE_WINDOW`.
