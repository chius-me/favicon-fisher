import { load } from 'cheerio';

export type IconCandidate = {
  url: string;
  rel: string;
  sizes?: string;
  type?: string;
  size: number | null;
  source: 'html' | 'manifest' | 'default';
};

export type FaviconResult = {
  siteUrl: string;
  title: string | null;
  bestIcon: IconCandidate | null;
  candidates: IconCandidate[];
};

export type TextFetcher = (url: string) => Promise<string | null>;

type ExtractOptions = {
  websiteUrl: string;
  html: string;
  fetchText?: TextFetcher;
};

type GetFaviconOptions = {
  fetchText?: TextFetcher;
};

const ICON_REL_TOKENS = new Set([
  'icon',
  'shortcut icon',
  'mask-icon',
  'apple-touch-icon',
  'apple-touch-icon-precomposed',
]);

export function normalizeWebsiteUrl(input: string): string {
  const value = input.trim();
  if (!value) {
    throw new Error('Please enter a website URL.');
  }

  const withProtocol = /^[a-z][a-z\d+.-]*:/i.test(value) ? value : `https://${value}`;
  const url = new URL(withProtocol);

  if (!['http:', 'https:'].includes(url.protocol)) {
    throw new Error('Only HTTP and HTTPS websites are supported.');
  }

  return url.toString();
}

export async function extractIconCandidates({
  websiteUrl,
  html,
  fetchText = fetchTextFromWeb,
}: ExtractOptions): Promise<IconCandidate[]> {
  const $ = load(html);
  const candidates: IconCandidate[] = [];
  const seen = new Set<string>();

  $('link[href]').each((_, element) => {
    const rel = ($(element).attr('rel') || '').trim().toLowerCase();
    const href = ($(element).attr('href') || '').trim();

    if (!href) {
      return;
    }

    if (isIconRel(rel)) {
      pushCandidate(candidates, seen, {
        url: resolveUrl(websiteUrl, href),
        rel: rel || 'icon',
        sizes: ($(element).attr('sizes') || '').trim() || undefined,
        type: ($(element).attr('type') || '').trim() || undefined,
        size: parseLargestSize($(element).attr('sizes')),
        source: 'html',
      });
      return;
    }

    if (rel === 'manifest') {
      pushManifestIcons(candidates, seen, websiteUrl, href, fetchText).catch(() => {
        return;
      });
    }
  });

  await Promise.all(
    $('link[href]')
      .map((_, element) => {
        const rel = ($(element).attr('rel') || '').trim().toLowerCase();
        const href = ($(element).attr('href') || '').trim();

        if (rel !== 'manifest' || !href) {
          return null;
        }

        return pushManifestIcons(candidates, seen, websiteUrl, href, fetchText);
      })
      .get()
      .filter(Boolean),
  );

  pushCandidate(candidates, seen, {
    url: resolveUrl(websiteUrl, '/favicon.ico'),
    rel: 'icon',
    size: 16,
    source: 'default',
  });

  return sortCandidates(candidates);
}

export function pickBestIcon(candidates: IconCandidate[]): IconCandidate | null {
  if (!candidates.length) {
    return null;
  }

  return [...candidates].sort(compareCandidates)[0] ?? null;
}

export async function getFaviconData(
  input: string,
  { fetchText = fetchTextFromWeb }: GetFaviconOptions = {},
): Promise<FaviconResult> {
  const siteUrl = normalizeWebsiteUrl(input);
  const html = await fetchText(siteUrl);

  if (!html) {
    throw new Error('Could not fetch the target website.');
  }

  const $ = load(html);
  const title = $('title').first().text().trim() || null;
  const candidates = await extractIconCandidates({
    websiteUrl: siteUrl,
    html,
    fetchText,
  });

  return {
    siteUrl,
    title,
    bestIcon: pickBestIcon(candidates),
    candidates,
  };
}

async function pushManifestIcons(
  candidates: IconCandidate[],
  seen: Set<string>,
  websiteUrl: string,
  manifestHref: string,
  fetchText: TextFetcher,
): Promise<void> {
  const manifestUrl = resolveUrl(websiteUrl, manifestHref);
  const manifestText = await fetchText(manifestUrl);

  if (!manifestText) {
    return;
  }

  let manifest: { icons?: Array<{ src?: string; sizes?: string; type?: string }> };

  try {
    manifest = JSON.parse(manifestText);
  } catch {
    return;
  }

  for (const icon of manifest.icons ?? []) {
    if (!icon.src) {
      continue;
    }

    pushCandidate(candidates, seen, {
      url: resolveUrl(manifestUrl, icon.src),
      rel: 'manifest',
      sizes: icon.sizes,
      type: icon.type,
      size: parseLargestSize(icon.sizes),
      source: 'manifest',
    });
  }
}

function pushCandidate(candidates: IconCandidate[], seen: Set<string>, candidate: IconCandidate): void {
  const key = `${candidate.url}|${candidate.rel}|${candidate.sizes || ''}`;
  if (seen.has(key)) {
    return;
  }

  seen.add(key);
  candidates.push(candidate);
}

function resolveUrl(baseUrl: string, target: string): string {
  return new URL(target, baseUrl).toString();
}

function isIconRel(rel: string): boolean {
  if (!rel) {
    return false;
  }

  const normalized = rel
    .split(/\s+/)
    .filter(Boolean)
    .join(' ');

  return ICON_REL_TOKENS.has(normalized) || normalized.includes('icon');
}

function parseLargestSize(sizes?: string | null): number | null {
  if (!sizes) {
    return null;
  }

  const matches = sizes.match(/(\d+)x(\d+)/gi);
  if (!matches?.length) {
    return null;
  }

  let largest = 0;
  for (const match of matches) {
    const [width, height] = match.toLowerCase().split('x').map(Number);
    if (Number.isFinite(width) && Number.isFinite(height)) {
      largest = Math.max(largest, Math.min(width, height));
    }
  }

  return largest || null;
}

function compareCandidates(left: IconCandidate, right: IconCandidate): number {
  const leftScore = scoreCandidate(left);
  const rightScore = scoreCandidate(right);
  return rightScore - leftScore;
}

function scoreCandidate(candidate: IconCandidate): number {
  const sizeScore = candidate.size ?? 0;
  const sourceScore =
    candidate.source === 'manifest' ? 3 : candidate.source === 'html' ? 2 : 1;
  const appleBonus = candidate.rel.includes('apple-touch-icon') ? 2 : 0;
  const svgBonus = candidate.url.endsWith('.svg') ? 1 : 0;

  return sizeScore * 100 + sourceScore * 10 + appleBonus + svgBonus;
}

function sortCandidates(candidates: IconCandidate[]): IconCandidate[] {
  return [...candidates].sort(compareCandidates);
}

async function fetchTextFromWeb(url: string): Promise<string | null> {
  const response = await fetch(url, {
    headers: {
      'user-agent':
        'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36',
      accept: 'text/html,application/xhtml+xml,application/xml,text/plain,application/json;q=0.9,*/*;q=0.8',
    },
    redirect: 'follow',
    cache: 'no-store',
  });

  if (!response.ok) {
    return null;
  }

  return response.text();
}
