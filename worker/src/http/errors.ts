import { securityHeaders } from './headers';

export function publicError(err: unknown): string {
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

export function statusFor(err: unknown): number {
  const message = err instanceof Error ? err.message : '';
  const lower = message.toLowerCase();
  if (lower.includes('fvf_signing_secret')) return 503;
  if (lower.includes('required') || lower.includes('invalid url') || lower.includes('only http')) return 400;
  if (lower.includes('token')) return 403;
  if (lower.includes('rate')) return 429;
  if (lower.includes('private') || lower.includes('not allowed')) return 400;
  return 502;
}

export function jsonResponse<T>(data: T, status = 200, request?: Request): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json', ...securityHeaders(request) },
  });
}

export function jsonError(message: string, status = 500, request?: Request): Response {
  return jsonResponse({ error: message }, status, request);
}
