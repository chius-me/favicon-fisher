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

form.addEventListener('submit', async (event: Event) => {
  event.preventDefault();
  const url = (document.getElementById('url') as HTMLInputElement).value;
  setStatus('Loading preview...', false);
  resultsEl.classList.add('hidden');
  previewBtn.disabled = true;
  form.setAttribute('aria-busy', 'true');

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
    form.removeAttribute('aria-busy');
  }
});

downloadBtn.addEventListener('click', async () => {
  if (!selectedIcon) return;
  setStatus('Preparing download...', false);
  downloadBtn.disabled = true;

  try {
    const response = await fetch('/api/download', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        icon_url: selectedIcon.icon_url,
        format: formatEl.value,
        token: selectedIcon.token,
      }),
    });

    if (!response.ok) {
      const payload = (await response.json()) as { error?: string };
      throw new Error(payload.error || 'Download failed');
    }

    const blob = await response.blob();
    const disposition = response.headers.get('Content-Disposition') || '';
    const match = disposition.match(/filename="([^"]+)"/);
    const filename = match ? match[1] : `favicon.${formatEl.value}`;

    const objectURL = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = objectURL;
    anchor.download = filename;
    anchor.click();
    URL.revokeObjectURL(objectURL);
    setStatus(`Downloaded ${filename}.`, false);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Download failed';
    setStatus(message, true);
  } finally {
    downloadBtn.disabled = false;
  }
});

function proxyUrl(icon: IconInfo): string {
  return `/api/proxy?url=${encodeURIComponent(icon.icon_url)}&token=${encodeURIComponent(icon.token)}`;
}

function renderPreview(): void {
  if (!previewState || !selectedIcon) return;

  heroIconEl.src = proxyUrl(selectedIcon);
  heroIconEl.alt = 'Selected favicon preview';
  heroIconEl.onerror = () => {
    heroIconEl.removeAttribute('src');
    heroIconEl.alt = 'Preview unavailable';
  };
  pageUrlEl.textContent = previewState.page_url;
  selectedUrlEl.textContent = selectedIcon.icon_url;

  while (iconListEl.firstChild) {
    iconListEl.removeChild(iconListEl.firstChild);
  }

  previewState.icons.forEach((icon: IconInfo) => {
    const button = document.createElement('button');
    button.type = 'button';
    const isActive = icon.icon_url === selectedIcon!.icon_url;
    button.className = `icon-item ${isActive ? 'active' : ''}`;
    button.setAttribute('aria-pressed', isActive ? 'true' : 'false');

    const img = document.createElement('img');
    img.src = proxyUrl(icon);
    img.alt = icon.source_rel || 'icon';
    img.onerror = () => {
      img.alt = 'unavailable';
      img.style.opacity = '0.3';
    };

    const span = document.createElement('span');
    span.textContent = icon.source_rel || 'icon';

    const small = document.createElement('small');
    small.textContent = icon.sizes || 'unknown size';

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
  selectedIcon.allowed_types.forEach((format: string) => {
    const option = document.createElement('option');
    option.value = format;
    option.textContent = format.toUpperCase();
    formatEl.appendChild(option);
  });
}

function setStatus(message: string, isError: boolean): void {
  statusEl.textContent = message;
  statusEl.className = `status ${isError ? 'error' : ''}`;
}
