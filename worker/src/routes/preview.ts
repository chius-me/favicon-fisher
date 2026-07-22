import {
  MAX_HTML,
  MAX_JSON,
  MAX_MANIFEST,
  MAX_MANIFEST_ICONS,
  MAX_REDIRECT_HOPS,
  MAX_TOTAL_CANDIDATES,
} from '../constants';
import {
  discoverCandidates,
  enrichCandidates,
  findManifestHref,
  getAllowedTypes,
  guessContentType,
  relPriority,
} from '../discovery/html';
import { readRequestText, readResponseText } from '../http/body';
import { jsonError, jsonResponse, publicError, statusFor } from '../http/errors';
import { signUrl, signingKey } from '../security/signing';
import { assertSafeUrl, followRedirects, normalizeUrl, resolveUrl, safeFetch } from '../security/url';
import type { Env, Manifest, PreviewPayload } from '../types';

export async function handlePreview(request: Request, env: Env): Promise<Response> {
  try {
    const rawBody = await readRequestText(request, MAX_JSON);
    const { url: inputUrl } = JSON.parse(rawBody) as { url?: string };
    if (!inputUrl) {
      return jsonError('url is required', 400, request);
    }

    const allowPrivate = env.FVF_ALLOW_PRIVATE === '1';
    const normalized = normalizeUrl(inputUrl);
    await assertSafeUrl(normalized, allowPrivate);

    const resp = await safeFetch(normalized);
    const followed = await followRedirects(resp, normalized, allowPrivate, MAX_REDIRECT_HOPS);
    if (!followed.ok) {
      return jsonError('upstream request failed', 502, request);
    }

    const pageUrl = followed.url;
    const html = await readResponseText(followed, MAX_HTML);
    const candidates = discoverCandidates(pageUrl, html);

    const manifestHref = findManifestHref(html);
    if (manifestHref) {
      const manifestUrl = resolveUrl(pageUrl, manifestHref);
      if (manifestUrl) {
        try {
          await assertSafeUrl(manifestUrl, allowPrivate);
          const manifestResp = await safeFetch(manifestUrl);
          const mFollowed = await followRedirects(manifestResp, manifestUrl, allowPrivate, MAX_REDIRECT_HOPS);
          if (mFollowed.ok) {
            const manifestText = await readResponseText(mFollowed, MAX_MANIFEST);
            const manifest = JSON.parse(manifestText) as Manifest;
            const seen = new Set<string>(candidates.map((c) => c.url));
            const base = mFollowed.url;
            if (manifest.icons && Array.isArray(manifest.icons)) {
              let manifestCount = 0;
              for (const icon of manifest.icons) {
                if (manifestCount >= MAX_MANIFEST_ICONS) break;
                if (candidates.length >= MAX_TOTAL_CANDIDATES) break;
                const src = icon.src?.trim();
                if (!src) continue;
                const resolved = resolveUrl(base, src);
                if (resolved && !seen.has(resolved)) {
                  candidates.push({
                    url: resolved,
                    rel: 'manifest',
                    sizes: icon.sizes || '',
                    type: icon.type || '',
                    priority: relPriority('manifest'),
                  });
                  seen.add(resolved);
                  manifestCount++;
                }
              }
            }
          }
        } catch {
          // manifest fetch failed, skip
        }
      }
    }

    await enrichCandidates(candidates, allowPrivate);

    const secret = await signingKey(env);
    const recommended = candidates[0] || null;
    const icons = [];
    for (const c of candidates) {
      const allowed = getAllowedTypes(c.url, c.type);
      if (allowed.length === 0) continue;
      icons.push({
        icon_url: c.url,
        token: await signUrl(secret, c.url),
        source_rel: c.rel,
        sizes: c.sizes || null,
        content_type: guessContentType(c.url, c.type),
        allowed_types: allowed,
      });
    }

    return jsonResponse<PreviewPayload>(
      {
        input_url: inputUrl,
        page_url: pageUrl,
        recommended_icon_url: recommended?.url || null,
        icons,
      },
      200,
      request
    );
  } catch (err: unknown) {
    return jsonError(publicError(err), statusFor(err), request);
  }
}
