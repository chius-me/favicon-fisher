import { describe, expect, it } from 'vitest';
import { discoverCandidates, getAttr, getAllowedTypes } from '../src/discovery/html';
import { assertSafeUrl } from '../src/security/url';
import { signingKey, signUrl, verifyUrl } from '../src/security/signing';
import type { Env } from '../src/types';

describe('HTML discovery', () => {
  it('parses unquoted link attributes', () => {
    const html = `<link rel=icon href=/favicon.ico><link rel=apple-touch-icon href=/apple.png>`;
    const candidates = discoverCandidates('https://example.com/', html);
    const urls = candidates.map((c) => c.url);
    expect(urls).toContain('https://example.com/favicon.ico');
    expect(urls).toContain('https://example.com/apple.png');
  });

  it('caps HTML icon candidates', () => {
    let html = '<html><head>';
    for (let i = 0; i < 100; i++) html += `<link rel="icon" href="/i${i}.png">`;
    html += '</head></html>';
    const candidates = discoverCandidates('https://example.com/', html);
    const nonFallback = candidates.filter((c) => c.rel !== 'fallback');
    expect(nonFallback.length).toBeLessThanOrEqual(32);
  });

  it('getAttr supports quoted and unquoted', () => {
    expect(getAttr('<link rel="icon" href="/a">', 'rel')).toBe('icon');
    expect(getAttr('<link rel=icon href=/b>', 'href')).toBe('/b');
  });
});

describe('allowed types', () => {
  it('returns empty for unknown non-image', () => {
    expect(getAllowedTypes('https://example.com/x', 'text/html')).toEqual([]);
  });

  it('returns png/jpg for png URL', () => {
    expect(getAllowedTypes('https://example.com/a.png', '')).toContain('png');
  });
});

describe('SSRF', () => {
  it('blocks private IP literals', async () => {
    await expect(assertSafeUrl('http://127.0.0.1/', false)).rejects.toThrow(/private|reserved/i);
    await expect(assertSafeUrl('http://192.168.1.1/admin', false)).rejects.toThrow(/private|reserved/i);
  });

  it('allows public https', async () => {
    await expect(assertSafeUrl('https://example.com/', false)).resolves.toBeUndefined();
  });
});

describe('signing', () => {
  it('round-trips purpose-bound tokens', async () => {
    const env = { FVF_SIGNING_SECRET: 'test-signing-secret-at-least-32-chars!!' } as Env;
    const key = await signingKey(env);
    const url = 'https://cdn.example.com/favicon.png';
    const token = await signUrl(key, url);
    expect(await verifyUrl(key, url, token)).toBe(true);
    expect(await verifyUrl(key, url + 'x', token)).toBe(false);
  });

  it('fails closed without secret', async () => {
    await expect(signingKey({} as Env)).rejects.toThrow(/FVF_SIGNING_SECRET/);
    await expect(signingKey({ FVF_SIGNING_SECRET: 'short' } as Env)).rejects.toThrow(
      /FVF_SIGNING_SECRET/
    );
  });
});
