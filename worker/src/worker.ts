/**
 * favicon-worker — Cloudflare Worker entrypoint.
 * Discovers favicon candidates and proxies icon downloads with SSRF guards,
 * signed tokens, body limits, and rate limiting.
 */

import { allowRate } from './security/rate-limit';
import { securityHeaders, withSecurityHeaders } from './http/headers';
import { jsonError } from './http/errors';
import { handlePreview } from './routes/preview';
import { handleProxy } from './routes/proxy';
import type { Env } from './types';

export default {
  async fetch(request: Request, env: Env, _ctx: ExecutionContext): Promise<Response> {
    const started = Date.now();
    const url = new URL(request.url);
    const requestId = request.headers.get('X-Request-ID') || crypto.randomUUID().slice(0, 16);

    if (request.method === 'OPTIONS') {
      return new Response(null, {
        status: 204,
        headers: { ...securityHeaders(request), 'X-Request-ID': requestId },
      });
    }

    if (url.pathname.startsWith('/api/')) {
      const ip = request.headers.get('cf-connecting-ip') || 'unknown';
      if (!allowRate(ip)) {
        return withRequestId(jsonError('rate limit exceeded', 429, request), requestId);
      }
    }

    let response: Response;
    try {
      if (url.pathname === '/api/preview' && request.method === 'POST') {
        response = withSecurityHeaders(request, await handlePreview(request, env));
      } else if (url.pathname === '/api/proxy' && request.method === 'GET') {
        response = withSecurityHeaders(request, await handleProxy(url, env));
      } else {
        const assetResp = await env.ASSETS.fetch(request);
        response = withSecurityHeaders(request, assetResp);
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'request failed';
      console.error(
        JSON.stringify({
          level: 'error',
          request_id: requestId,
          path: url.pathname,
          error: message,
        })
      );
      response = jsonError('request failed', 502, request);
    }

    response = withRequestId(response, requestId);

    if (url.pathname.startsWith('/api/') || response.status >= 400) {
      console.log(
        JSON.stringify({
          level: response.status >= 500 ? 'error' : response.status >= 400 ? 'warn' : 'info',
          request_id: requestId,
          method: request.method,
          path: url.pathname,
          status: response.status,
          duration_ms: Date.now() - started,
        })
      );
    }

    return response;
  },
} satisfies ExportedHandler<Env>;

function withRequestId(resp: Response, requestId: string): Response {
  const headers = new Headers(resp.headers);
  headers.set('X-Request-ID', requestId);
  return new Response(resp.body, {
    status: resp.status,
    statusText: resp.statusText,
    headers,
  });
}
