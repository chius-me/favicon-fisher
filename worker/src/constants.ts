export const USER_AGENT = 'FaviconFisher/1.0 (+https://github.com/chius-me/favicon-fisher)';
export const MAX_HTML = 5 * 1024 * 1024;
export const MAX_ICON = 5 * 1024 * 1024;
export const MAX_MANIFEST = 1 * 1024 * 1024;
export const MAX_JSON = 64 * 1024;
export const TOKEN_TTL_SEC = 15 * 60;
export const RATE_LIMIT = 60;
export const RATE_WINDOW_MS = 60_000;
export const MAX_HTML_ICON_CANDIDATES = 32;
export const MAX_MANIFEST_ICONS = 32;
export const MAX_TOTAL_CANDIDATES = 48;
export const MAX_CONTENT_TYPE_PROBES = 12;
export const MAX_REDIRECT_HOPS = 5;
export const FETCH_TIMEOUT_MS = 10_000;
export const MIN_SIGNING_SECRET_LEN = 32;
/** Bound in HMAC payload; matches Go PurposeFetch. */
export const PURPOSE_FETCH = 'fetch';
