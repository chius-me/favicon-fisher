import { MAX_ICON, MAX_REDIRECT_HOPS } from '../constants';
import { readResponseBytes } from '../http/body';
import { jsonError, publicError, statusFor } from '../http/errors';
import { signingKey, verifyUrl } from '../security/signing';
import { assertSafeUrl, followRedirects, safeFetch } from '../security/url';
import type { Env } from '../types';

export async function handleProxy(urlObj: URL, env: Env): Promise<Response> {
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

    const resp = await safeFetch(iconUrl);
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
