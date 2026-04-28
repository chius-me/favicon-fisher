import { describe, expect, it } from 'vitest';
import {
  extractIconCandidates,
  getFaviconData,
  normalizeWebsiteUrl,
  pickBestIcon,
} from './favicon';

describe('normalizeWebsiteUrl', () => {
  it('adds https when the user omits the protocol', () => {
    expect(normalizeWebsiteUrl('example.com')).toBe('https://example.com/');
  });

  it('preserves an existing protocol', () => {
    expect(normalizeWebsiteUrl('http://example.com/path')).toBe('http://example.com/path');
  });
});

describe('extractIconCandidates', () => {
  it('extracts icons from link tags and manifest icons', async () => {
    const html = `
      <html>
        <head>
          <link rel="icon" href="/favicon-32x32.png" sizes="32x32" />
          <link rel="apple-touch-icon" href="https://cdn.example.com/apple.png" sizes="180x180" />
          <link rel="manifest" href="/site.webmanifest" />
        </head>
      </html>
    `;

    const candidates = await extractIconCandidates({
      websiteUrl: 'https://example.com',
      html,
      fetchText: async (url) => {
        if (url === 'https://example.com/site.webmanifest') {
          return JSON.stringify({
            icons: [
              {
                src: '/android-chrome-512x512.png',
                sizes: '512x512',
                type: 'image/png',
              },
            ],
          });
        }

        return null;
      },
    });

    expect(candidates.map((candidate) => candidate.url)).toEqual([
      'https://example.com/android-chrome-512x512.png',
      'https://cdn.example.com/apple.png',
      'https://example.com/favicon-32x32.png',
      'https://example.com/favicon.ico',
    ]);
  });
});

describe('pickBestIcon', () => {
  it('prefers the largest square icon', () => {
    const best = pickBestIcon([
      {
        url: 'https://example.com/favicon.ico',
        rel: 'icon',
        size: 16,
        source: 'default',
      },
      {
        url: 'https://example.com/apple-touch-icon.png',
        rel: 'apple-touch-icon',
        size: 180,
        source: 'html',
      },
      {
        url: 'https://example.com/icon.svg',
        rel: 'icon',
        size: null,
        source: 'html',
      },
    ]);

    expect(best?.url).toBe('https://example.com/apple-touch-icon.png');
  });
});

describe('getFaviconData', () => {
  it('returns normalized site info and the best icon', async () => {
    const result = await getFaviconData('example.com', {
      fetchText: async (url) => {
        if (url === 'https://example.com/') {
          return `
            <html>
              <head>
                <title>Example Domain</title>
                <link rel="icon" href="/favicon-32x32.png" sizes="32x32" />
                <link rel="apple-touch-icon" href="/apple-touch-icon.png" sizes="180x180" />
              </head>
            </html>
          `;
        }

        return null;
      },
    });

    expect(result.siteUrl).toBe('https://example.com/');
    expect(result.bestIcon?.url).toBe('https://example.com/apple-touch-icon.png');
    expect(result.candidates).toHaveLength(3);
  });
});
