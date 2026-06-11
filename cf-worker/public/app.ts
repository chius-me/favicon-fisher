interface PreviewPayload {
  input_url: string;
  page_url: string;
  recommended_icon_url: string | null;
  icons: IconInfo[];
}

interface IconInfo {
  icon_url: string;
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
// Cache: icon_url → ArrayBuffer (raw binary from proxy)
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

    const payload: PreviewPayload = await response.json();
    if (!response.ok) {
      throw new Error((payload as { error?: string }).error || 'Preview failed');
    }

    if (!payload.icons || payload.icons.length === 0) {
      throw new Error('No favicon candidates found');
    }

    previewState = payload;
    selectedIcon = payload.icons.find((i) => i.icon_url === payload.recommended_icon_url) || payload.icons[0];
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
    const buf = await fetchIconBuffer(selectedIcon.icon_url);
    const ct = selectedIcon.content_type || '';
    const isSvg = ct.includes('svg') || selectedIcon.icon_url.endsWith('.svg');
    const isIco = ct.includes('icon') || selectedIcon.icon_url.endsWith('.ico');

    let blob: Blob | null;

    if (fmt === 'svg' && isSvg) {
      // SVG passthrough
      blob = new Blob([buf], { type: 'image/svg+xml' });
    } else if (fmt === 'ico' && isIco) {
      // ICO passthrough
      blob = new Blob([buf], { type: 'image/x-icon' });
    } else {
      // Raster conversion via Canvas
      blob = await convertViaCanvas(selectedIcon.icon_url, fmt, buf, ct);
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

// ─── Rendering ─────────────────────────────────────────────────────

function renderPreview(): void {
  if (!previewState || !selectedIcon) return;

  // Use proxy for preview to avoid CORS issues
  heroIconEl.src = proxyUrl(selectedIcon.icon_url);
  pageUrlEl.textContent = previewState.page_url;
  selectedUrlEl.textContent = selectedIcon.icon_url;

  iconListEl.innerHTML = '';
  previewState.icons.forEach((icon: IconInfo) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `icon-item ${icon.icon_url === selectedIcon!.icon_url ? 'active' : ''}`;
    button.innerHTML = `
      <img src="${proxyUrl(icon.icon_url)}" alt="${icon.source_rel}">
      <span>${icon.source_rel}</span>
      <small>${icon.sizes || guessSizeFromUrl(icon.icon_url)}</small>
    `;
    button.addEventListener('click', () => {
      selectedIcon = icon;
      renderPreview();
    });
    iconListEl.appendChild(button);
  });

  formatEl.innerHTML = '';
  selectedIcon.allowed_types.forEach((fmt: string) => {
    const option = document.createElement('option');
    option.value = fmt;
    option.textContent = fmt.toUpperCase();
    formatEl.appendChild(option);
  });
}

// ─── Icon fetching ─────────────────────────────────────────────────

async function fetchIconBuffer(iconUrl: string): Promise<ArrayBuffer> {
  if (iconCache.has(iconUrl)) return iconCache.get(iconUrl)!;
  const resp = await fetch(proxyUrl(iconUrl));
  if (!resp.ok) throw new Error(`Failed to fetch icon: ${resp.status}`);
  const buf = await resp.arrayBuffer();
  iconCache.set(iconUrl, buf);
  return buf;
}

function proxyUrl(iconUrl: string): string {
  return `/api/proxy?url=${encodeURIComponent(iconUrl)}`;
}

// ─── Canvas-based format conversion ────────────────────────────────

async function convertViaCanvas(iconUrl: string, targetFmt: string, buffer: ArrayBuffer, contentType: string): Promise<Blob | null> {
  const img = new Image();
  img.crossOrigin = 'anonymous';
  img.src = proxyUrl(iconUrl);

  await new Promise<void>((resolve, reject) => {
    img.onload = () => resolve();
    img.onerror = () => reject(new Error('Failed to load icon image'));
  });

  const size = Math.max(img.naturalWidth, img.naturalHeight, 16);
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d')!;

  // White background for formats without transparency
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
      // ICO: generate PNG and wrap in ICO format (single 32-bit entry)
      const pngBlob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'));
      const pngBuf = new Uint8Array(await pngBlob!.arrayBuffer());
      const ico = buildIco(pngBuf, size);
      return new Blob([ico], { type: 'image/x-icon' });
    }
    default:
      return new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'));
  }
}

// Minimal ICO wrapper: one PNG entry
function buildIco(pngData: Uint8Array, size: number): ArrayBuffer {
  const headerSize = 6;
  const dirSize = 16;
  const dataOffset = headerSize + dirSize;
  const buf = new ArrayBuffer(dataOffset + pngData.length);
  const view = new DataView(buf);

  // ICO header
  view.setUint16(0, 0, true);    // reserved
  view.setUint16(2, 1, true);    // type: 1 = ICO
  view.setUint16(4, 1, true);    // count: 1 image

  // Directory entry
  view.setUint8(6, size >= 256 ? 0 : size);  // width (0 = 256+)
  view.setUint8(7, size >= 256 ? 0 : size);  // height
  view.setUint8(8, 0);           // color palette
  view.setUint8(9, 0);           // reserved
  view.setUint16(10, 1, true);   // color planes
  view.setUint16(12, 32, true);  // bits per pixel
  view.setUint32(14, pngData.length, true);  // image size
  view.setUint32(18, dataOffset, true);       // image offset

  // PNG data
  new Uint8Array(buf).set(pngData, dataOffset);

  return buf;
}

// ─── Helpers ───────────────────────────────────────────────────────

function guessSizeFromUrl(url: string): string {
  // Try to extract size from URL like "favicon-32x32.png"
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