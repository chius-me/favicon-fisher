import { RATE_LIMIT, RATE_WINDOW_MS } from '../constants';

const rateHits = new Map<string, { count: number; start: number }>();

export function allowRate(key: string): boolean {
  const now = Date.now();
  const v = rateHits.get(key);
  if (!v || now - v.start >= RATE_WINDOW_MS) {
    rateHits.set(key, { count: 1, start: now });
    return true;
  }
  if (v.count >= RATE_LIMIT) return false;
  v.count++;
  return true;
}
