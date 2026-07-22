import {
  FETCH_TIMEOUT_MS,
  MAX_CONTENT_TYPE_PROBES,
  MAX_HTML_ICON_CANDIDATES,
  MAX_REDIRECT_HOPS,
  MAX_TOTAL_CANDIDATES,
  USER_AGENT,
} from '../constants';
import type { IconCandidate } from '../types';
import { assertSafeUrl, followRedirects, resolveUrl } from '../security/url';

export function discoverCandidates(pageUrl: string, html: string): IconCandidate[] {
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

export function findManifestHref(html: string): string | null {
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

/** Exported for golden / parity tests. */
export function getAttr(tag: string, name: string): string | null {
  const quoted = new RegExp(`${name}\\s*=\\s*["']([^"']*)["']`, 'i');
  const qm = tag.match(quoted);
  if (qm) return qm[1];
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

export function relPriority(rel: string): number {
  if (rel === 'icon' || rel === 'shortcut icon') return 10;
  if (rel === 'manifest') return 15;
  if (rel === 'apple-touch-icon' || rel === 'apple-touch-icon-precomposed') return 20;
  if (rel === 'mask-icon') return 30;
  if (rel.includes('icon')) return 40;
  return 90;
}

export function sizeScore(sizes: string): number {
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

export function hasImageExtension(url: string): boolean {
  const path = url.split(/[?#]/)[0].toLowerCase();
  return ['.png', '.jpg', '.jpeg', '.svg', '.ico', '.gif', '.webp'].some((e) => path.endsWith(e));
}

export async function probeContentType(url: string, allowPrivate: boolean): Promise<string> {
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

export function guessContentType(url: string, declaredType: string): string {
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

export function getAllowedTypes(url: string, declaredType: string): string[] {
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
  return [];
}

export async function enrichCandidates(
  candidates: IconCandidate[],
  allowPrivate: boolean
): Promise<void> {
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
}
