export function securityHeaders(request?: Request): Record<string, string> {
  const h: Record<string, string> = {
    'X-Content-Type-Options': 'nosniff',
    'X-Frame-Options': 'DENY',
    'Referrer-Policy': 'no-referrer',
    'Content-Security-Policy':
      "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'",
    'Permissions-Policy': 'camera=(), microphone=(), geolocation=()',
  };
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

export function withSecurityHeaders(request: Request, resp: Response): Response {
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
