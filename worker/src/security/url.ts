import { FETCH_TIMEOUT_MS, MAX_REDIRECT_HOPS, USER_AGENT } from '../constants';

export async function assertSafeUrl(raw: string, allowPrivate: boolean): Promise<void> {
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
  if (host === 'metadata.google.internal' || host === 'metadata' || host.endsWith('.internal')) {
    throw new Error('requests to private or reserved addresses are not allowed');
  }

  if (isIPLiteral(host) && isBlockedIPLiteral(host)) {
    throw new Error('requests to private or reserved addresses are not allowed');
  }
}

function isIPLiteral(host: string): boolean {
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(host)) return true;
  if (host.includes(':')) return true;
  return false;
}

function isBlockedIPLiteral(host: string): boolean {
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
    if (a[0] >= 224) return true;
    return false;
  }
  const h = host.toLowerCase();
  if (h === '::1' || h === '0:0:0:0:0:0:0:1') return true;
  if (h.startsWith('fc') || h.startsWith('fd')) return true;
  if (h.startsWith('fe80')) return true;
  if (h.startsWith('::ffff:')) {
    const v4 = h.slice('::ffff:'.length);
    return isBlockedIPLiteral(v4);
  }
  return false;
}

export async function followRedirects(
  resp: Response,
  originalUrl: string,
  allowPrivate: boolean,
  maxHops: number = MAX_REDIRECT_HOPS
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
    current = await safeFetch(next);
  }
  throw new Error('too many redirects');
}

export async function safeFetch(url: string): Promise<Response> {
  return fetch(url, {
    headers: { 'User-Agent': USER_AGENT },
    redirect: 'manual',
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
}

export function normalizeUrl(raw: string): string {
  let trimmed = raw.trim();
  if (!trimmed.includes('://')) trimmed = 'https://' + trimmed;
  return trimmed;
}

export function resolveUrl(base: string, href: string): string {
  try {
    const u = new URL(href, base);
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return '';
    return u.href;
  } catch {
    return '';
  }
}
