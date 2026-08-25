import { createExecutionContext, env, waitOnExecutionContext } from 'cloudflare:test';
import { describe, expect, it } from 'vitest';
import worker from '../src/worker';
import type { Env } from '../src/types';

describe('worker fetch handler', () => {
  it('returns 503-class misconfiguration without signing secret on preview', async () => {
    const bareEnv = { ASSETS: env.ASSETS } as Env;
    const request = new Request('https://example.com/api/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'test-req-1' },
      body: JSON.stringify({ url: 'https://example.com' }),
    });
    const ctx = createExecutionContext();
    const response = await worker.fetch(request, bareEnv, ctx);
    await waitOnExecutionContext(ctx);
    expect(response.status).toBe(503);
    const body = (await response.json()) as { error: string };
    expect(body.error).toMatch(/misconfigured/i);
    expect(response.headers.get('X-Request-ID')).toBe('test-req-1');
  });

  it('rejects private page URLs', async () => {
    const request = new Request('https://example.com/api/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: 'http://127.0.0.1/' }),
    });
    const ctx = createExecutionContext();
    const response = await worker.fetch(request, env as Env, ctx);
    await waitOnExecutionContext(ctx);
    expect(response.status).toBeGreaterThanOrEqual(400);
    expect(response.status).toBeLessThan(500);
    const body = (await response.json()) as { error: string };
    expect(body.error).toMatch(/not allowed|required|failed/i);
  });

  it('requires token on proxy', async () => {
    const request = new Request(
      'https://example.com/api/proxy?url=https%3A%2F%2Fexample.com%2Ffavicon.ico'
    );
    const ctx = createExecutionContext();
    const response = await worker.fetch(request, env as Env, ctx);
    await waitOnExecutionContext(ctx);
    expect(response.status).toBe(403);
  });

  it('serves home HTML with a favicon link through the worker', async () => {
    const request = new Request('https://example.com/');
    const ctx = createExecutionContext();
    const response = await worker.fetch(request, env as Env, ctx);
    await waitOnExecutionContext(ctx);
    expect(response.status).toBe(200);
    const html = await response.text();
    expect(html).toMatch(/rel=["']icon["']/);
    expect(html).toContain('/favicon.svg');
  });

  it('serves favicon.svg as SVG', async () => {
    const request = new Request('https://example.com/favicon.svg');
    const ctx = createExecutionContext();
    const response = await worker.fetch(request, env as Env, ctx);
    await waitOnExecutionContext(ctx);
    expect(response.status).toBe(200);
    const body = await response.text();
    expect(body.length).toBeGreaterThan(0);
    expect(body).toContain('<svg');
  });
});
