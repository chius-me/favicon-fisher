interface PreviewPayload {
  input_url: string;
  page_url: string;
  recommended_icon_url: string | null;
  icons: IconInfo[];
}

interface IconInfo {
  icon_url: string;
  token: string;
  source_rel: string;
  sizes: string | null;
  content_type: string;
  allowed_types: string[];
}

const form = document.getElementById('preview-form') as HTMLFormElement;
const statusEl = document.getElementById('status') as HTMLParagraphElement;
const resultsEl = document.getElementById('results') as HTMLElement;
const heroIconEl = document.getElementById('hero-icon') as HTMLImageElement;
const pageUrlEl = document.getElementById('page-url') as HTMLSpanElement;
const selectedUrlEl = document.getElementById('selected-url') as HTMLSpanElement;
const iconListEl = document.getElementById('icon-list') as HTMLDivElement;
const formatEl = document.getElementById('format') as HTMLSelectElement;
const downloadBtn = document.getElementById('download-btn') as HTMLButtonElement;
const previewBtn = document.getElementById('preview-btn') as HTMLButtonElement;

let previewState: PreviewPayload | null = null;
let selectedIcon: IconInfo | null = null;
const iconCache = new Map<string, ArrayBuffer>();

form.addEventListener('submit', async (event: Event) => {
  event.preventDefault();
  const url = (document.getElementById('url') as HTMLInputElement).value;
  setStatus('Discovering icons...', false);
  resultsEl.classList.add('hidden');
  previewBtn.disabled = true;

  try {
    const response = await fetch('/api/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url }),
    });

    const payload = (await response.json()) as PreviewPayload & { error?: string };
    if (!response.ok) {
      throw new Error(payload.error || 'Preview failed');
    }

    if (!payload.icons || payload.icons.length === 0) {
      throw new Error('No favicon candidates found');
    }

    previewState = payload;
    selectedIcon =
      payload.icons.find((i) => i.icon_url === payload.recommended_icon_url) || payload.icons[0];
    renderPreview();
    setStatus(`Found ${payload.icons.length} icon candidate(s).`, false);
    resultsEl.classList.remove('hidden');
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Preview failed';
    setStatus(message, true);
  } finally {
    previewBtn.disabled = false;
  }
});

downloadBtn.addEventListener('click', async () => {
  if (!selectedIcon) return;
  setStatus('Preparing download...', false);
  downloadBtn.disabled = true;

  try {
    const fmt = formatEl.value;
    const buf = await fetchIconBuffer(selectedIcon);
    const ct = selectedIcon.content_type || '';
    const isSvg = ct.includes('svg') || selectedIcon.icon_url.endsWith('.svg');
    const isIco = ct.includes('icon') || selectedIcon.icon_url.endsWith('.ico');

    let blob: Blob | null;

    if (fmt === 'svg' && isSvg) {
      blob = new Blob([buf], { type: 'image/svg+xml' });
    } else if (fmt === 'ico' && isIco) {
      blob = new Blob([buf], { type: 'image/x-icon' });
    } else {
      blob = await convertViaCanvas(selectedIcon, fmt);
    }

    if (!blob) throw new Error('Conversion failed');

    const filename = buildFilename(selectedIcon.icon_url, fmt);
    triggerDownload(blob, filename);
    setStatus(`Downloaded ${filename}`, false);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Download failed';
    setStatus(message, true);
  } finally {
    downloadBtn.disabled = false;
  }
});

function renderPreview(): void {
  if (!previewState || !selectedIcon) return;

  heroIconEl.src = proxyUrl(selectedIcon);
  heroIconEl.alt = 'Selected favicon preview';
  pageUrlEl.textContent = previewState.page_url;
  selectedUrlEl.textContent = selectedIcon.icon_url;

  while (iconListEl.firstChild) {
    iconListEl.removeChild(iconListEl.firstChild);
  }

  previewState.icons.forEach((icon: IconInfo) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `icon-item ${icon.icon_url === selectedIcon!.icon_url ? 'active' : ''}`;

    const img = document.createElement('img');
    img.src = proxyUrl(icon);
    img.alt = icon.source_rel || 'icon';

    const span = document.createElement('span');
    span.textContent = icon.source_rel || 'icon';

    const small = document.createElement('small');
    small.textContent = icon.sizes || guessSizeFromUrl(icon.icon_url) || 'unknown size';

    button.appendChild(img);
    button.appendChild(span);
    button.appendChild(small);
    button.addEventListener('click', () => {
      selectedIcon = icon;
      renderPreview();
    });
    iconListEl.appendChild(button);
  });

  while (formatEl.firstChild) {
    formatEl.removeChild(formatEl.firstChild);
  }
  selectedIcon.allowed_types.forEach((fmt: string) => {
    const option = document.createElement('option');
    option.value = fmt;
    option.textContent = fmt.toUpperCase();
    formatEl.appendChild(option);
  });
}

async function fetchIconBuffer(icon: IconInfo): Promise<ArrayBuffer> {
  if (iconCache.has(icon.icon_url)) return iconCache.get(icon.icon_url)!;
  const resp = await fetch(proxyUrl(icon));
  if (!resp.ok) throw new Error(`Failed to fetch icon: ${resp.status}`);
  const buf = await resp.arrayBuffer();
  iconCache.set(icon.icon_url, buf);
  return buf;
}

function proxyUrl(icon: IconInfo): string {
  return `/api/proxy?url=${encodeURIComponent(icon.icon_url)}&token=${encodeURIComponent(icon.token)}`;
}

async function convertViaCanvas(icon: IconInfo, targetFmt: string): Promise<Blob | null> {
  const img = new Image();
  img.crossOrigin = 'anonymous';
  img.src = proxyUrl(icon);

  await new Promise<void>((resolve, reject) => {
    img.onload = () => resolve();
    img.onerror = () => reject(new Error('Failed to load icon image'));
  });

  const size = Math.max(img.naturalWidth, img.naturalHeight, 16);
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d')!;

  if (targetFmt === 'jpg' || targetFmt === 'jpeg') {
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, size, size);
  }

  ctx.drawImage(img, 0, 0, size, size);

  switch (targetFmt) {
    case 'png':
      return new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'));
    case 'jpg':
    case 'jpeg':
      return new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.92));
    case 'webp':
      return new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/webp', 0.92));
    case 'ico': {
      const pngBlob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'));
      const pngBuf = new Uint8Array(await pngBlob!.arrayBuffer());
      const ico = buildIco(pngBuf, size);
      return new Blob([ico], { type: 'image/x-icon' });
    }
    default:
      return new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'));
  }
}

function buildIco(pngData: Uint8Array, size: number): ArrayBuffer {
  const headerSize = 6;
  const dirSize = 16;
  const dataOffset = headerSize + dirSize;
  const buf = new ArrayBuffer(dataOffset + pngData.length);
  const view = new DataView(buf);

  view.setUint16(0, 0, true);
  view.setUint16(2, 1, true);
  view.setUint16(4, 1, true);

  view.setUint8(6, size >= 256 ? 0 : size);
  view.setUint8(7, size >= 256 ? 0 : size);
  view.setUint8(8, 0);
  view.setUint8(9, 0);
  view.setUint16(10, 1, true);
  view.setUint16(12, 32, true);
  view.setUint32(14, pngData.length, true);
  view.setUint32(18, dataOffset, true);

  new Uint8Array(buf).set(pngData, dataOffset);
  return buf;
}

function guessSizeFromUrl(url: string): string {
  const m = url.match(/[-_](\d+)x(\d+)/i);
  return m ? `${m[1]}x${m[2]}` : '';
}

function buildFilename(iconUrl: string, fmt: string): string {
  try {
    const path = new URL(iconUrl).pathname;
    const base = path.split('/').pop()!.replace(/\.[^.]+$/, '') || 'favicon';
    return `${base}.${fmt}`;
  } catch {
    return `favicon.${fmt}`;
  }
}

function triggerDownload(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

function setStatus(message: string, isError: boolean): void {
  statusEl.textContent = message;
  statusEl.className = `status ${isError ? 'error' : ''}`;
}
