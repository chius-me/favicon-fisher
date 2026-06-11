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

form.addEventListener('submit', async (event: Event) => {
  event.preventDefault();
  const url = (document.getElementById('url') as HTMLInputElement).value;
  setStatus('Loading preview...', false);
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
    const response = await fetch('/api/download', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        icon_url: selectedIcon.icon_url,
        format: formatEl.value,
      }),
    });

    if (!response.ok) {
      const payload = await response.json();
      throw new Error((payload as { error?: string }).error || 'Download failed');
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

function renderPreview(): void {
  if (!previewState || !selectedIcon) return;

  heroIconEl.src = selectedIcon.icon_url;
  heroIconEl.alt = selectedIcon.icon_url;
  pageUrlEl.textContent = previewState.page_url;
  selectedUrlEl.textContent = selectedIcon.icon_url;

  iconListEl.innerHTML = '';
  previewState.icons.forEach((icon: IconInfo) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `icon-item ${icon.icon_url === selectedIcon!.icon_url ? 'active' : ''}`;
    button.innerHTML = `
      <img src="${icon.icon_url}" alt="${icon.source_rel}">
      <span>${icon.source_rel}</span>
      <small>${icon.sizes || 'unknown size'}</small>
    `;
    button.addEventListener('click', () => {
      selectedIcon = icon;
      renderPreview();
    });
    iconListEl.appendChild(button);
  });

  formatEl.innerHTML = '';
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
