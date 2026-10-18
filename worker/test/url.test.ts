import { describe, expect, it } from 'vitest';
import { assertSafeUrl } from '../src/security/url';

describe('IPv6 destination validation', () => {
  it.each(['::', '::1', '0:0:0:0:0:0:0:1', 'fd00::1', 'fc00::1', 'fe80::1', 'febf::1', 'ff02::1', '::ffff:127.0.0.1', '::ffff:192.168.1.2', '::ffff:7f00:1'])('blocks %s', async host => {
    await expect(assertSafeUrl(`http://[${host}]/`, false)).rejects.toThrow('not allowed');
  });
  it.each(['2606:4700:4700::1111', '2001:4860:4860::8888', '::ffff:8.8.8.8'])('allows public %s', async host => {
    await expect(assertSafeUrl(`https://[${host}]/`, false)).resolves.toBeUndefined();
  });
  it('honors explicit local policy', async () => {
    await expect(assertSafeUrl('http://[::1]/', true)).resolves.toBeUndefined();
  });
});
