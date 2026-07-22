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
