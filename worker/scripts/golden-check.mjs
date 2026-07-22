/**
 * Lightweight parity check against testdata/golden fixtures.
 * Mirrors Worker HTML discovery rules without spinning up a full Vitest stack.
 */
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const goldenDir = join(root, 'testdata', 'golden');

function getAttr(tag, name) {
  const quoted = new RegExp(`${name}\\s*=\\s*["']([^"']*)["']`, 'i');
  const qm = tag.match(quoted);
  if (qm) return qm[1];
  const unquoted = new RegExp(`${name}\\s*=\\s*([^\\s>]+)`, 'i');
  const um = tag.match(unquoted);
  return um ? um[1] : null;
}

function isIconRel(rel) {
  return rel.split(/\s+/).some((p) =>
    ['icon', 'shortcut', 'apple-touch-icon', 'apple-touch-icon-precomposed', 'mask-icon'].includes(p)
  );
}

function discover(pageUrl, html) {
  const candidates = [];
  const seen = new Set();
  const linkRe = /<link\b[^>]*>/gi;
  let match;
  while ((match = linkRe.exec(html)) !== null) {
    const tag = match[0];
    const rel = (getAttr(tag, 'rel') || '').toLowerCase().trim();
    const href = (getAttr(tag, 'href') || '').trim();
    if (!href || !isIconRel(rel)) continue;
    const url = new URL(href, pageUrl).href;
    if (!seen.has(url)) {
      candidates.push({ url, rel });
      seen.add(url);
    }
  }
  const fallback = new URL('/favicon.ico', pageUrl).href;
  if (!seen.has(fallback)) candidates.push({ url: fallback, rel: 'fallback' });
  return candidates;
}

// Contract fixture
const contract = JSON.parse(readFileSync(join(goldenDir, 'discovery.json'), 'utf8'));
const sampleHtml = readFileSync(join(goldenDir, contract.html_file), 'utf8');
const found = discover(contract.page_url, sampleHtml);
const urls = new Set(found.map((c) => c.url));
for (const want of contract.expected_urls) {
  if (!urls.has(want)) {
    console.error('missing candidate from discovery.json:', want);
    process.exit(1);
  }
}

// Unquoted attributes
const unquoted = readFileSync(join(goldenDir, 'unquoted.html'), 'utf8');
const uFound = discover('https://example.com/', unquoted);
const uUrls = new Set(uFound.map((c) => c.url));
for (const want of ['https://example.com/favicon.ico', 'https://example.com/apple-touch.png']) {
  if (!uUrls.has(want)) {
    console.error('missing unquoted candidate:', want);
    process.exit(1);
  }
}

console.log('golden contract check ok');
