/**
 * favicon-worker — Cloudflare Worker
 * Discovers favicon candidates from a URL and proxies icon downloads (bypass CORS).
 */

interface Env {
  ASSETS: Fetcher;
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
    source_rel: string;
    sizes: string | null;
    content_type: string;
    allowed_types: string[];
  }[];
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    // CORS preflight
    if (request.method === 'OPTIONS') {
      return new Response(null, { status: 204, headers: corsHeaders() });
    }

    // API routes
    if (url.pathname === '/api/preview' && request.method === 'POST') {
      return handlePreview(request);
    }

    if (url.pathname === '/api/proxy' && request.method === 'GET') {
      return handleProxy(url);
    }

    // Static assets fall through to Workers Static Assets
    return env.ASSETS.fetch(request);
  },
} satisfies ExportedHandler<Env>;

// ─── Preview: discover favicon candidates ──────────────────────────

async function handlePreview(request: Request): Promise<Response> {
  try {
    const { url: inputUrl } = (await request.json()) as { url?: string };
    if (!inputUrl) {
      return jsonError('url is required', 400);
    }

    const normalized = normalizeUrl(inputUrl);
    const resp = await fetch(normalized, {
      headers: { 'User-Agent': 'Mozilla/5.0 (compatible; FaviconFisher/1.0)' },
      redirect: 'follow',
    });

    if (!resp.ok) {
      return jsonError(`fetch page failed: status ${resp.status}`, 502);
    }

    const pageUrl = resp.url;
    const html = await resp.text();
    const candidates = discoverCandidates(pageUrl, html);

    // Try fetching web manifest for additional icons
    const manifestHref = findManifestHref(html);
    if (manifestHref) {
      const manifestUrl = resolveUrl(pageUrl, manifestHref);
      if (manifestUrl) {
        try {
          const manifestResp = await fetch(manifestUrl, {
            headers: { 'User-Agent': 'Mozilla/5.0 (compatible; FaviconFisher/1.0)' },
            redirect: 'follow',
          });
          if (manifestResp.ok) {
            const manifest = (await manifestResp.json()) as Manifest;
            const seen = new Set<string>(candidates.map((c) => c.url));
            if (manifest.icons && Array.isArray(manifest.icons)) {
              for (const icon of manifest.icons) {
                const src = icon.src?.trim();
                if (!src) continue;
                const resolved = resolveUrl(pageUrl, src);
                if (resolved && !seen.has(resolved)) {
                  candidates.push({
                    url: resolved,
                    rel: 'manifest',
                    sizes: icon.sizes || '',
                    type: icon.type || '',
                    priority: 15,
                  });
                  seen.add(resolved);
                }
              }
            }
          }
        } catch {
          // manifest fetch failed, skip
        }
      }
    }

    // Rank: best first
    candidates.sort((a, b) => {
      if (a.priority !== b.priority) return a.priority - b.priority;
      return sizeScore(b.sizes) - sizeScore(a.sizes);
    });

    const recommended = candidates[0] || null;

    return jsonResponse<PreviewPayload>({
      input_url: inputUrl,
      page_url: pageUrl,
      recommended_icon_url: recommended?.url || null,
      icons: candidates.map((c) => ({
        icon_url: c.url,
        source_rel: c.rel,
        sizes: c.sizes || null,
        content_type: guessContentType(c.url, c.type),
        allowed_types: getAllowedTypes(c.url, c.type),
      })),
    });
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Unknown error';
    return jsonError(message, 500);
  }
}

// ─── Proxy: fetch icon binary (bypass CORS for browser) ────────────

async function handleProxy(urlObj: URL): Promise<Response> {
  const iconUrl = urlObj.searchParams.get('url');
  if (!iconUrl) {
    return jsonError('url parameter is required', 400);
  }

  try {
    const resp = await fetch(iconUrl, {
      headers: { 'User-Agent': 'Mozilla/5.0 (compatible; FaviconFisher/1.0)' },
      redirect: 'follow',
    });

    if (!resp.ok) {
      return jsonError(`fetch icon failed: status ${resp.status}`, 502);
    }

    const contentType = resp.headers.get('content-type') || 'application/octet-stream';
    const body = await resp.arrayBuffer();

    return new Response(body, {
      headers: {
        ...corsHeaders(),
        'Content-Type': contentType,
        'Cache-Control': 'public, max-age=86400',
      },
    });
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Unknown error';
    return jsonError(message, 502);
  }
}

// ─── Discovery: parse HTML for favicon candidates ──────────────────

function discoverCandidates(pageUrl: string, html: string): IconCandidate[] {
  const candidates: IconCandidate[] = [];
  const seen = new Set<string>();

  // Parse <link> tags with icon-related rel
  const linkRe = /<link\b[^>]*>/gi;
  let match: RegExpExecArray | null;
  while ((match = linkRe.exec(html)) !== null) {
    const tag = match[0];
    const rel = getAttr(tag, 'rel')?.toLowerCase().trim() || '';
    const href = getAttr(tag, 'href')?.trim() || '';
    const sizes = getAttr(tag, 'sizes')?.trim() || '';
    const type = getAttr(tag, 'type')?.trim() || '';

    if (href && isIconRel(rel)) {
      const resolved = resolveUrl(pageUrl, href);
      if (resolved && !seen.has(resolved)) {
        candidates.push({ url: resolved, rel, sizes, type, priority: relPriority(rel) });
        seen.add(resolved);
      }
    }
  }

  // Manifest icons are handled asynchronously in the preview handler
  // after the initial HTML candidates are found.

  // Fallback /favicon.ico
  try {
    const fallback = new URL('/favicon.ico', pageUrl).href;
    if (!seen.has(fallback)) {
      candidates.push({ url: fallback, rel: 'fallback', sizes: '', type: '', priority: 100 });
      seen.add(fallback);
    }
  } catch {
    // invalid URL, skip
  }

  return candidates;
}

// ─── Manifest link discovery ───────────────────────────────────────

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

// ─── HTML attribute helpers ────────────────────────────────────────

function getAttr(tag: string, name: string): string | null {
  // Match attr="value" or attr='value' (greedy within quotes)
  const re = new RegExp(`${name}\\s*=\\s*["']([^"']*)["']`, 'i');
  const m = tag.match(re);
  return m ? m[1] : null;
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
  if (rel === 'apple-touch-icon' || rel === 'apple-touch-icon-precomposed') return 20;
  if (rel === 'mask-icon') return 30;
  if (rel.includes('icon')) return 40;
  return 90;
}

function sizeScore(sizes: string): number {
  let max = 0;
  for (const part of sizes.toLowerCase().split(/\s+/)) {
    const [w, h] = part.split('x').map(Number);
    if (w && h) max = Math.max(max, w * h);
  }
  return max;
}

// ─── URL helpers ───────────────────────────────────────────────────

function normalizeUrl(raw: string): string {
  let trimmed = raw.trim();
  if (!trimmed.includes('://')) trimmed = 'https://' + trimmed;
  return trimmed;
}

function resolveUrl(base: string, href: string): string {
  try {
    return new URL(href, base).href;
  } catch {
    return '';
  }
}

// ─── Content type / allowed types ──────────────────────────────────

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
  const ct = guessContentType(url, declaredType);
  // Raster formats: can convert to png, jpg, webp, ico
  // SVG: only svg passthrough
  // ICO: only ico passthrough
  if (ct.includes('svg')) return ['svg'];
  if (ct.includes('icon') || ct.includes('ico')) return ['ico', 'png', 'jpg', 'webp'];
  // png, jpg, gif, webp — all raster
  return ['png', 'jpg', 'webp', 'ico'];
}

// ─── Response helpers ──────────────────────────────────────────────

function corsHeaders(): Record<string, string> {
  return {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type',
  };
}

function jsonResponse<T>(data: T, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json', ...corsHeaders() },
  });
}

function jsonError(message: string, status = 500): Response {
  return jsonResponse({ error: message }, status);
}