import { MIN_SIGNING_SECRET_LEN, PURPOSE_FETCH, TOKEN_TTL_SEC } from '../constants';
import type { Env } from '../types';

export async function signingKey(env: Env): Promise<CryptoKey> {
  if (!env.FVF_SIGNING_SECRET || env.FVF_SIGNING_SECRET.length < MIN_SIGNING_SECRET_LEN) {
    throw new Error('FVF_SIGNING_SECRET is not configured');
  }
  const raw = new TextEncoder().encode(env.FVF_SIGNING_SECRET);
  return crypto.subtle.importKey('raw', raw, { name: 'HMAC', hash: 'SHA-256' }, false, [
    'sign',
    'verify',
  ]);
}

export async function signUrl(key: CryptoKey, iconUrl: string, purpose = PURPOSE_FETCH): Promise<string> {
  const exp = Math.floor(Date.now() / 1000) + TOKEN_TTL_SEC;
  const payload = `${purpose}\n${iconUrl}\n${exp}`;
  const sig = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(payload));
  return `${exp}.${bufferToBase64Url(sig)}`;
}

export async function verifyUrl(
  key: CryptoKey,
  iconUrl: string,
  token: string,
  purpose = PURPOSE_FETCH
): Promise<boolean> {
  const parts = token.split('.');
  if (parts.length !== 2) return false;
  const exp = Number(parts[0]);
  if (!Number.isFinite(exp) || Math.floor(Date.now() / 1000) > exp) return false;
  const payload = `${purpose}\n${iconUrl}\n${exp}`;
  const expected = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(payload));
  const provided = base64UrlToBuffer(parts[1]);
  if (!provided || provided.byteLength !== expected.byteLength) return false;
  const a = new Uint8Array(expected);
  const b = new Uint8Array(provided);
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a[i] ^ b[i];
  return diff === 0;
}

function bufferToBase64Url(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let s = '';
  for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i]);
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function base64UrlToBuffer(s: string): ArrayBuffer | null {
  try {
    const pad = s.length % 4 === 0 ? '' : '='.repeat(4 - (s.length % 4));
    const b64 = s.replace(/-/g, '+').replace(/_/g, '/') + pad;
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out.buffer;
  } catch {
    return null;
  }
}
