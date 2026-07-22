export async function readRequestText(request: Request, limit: number): Promise<string> {
  const buf = await request.arrayBuffer();
  if (buf.byteLength > limit) throw new Error('request body too large');
  return new TextDecoder().decode(buf);
}

export async function readResponseText(resp: Response, limit: number): Promise<string> {
  const bytes = await readResponseBytes(resp, limit);
  return new TextDecoder().decode(bytes);
}

export async function readResponseBytes(resp: Response, limit: number): Promise<ArrayBuffer> {
  if (!resp.body) return new ArrayBuffer(0);
  const reader = resp.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > limit) {
      try {
        await reader.cancel();
      } catch {
        /* ignore */
      }
      throw new Error('response too large');
    }
    chunks.push(value);
  }
  const out = new Uint8Array(total);
  let offset = 0;
  for (const c of chunks) {
    out.set(c, offset);
    offset += c.byteLength;
  }
  return out.buffer;
}
