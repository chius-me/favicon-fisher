const form = document.getElementById('preview-form');
const statusEl = document.getElementById('status');
const resultsEl = document.getElementById('results');
const heroIconEl = document.getElementById('hero-icon');
const pageUrlEl = document.getElementById('page-url');
const selectedUrlEl = document.getElementById('selected-url');
const iconListEl = document.getElementById('icon-list');
const formatEl = document.getElementById('format');
const downloadBtn = document.getElementById('download-btn');

let previewState = null;
let selectedIcon = null;

form.addEventListener('submit', async (event) => {
  event.preventDefault();
  const formData = new FormData(form);
  const url = formData.get('url');
  setStatus('Loading preview...', false);
  resultsEl.classList.add('hidden');

  try {
    const response = await fetch('/api/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url }),
    });

    const payload = await response.json();
    if (!response.ok) {
      throw new Error(payload.error || 'Preview failed');
    }

    previewState = payload;
    selectedIcon = payload.icons.find((icon) => icon.icon_url === payload.recommended_icon_url) || payload.icons[0];
    renderPreview();
    setStatus(`Found ${payload.icons.length} icon candidate(s).`, false);
    resultsEl.classList.remove('hidden');
  } catch (error) {
    setStatus(error.message || 'Preview failed', true);
  }
});

downloadBtn.addEventListener('click', async () => {
  if (!selectedIcon) return;

  setStatus('Preparing download...', false);
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
  } catch (error) {
    setStatus(error.message || 'Download failed', true);
  }
});

function renderPreview() {
  if (!previewState || !selectedIcon) return;

  heroIconEl.src = selectedIcon.icon_url;
  heroIconEl.alt = selectedIcon.icon_url;
  pageUrlEl.textContent = previewState.page_url;
  selectedUrlEl.textContent = selectedIcon.icon_url;

  iconListEl.innerHTML = '';
  previewState.icons.forEach((icon) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `icon-item ${icon.icon_url === selectedIcon.icon_url ? 'active' : ''}`;
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
  selectedIcon.allowed_types.forEach((format) => {
    const option = document.createElement('option');
    option.value = format;
    option.textContent = format.toUpperCase();
    formatEl.appendChild(option);
  });
}

function setStatus(message, isError) {
  statusEl.textContent = message;
  statusEl.className = `status ${isError ? 'error' : ''}`;
}
