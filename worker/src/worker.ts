/**
 * favicon-worker — Cloudflare Worker
 * Discovers favicon candidates and proxies icon downloads with SSRF guards,
 * signed tokens, body limits, and rate limiting.
 */

interface Env {
  ASSETS: Fetcher;
  FVF_SIGNING_SECRET?: string;
  FVF_ALLOW_PRIVATE?: string;
}

interface IconCandidate {
  url: string;
  rel: string;
  sizes: string;
  type: string;
  priority: number;
}

interface ManifestIcon {
  src?: string;
  sizes?: string;
  type?: string;
}

interface Manifest {
  icons?: ManifestIcon[];
}

interface PreviewPayload {
  input_url: string;
  page_url: string;
  recommended_icon_url: string | null;
  icons: {
    icon_url: string;
    token: string;
    source_rel: string;
    sizes: string | null;
    content_type: string;
    allowed_types: string[];
  }[];
}

const USER_AGENT = 'FaviconFisher/1.0 (+https://github.com/chius-me/favicon-fisher)';
const MAX_HTML = 5 * 1024 * 1024;
const MAX_ICON = 5 * 1024 * 1024;
const MAX_MANIFEST = 1 * 1024 * 1024;
const MAX_JSON = 64 * 1024;
const TOKEN_TTL_SEC = 15 * 60;
const RATE_LIMIT = 60;
const RATE_WINDOW_MS = 60_000;
const MAX_HTML_ICON_CANDIDATES = 32;
const MAX_MANIFEST_ICONS = 32;
const MAX_TOTAL_CANDIDATES = 48;
const MAX_CONTENT_TYPE_PROBES = 12;
const MAX_REDIRECT_HOPS = 5;
const FETCH_TIMEOUT_MS = 10_000;
const MIN_SIGNING_SECRET_LEN = 32;

// Per-isolate rate limit state (best-effort on Workers).
const rateHits = new Map<string, { count: number; start: number }>();

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    if (request.method === 'OPTIONS') {
      return new Response(null, { status: 204, headers: securityHeaders(request) });
    }

    if (url.pathname.startsWith('/api/')) {
      const ip = request.headers.get('cf-connecting-ip') || 'unknown';
      if (!allowRate(ip)) {
        return jsonError('rate limit exceeded', 429, request);
      }
    }

    if (url.pathname === '/api/preview' && request.method === 'POST') {
      return withSecurityHeaders(request, await handlePreview(request, env));
    }

    if (url.pathname === '/api/proxy' && request.method === 'GET') {
      return withSecurityHeaders(request, await handleProxy(url, env));
    }

    const assetResp = await env.ASSETS.fetch(request);
    return withSecurityHeaders(request, assetResp);
  },
} satisfies ExportedHandler<Env>;

// ─── Preview ───────────────────────────────────────────────────────

async function handlePreview(request: Request, env: Env): Promise<Response> {
  try {
    const rawBody = await readRequestText(request, MAX_JSON);
    const { url: inputUrl } = JSON.parse(rawBody) as { url?: string };
    if (!inputUrl) {
      return jsonError('url is required', 400, request);
    }

    const allowPrivate = env.FVF_ALLOW_PRIVATE === '1';
    const normalized = normalizeUrl(inputUrl);
    await assertSafeUrl(normalized, allowPrivate);

    const resp = await safeFetch(normalized, allowPrivate);
    const followed = await followRedirects(resp, normalized, allowPrivate, MAX_REDIRECT_HOPS);
    if (!followed.ok) {
      return jsonError('upstream request failed', 502, request);
    }

    const pageUrl = followed.url;
    const html = await readResponseText(followed, MAX_HTML);
    const candidates = discoverCandidates(pageUrl, html);

    const manifestHref = findManifestHref(html);
    if (manifestHref) {
      const manifestUrl = resolveUrl(pageUrl, manifestHref);
      if (manifestUrl) {
        try {
          await assertSafeUrl(manifestUrl, allowPrivate);
          const manifestResp = await safeFetch(manifestUrl, allowPrivate);
          const mFollowed = await followRedirects(manifestResp, manifestUrl, allowPrivate, MAX_REDIRECT_HOPS);
          if (mFollowed.ok) {
            const manifestText = await readResponseText(mFollowed, MAX_MANIFEST);
            const manifest = JSON.parse(manifestText) as Manifest;
            const seen = new Set<string>(candidates.map((c) => c.url));
            const base = mFollowed.url;
            if (manifest.icons && Array.isArray(manifest.icons)) {
              let manifestCount = 0;
              for (const icon of manifest.icons) {
                if (manifestCount >= MAX_MANIFEST_ICONS) break;
                if (candidates.length >= MAX_TOTAL_CANDIDATES) break;
                const src = icon.src?.trim();
                if (!src) continue;
                const resolved = resolveUrl(base, src);
                if (resolved && !seen.has(resolved)) {
                  candidates.push({
                    url: resolved,
                    rel: 'manifest',
                    sizes: icon.sizes || '',
                    type: icon.type || '',
                    priority: 15,
                  });
                  seen.add(resolved);
                  manifestCount++;
                }
              }
            }
          }
        } catch {
          // manifest fetch failed, skip
        }
      }
    }

    // Probe content-type for extensionless candidates (budgeted).
    let probes = 0;
    for (const c of candidates) {
      if (c.type || hasImageExtension(c.url)) continue;
      if (probes >= MAX_CONTENT_TYPE_PROBES) break;
      const ct = await probeContentType(c.url, allowPrivate);
      if (ct) c.type = ct;
      probes++;
    }

    candidates.sort((a, b) => {
      if (a.priority !== b.priority) return a.priority - b.priority;
      return sizeScore(b.sizes) - sizeScore(a.sizes);
    });
    if (candidates.length > MAX_TOTAL_CANDIDATES) {
      candidates.length = MAX_TOTAL_CANDIDATES;
    }

    const secret = await signingKey(env);
    const recommended = candidates[0] || null;
    const icons = [];
    for (const c of candidates) {
      const allowed = getAllowedTypes(c.url, c.type);
      if (allowed.length === 0) continue;
      icons.push({
        icon_url: c.url,
        token: await signUrl(secret, c.url),
        source_rel: c.rel,
        sizes: c.sizes || null,
        content_type: guessContentType(c.url, c.type),
        allowed_types: allowed,
      });
    }

    return jsonResponse<PreviewPayload>(
      {
        input_url: inputUrl,
        page_url: pageUrl,
        recommended_icon_url: recommended?.url || null,
        icons,
      },
      200,
      request
    );
  } catch (err: unknown) {
    return jsonError(publicError(err), statusFor(err), request);
  }
}

// ─── Proxy ─────────────────────────────────────────────────────────

async function handleProxy(urlObj: URL, env: Env): Promise<Response> {
  const iconUrl = urlObj.searchParams.get('url');
  const token = urlObj.searchParams.get('token');
  if (!iconUrl) {
    return jsonError('url parameter is required', 400);
  }
  if (!token) {
    return jsonError('token is required', 403);
  }

  try {
    const secret = await signingKey(env);
    if (!(await verifyUrl(secret, iconUrl, token))) {
      return jsonError('invalid token', 403);
    }

    const allowPrivate = env.FVF_ALLOW_PRIVATE === '1';
    await assertSafeUrl(iconUrl, allowPrivate);

    const resp = await safeFetch(iconUrl, allowPrivate);
    const followed = await followRedirects(resp, iconUrl, allowPrivate, MAX_REDIRECT_HOPS);
    if (!followed.ok) {
      return jsonError('upstream request failed', 502);
    }

    const body = await readResponseBytes(followed, MAX_ICON);
    const contentType = followed.headers.get('content-type') || 'application/octet-stream';

    return new Response(body, {
      headers: {
        'Content-Type': contentType,
        'Cache-Control': 'private, max-age=300',
        'X-Content-Type-Options': 'nosniff',
      },
    });
  } catch (err: unknown) {
    return jsonError(publicError(err), statusFor(err));
  }
}

// ─── SSRF / URL safety ─────────────────────────────────────────────

async function assertSafeUrl(raw: string, allowPrivate: boolean): Promise<void> {
  let u: URL;
  try {
    u = new URL(raw);
  } catch {
    throw new Error('invalid URL');
  }
  if (u.protocol !== 'http:' && u.protocol !== 'https:') {
    throw new Error('only http and https URLs are allowed');
  }
  if (u.username || u.password) {
    throw new Error('URLs with userinfo are not allowed');
  }
  const host = u.hostname.toLowerCase();
  if (!host) throw new Error('invalid URL host');

  if (allowPrivate) return;

  if (host === 'localhost' || host.endsWith('.localhost') || host.endsWith('.local')) {
    throw new Error('requests to private or reserved addresses are not allowed');
  }
  if (
    host === 'metadata.google.internal' ||
    host === 'metadata' ||
    host.endsWith('.internal')
  ) {
    throw new Error('requests to private or reserved addresses are not allowed');
  }

  // Block literal IPs that are private/reserved.
  if (isIPLiteral(host) && isBlockedIPLiteral(host)) {
    throw new Error('requests to private or reserved addresses are not allowed');
  }
}

function isIPLiteral(host: string): boolean {
  // IPv4
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(host)) return true;
  // IPv6 (URL hostname without brackets already)
  if (host.includes(':')) return true;
  return false;
}

function isBlockedIPLiteral(host: string): boolean {
  // IPv4
  const m = host.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
  if (m) {
    const a = [Number(m[1]), Number(m[2]), Number(m[3]), Number(m[4])];
    if (a.some((n) => n > 255)) return true;
    if (a[0] === 0) return true;
    if (a[0] === 10) return true;
    if (a[0] === 127) return true;
    if (a[0] === 169 && a[1] === 254) return true;
    if (a[0] === 172 && a[1] >= 16 && a[1] <= 31) return true;
    if (a[0] === 192 && a[1] === 168) return true;
    if (a[0] === 100 && a[1] >= 64 && a[1] <= 127) return true;
    if (a[0] === 192 && a[1] === 0 && a[2] === 0) return true;
    if (a[0] === 192 && a[1] === 0 && a[2] === 2) return true;
    if (a[0] === 198 && a[1] === 51 && a[2] === 100) return true;
    if (a[0] === 203 && a[1] === 0 && a[2] === 113) return true;
    if (a[0] === 198 && (a[1] === 18 || a[1] === 19)) return true;
    if (a[0] >= 224) return true; // multicast / reserved
    return false;
  }
  // IPv6 simplified checks
  const h = host.toLowerCase();
  if (h === '::1' || h === '0:0:0:0:0:0:0:1') return true;
  if (h.startsWith('fc') || h.startsWith('fd')) return true; // ULA rough
  if (h.startsWith('fe80')) return true;
  if (h.startsWith('::ffff:')) {
    const v4 = h.slice('::ffff:'.length);
    return isBlockedIPLiteral(v4);
  }
  return false;
}

async function followRedirects(
  resp: Response,
  originalUrl: string,
  allowPrivate: boolean,
  maxHops: number
): Promise<Response> {
  let current = resp;
  let currentUrl = originalUrl;
  for (let i = 0; i < maxHops; i++) {
    if (current.status < 300 || current.status >= 400) {
      return current;
    }
    const loc = current.headers.get('location');
    if (!loc) return current;
    const next = new URL(loc, currentUrl).href;
    await assertSafeUrl(next, allowPrivate);
    currentUrl = next;
    current = await safeFetch(next, allowPrivate);
  }
  throw new Error('too many redirects');
}

async function safeFetch(url: string, _allowPrivate: boolean): Promise<Response> {
  return fetch(url, {
    headers: { 'User-Agent': USER_AGENT },
    redirect: 'manual',
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
}

// ─── Signing ───────────────────────────────────────────────────────

async function signingKey(env: Env): Promise<CryptoKey> {
  if (!env.FVF_SIGNING_SECRET || env.FVF_SIGNING_SECRET.length < MIN_SIGNING_SECRET_LEN) {
    throw new Error('FVF_SIGNING_SECRET is not configured');
  }
  const raw = new TextEncoder().encode(env.FVF_SIGNING_SECRET);
  return crypto.subtle.importKey('raw', raw, { name: 'HMAC', hash: 'SHA-256' }, false, [
    'sign',
    'verify',
  ]);
}

async function signUrl(key: CryptoKey, iconUrl: string): Promise<string> {
  const exp = Math.floor(Date.now() / 1000) + TOKEN_TTL_SEC;
  const payload = `${iconUrl}\n${exp}`;
  const sig = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(payload));
  return `${exp}.${bufferToBase64Url(sig)}`;
}

async function verifyUrl(key: CryptoKey, iconUrl: string, token: string): Promise<boolean> {
  const parts = token.split('.');
  if (parts.length !== 2) return false;
  const exp = Number(parts[0]);
  if (!Number.isFinite(exp) || Math.floor(Date.now() / 1000) > exp) return false;
  const payload = `${iconUrl}\n${exp}`;
  const expected = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(payload));
  const provided = base64UrlToBuffer(parts[1]);
  if (!provided || provided.byteLength !== expected.byteLength) return false;
  // constant-time compare
  const a = new Uint8Array(expected);
  const b = new Uint8Array(provided);
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a[i] ^ b[i];
  return diff === 0;
}

function bufferToBase64Url(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let s = '';
  for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i]);
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function base64UrlToBuffer(s: string): ArrayBuffer | null {
  try {
    const pad = s.length % 4 === 0 ? '' : '='.repeat(4 - (s.length % 4));
    const b64 = s.replace(/-/g, '+').replace(/_/g, '/') + pad;
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out.buffer;
  } catch {
    return null;
  }
}

// ─── Body limits ───────────────────────────────────────────────────

async function readRequestText(request: Request, limit: number): Promise<string> {
  const buf = await request.arrayBuffer();
  if (buf.byteLength > limit) throw new Error('request body too large');
  return new TextDecoder().decode(buf);
}

async function readResponseText(resp: Response, limit: number): Promise<string> {
  const bytes = await readResponseBytes(resp, limit);
  return new TextDecoder().decode(bytes);
}

async function readResponseBytes(resp: Response, limit: number): Promise<ArrayBuffer> {
  if (!resp.body) return new ArrayBuffer(0);
  const reader = resp.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > limit) {
      try {
        await reader.cancel();
      } catch {
        /* ignore */
      }
      throw new Error('response too large');
    }
    chunks.push(value);
  }
  const out = new Uint8Array(total);
  let offset = 0;
  for (const c of chunks) {
    out.set(c, offset);
    offset += c.byteLength;
  }
  return out.buffer;
}

// ─── Discovery ─────────────────────────────────────────────────────

function discoverCandidates(pageUrl: string, html: string): IconCandidate[] {
  const candidates: IconCandidate[] = [];
  const seen = new Set<string>();
  let htmlCount = 0;

  const linkRe = /<link\b[^>]*>/gi;
  let match: RegExpExecArray | null;
  while ((match = linkRe.exec(html)) !== null) {
    const tag = match[0];
    const rel = getAttr(tag, 'rel')?.toLowerCase().trim() || '';
    const href = getAttr(tag, 'href')?.trim() || '';
    const sizes = getAttr(tag, 'sizes')?.trim() || '';
    const type = getAttr(tag, 'type')?.trim() || '';

    if (href && isIconRel(rel) && htmlCount < MAX_HTML_ICON_CANDIDATES) {
      const resolved = resolveUrl(pageUrl, href);
      if (resolved && !seen.has(resolved)) {
        candidates.push({ url: resolved, rel, sizes, type, priority: relPriority(rel) });
        seen.add(resolved);
        htmlCount++;
      }
    }
  }

  try {
    const fallback = new URL('/favicon.ico', pageUrl).href;
    if (!seen.has(fallback) && candidates.length < MAX_TOTAL_CANDIDATES) {
      candidates.push({ url: fallback, rel: 'fallback', sizes: '', type: '', priority: 100 });
      seen.add(fallback);
    }
  } catch {
    // skip
  }

  return candidates;
}

function findManifestHref(html: string): string | null {
  const linkRe = /<link\b[^>]*>/gi;
  let match: RegExpExecArray | null;
  while ((match = linkRe.exec(html)) !== null) {
    const tag = match[0];
    const rel = getAttr(tag, 'rel')?.toLowerCase().trim() || '';
    if (rel === 'manifest') {
      return getAttr(tag, 'href')?.trim() || null;
    }
  }
  return null;
}

function getAttr(tag: string, name: string): string | null {
  // Quoted attributes: rel="icon" or rel='icon'
  const quoted = new RegExp(`${name}\\s*=\\s*["']([^"']*)["']`, 'i');
  const qm = tag.match(quoted);
  if (qm) return qm[1];
  // Unquoted attributes: rel=icon href=/favicon.ico
  const unquoted = new RegExp(`${name}\\s*=\\s*([^\\s>]+)`, 'i');
  const um = tag.match(unquoted);
  return um ? um[1] : null;
}

function isIconRel(rel: string): boolean {
  const parts = rel.split(/\s+/);
  return parts.some(
    (p) =>
      p === 'icon' ||
      p === 'shortcut' ||
      p === 'apple-touch-icon' ||
      p === 'apple-touch-icon-precomposed' ||
      p === 'mask-icon'
  );
}

function relPriority(rel: string): number {
  if (rel === 'icon' || rel === 'shortcut icon') return 10;
  if (rel === 'manifest') return 15;
  if (rel === 'apple-touch-icon' || rel === 'apple-touch-icon-precomposed') return 20;
  if (rel === 'mask-icon') return 30;
  if (rel.includes('icon')) return 40;
  return 90;
}

function sizeScore(sizes: string): number {
  let max = 0;
  const maxDim = 16384;
  for (const part of sizes.toLowerCase().split(/\s+/)) {
    let [w, h] = part.split('x').map(Number);
    if (!w || !h || w <= 0 || h <= 0) continue;
    w = Math.min(w, maxDim);
    h = Math.min(h, maxDim);
    max = Math.max(max, w * h);
  }
  return max;
}

function normalizeUrl(raw: string): string {
  let trimmed = raw.trim();
  if (!trimmed.includes('://')) trimmed = 'https://' + trimmed;
  return trimmed;
}

function resolveUrl(base: string, href: string): string {
  try {
    const u = new URL(href, base);
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return '';
    return u.href;
  } catch {
    return '';
  }
}

function hasImageExtension(url: string): boolean {
  const path = url.split(/[?#]/)[0].toLowerCase();
  return ['.png', '.jpg', '.jpeg', '.svg', '.ico', '.gif', '.webp'].some((e) => path.endsWith(e));
}

async function probeContentType(url: string, allowPrivate: boolean): Promise<string> {
  try {
    await assertSafeUrl(url, allowPrivate);
    const resp = await fetch(url, {
      method: 'HEAD',
      headers: { 'User-Agent': USER_AGENT },
      redirect: 'manual',
      signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
    });
    const followed = await followRedirects(resp, url, allowPrivate, Math.min(3, MAX_REDIRECT_HOPS));
    if (followed.ok) {
      return followed.headers.get('content-type') || '';
    }
  } catch {
    // ignore
  }
  return '';
}

function guessContentType(url: string, declaredType: string): string {
  if (declaredType && declaredType.includes('/')) return declaredType;
  const ext = url.split('?')[0].split('.').pop()!.toLowerCase();
  const map: Record<string, string> = {
    svg: 'image/svg+xml',
    png: 'image/png',
    ico: 'image/x-icon',
    jpg: 'image/jpeg',
    jpeg: 'image/jpeg',
    gif: 'image/gif',
    webp: 'image/webp',
  };
  return map[ext] || 'application/octet-stream';
}

function getAllowedTypes(url: string, declaredType: string): string[] {
  const path = url.split(/[?#]/)[0].toLowerCase();
  const ct = (declaredType || '').toLowerCase();
  if (path.endsWith('.svg') || ct.includes('image/svg+xml')) return ['svg'];
  if (path.endsWith('.ico') || ct.includes('image/x-icon') || ct.includes('image/vnd.microsoft.icon')) {
    return ['ico', 'png', 'jpg', 'webp'];
  }
  if (path.endsWith('.png') || ct.includes('image/png')) return ['png', 'jpg', 'webp', 'ico'];
  if (path.endsWith('.jpg') || path.endsWith('.jpeg') || ct.includes('image/jpeg')) {
    return ['jpg', 'png', 'webp', 'ico'];
  }
  if (path.endsWith('.gif') || ct.includes('image/gif')) return ['png', 'jpg', 'webp', 'ico'];
  if (path.endsWith('.webp') || ct.includes('image/webp')) return ['webp', 'png', 'jpg', 'ico'];
  if (ct.startsWith('image/')) return ['png', 'jpg', 'webp', 'ico'];
  // Unknown / non-image: do not advertise convertible formats.
  return [];
}

// ─── Rate limit / errors / headers ─────────────────────────────────

function allowRate(key: string): boolean {
  const now = Date.now();
  const v = rateHits.get(key);
  if (!v || now - v.start >= RATE_WINDOW_MS) {
    rateHits.set(key, { count: 1, start: now });
    return true;
  }
  if (v.count >= RATE_LIMIT) return false;
  v.count++;
  return true;
}

function publicError(err: unknown): string {
  const message = err instanceof Error ? err.message : 'Unknown error';
  const lower = message.toLowerCase();
  if (lower.includes('fvf_signing_secret')) return 'service misconfigured';
  if (lower.includes('private or reserved') || lower.includes('only http')) return 'URL is not allowed';
  if (lower.includes('token')) return message;
  if (lower.includes('rate limit')) return 'rate limit exceeded';
  if (lower.includes('too large') || lower.includes('exceeds')) return 'response too large';
  if (lower.includes('redirect')) return 'URL is not allowed';
  if (lower.includes('url is required')) return 'url is required';
  if (lower.includes('abort') || lower.includes('timeout')) return 'upstream request timed out';
  return 'request failed';
}

function statusFor(err: unknown): number {
  const message = err instanceof Error ? err.message : '';
  const lower = message.toLowerCase();
  if (lower.includes('fvf_signing_secret')) return 503;
  if (lower.includes('required') || lower.includes('invalid url') || lower.includes('only http')) return 400;
  if (lower.includes('token')) return 403;
  if (lower.includes('rate')) return 429;
  if (lower.includes('private') || lower.includes('not allowed')) return 400;
  return 502;
}

function securityHeaders(request?: Request): Record<string, string> {
  const h: Record<string, string> = {
    'X-Content-Type-Options': 'nosniff',
    'X-Frame-Options': 'DENY',
    'Referrer-Policy': 'no-referrer',
    'Content-Security-Policy':
      "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'",
    'Permissions-Policy': 'camera=(), microphone=(), geolocation=()',
  };
  // Same-origin UI only — do not open CORS to the world.
  if (request) {
    const origin = request.headers.get('Origin');
    if (origin) {
      try {
        const reqHost = new URL(request.url).host;
        if (new URL(origin).host === reqHost) {
          h['Access-Control-Allow-Origin'] = origin;
          h['Access-Control-Allow-Methods'] = 'GET, POST, OPTIONS';
          h['Access-Control-Allow-Headers'] = 'Content-Type';
          h['Vary'] = 'Origin';
        }
      } catch {
        // ignore
      }
    }
  }
  return h;
}

function withSecurityHeaders(request: Request, resp: Response): Response {
  const headers = new Headers(resp.headers);
  for (const [k, v] of Object.entries(securityHeaders(request))) {
    if (!headers.has(k)) headers.set(k, v);
  }
  return new Response(resp.body, {
    status: resp.status,
    statusText: resp.statusText,
    headers,
  });
}

function jsonResponse<T>(data: T, status = 200, request?: Request): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json', ...securityHeaders(request) },
  });
}

function jsonError(message: string, status = 500, request?: Request): Response {
  return jsonResponse({ error: message }, status, request);
}
