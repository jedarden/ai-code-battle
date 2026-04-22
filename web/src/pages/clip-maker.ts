// Clip maker: export replay segments as MP4 (WebM) or animated GIF
// with 5 social media format presets.

import { ReplayViewer } from '../replay-viewer';
import type { Replay } from '../types';

// ─── Social format presets ───────────────────────────────────────────────────

interface SocialPreset {
  name: string;
  width: number;
  height: number;
  ratio: string;
  icon: string;
}

const SOCIAL_PRESETS: SocialPreset[] = [
  { name: 'Twitter / X',       width: 1280, height: 720,  ratio: '16:9', icon: '𝕏' },
  { name: 'Instagram Square',  width: 1080, height: 1080, ratio: '1:1',  icon: '▣' },
  { name: 'Instagram Story',   width: 1080, height: 1920, ratio: '9:16', icon: '◱' },
  { name: 'TikTok / Reels',    width: 1080, height: 1920, ratio: '9:16', icon: '▶' },
  { name: 'YouTube Shorts',    width: 1080, height: 1920, ratio: '9:16', icon: '▷' },
];

// Preview scale: limit longest side to 360px
function previewDims(preset: SocialPreset): { w: number; h: number } {
  const scale = 360 / Math.max(preset.width, preset.height);
  return { w: Math.round(preset.width * scale), h: Math.round(preset.height * scale) };
}

// ─── Page render ─────────────────────────────────────────────────────────────

export function renderClipMakerPage(_params: Record<string, string>): void {
  const app = document.getElementById('app');
  if (!app) return;
  app.innerHTML = buildHTML();
  requestAnimationFrame(() => initClipMaker());
}

function buildHTML(): string {
  const presetOptions = SOCIAL_PRESETS.map((p, i) =>
    `<option value="${i}">${p.icon} ${p.name} (${p.ratio})</option>`,
  ).join('');

  return `
    <div class="clip-page">
      <h1 class="page-title">Clip Maker</h1>
      <p class="clip-intro">Export replay highlights as MP4 or animated GIF, sized for social media.</p>

      <div class="clip-layout">
        <!-- Left: load + settings -->
        <div class="clip-settings-col">
          <div class="clip-panel">
            <div class="panel-header"><span>Load Replay</span></div>
            <div class="load-controls">
              <label class="btn secondary small" for="clip-file-input">Choose File</label>
              <input type="file" id="clip-file-input" accept=".json" style="display:none">
              <div class="url-row">
                <input type="text" id="clip-url-input" placeholder="Or paste replay URL…" class="url-input">
                <button id="clip-load-url-btn" class="btn primary small">Load</button>
              </div>
            </div>
            <div id="clip-load-status" class="clip-status hidden"></div>
          </div>

          <div class="clip-panel" id="clip-settings-panel" style="display:none">
            <div class="panel-header"><span>Format Preset</span></div>
            <select id="clip-preset-select" class="clip-select">
              ${presetOptions}
            </select>
            <div class="preset-dims" id="preset-dims-label"></div>
          </div>

          <div class="clip-panel" id="clip-range-panel" style="display:none">
            <div class="panel-header"><span>Turn Range</span></div>
            <div class="range-grid">
              <label>Start Turn</label>
              <div class="range-row">
                <input type="range" id="clip-start-slider" min="0" max="0" value="0" class="range-slider">
                <span id="clip-start-val">0</span>
              </div>
              <label>End Turn</label>
              <div class="range-row">
                <input type="range" id="clip-end-slider" min="0" max="0" value="0" class="range-slider">
                <span id="clip-end-val">0</span>
              </div>
              <label>FPS</label>
              <select id="clip-fps-select" class="clip-select-sm">
                <option value="10">10 fps</option>
                <option value="15" selected>15 fps</option>
                <option value="24">24 fps</option>
                <option value="30">30 fps</option>
              </select>
            </div>
          </div>

          <div class="clip-panel" id="clip-export-panel" style="display:none">
            <div class="panel-header"><span>Export</span></div>
            <div class="export-buttons">
              <button id="clip-export-mp4" class="btn primary">Export MP4 / WebM</button>
              <button id="clip-export-gif" class="btn secondary">Export GIF</button>
            </div>
            <div id="clip-export-progress" class="clip-progress hidden">
              <div class="progress-bar"><div id="clip-progress-fill" class="progress-fill" style="width:0%"></div></div>
              <span id="clip-progress-label">0%</span>
            </div>
            <div id="clip-share-panel" class="clip-share-panel hidden">
              <div class="panel-header"><span>Share</span></div>
              <div id="clip-share-text" class="share-preview-text"></div>
              <div class="share-buttons">
                <button id="share-twitter-btn" class="btn share-btn share-twitter">𝕏 Post</button>
                <button id="share-reddit-btn" class="btn share-btn share-reddit">Reddit</button>
                <button id="share-discord-btn" class="btn share-btn share-discord">Discord</button>
                <button id="share-copy-btn" class="btn share-btn share-copy">Copy Link</button>
              </div>
              <div id="clip-share-native" class="hidden" style="margin-top:8px">
                <button id="share-native-btn" class="btn primary" style="width:100%">Share via System…</button>
              </div>
              <div id="share-toast" class="share-toast hidden">Copied!</div>
            </div>
          </div>
        </div>

        <!-- Right: preview -->
        <div class="clip-preview-col">
          <div class="clip-panel" id="clip-preview-panel" style="display:none">
            <div class="panel-header">
              <span>Preview</span>
              <span id="clip-preview-info" class="preview-info"></span>
            </div>
            <div id="clip-preview-frame" class="preview-frame"></div>
            <div class="preview-nav">
              <button id="clip-prev-btn" class="btn small">Prev</button>
              <span id="clip-frame-label" class="frame-label">Turn 0</span>
              <button id="clip-next-btn" class="btn small">Next</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    ${CLIP_STYLES}
  `;
}

// ─── Initialisation ───────────────────────────────────────────────────────────

function initClipMaker(): void {
  let replay: Replay | null = null;
  let previewViewer: ReplayViewer | null = null;
  let previewCanvas: HTMLCanvasElement | null = null;

  const loadStatus = document.getElementById('clip-load-status')!;
  const settingsPanel = document.getElementById('clip-settings-panel')!;
  const rangePanel = document.getElementById('clip-range-panel')!;
  const exportPanel = document.getElementById('clip-export-panel')!;
  const previewPanel = document.getElementById('clip-preview-panel')!;

  const startSlider = document.getElementById('clip-start-slider') as HTMLInputElement;
  const endSlider   = document.getElementById('clip-end-slider')   as HTMLInputElement;
  const startVal    = document.getElementById('clip-start-val')!;
  const endVal      = document.getElementById('clip-end-val')!;
  const fpsSelect   = document.getElementById('clip-fps-select')   as HTMLSelectElement;
  const presetSelect = document.getElementById('clip-preset-select') as HTMLSelectElement;
  const dimsLabel   = document.getElementById('preset-dims-label')!;
  const previewInfo = document.getElementById('clip-preview-info')!;
  const frameLabel  = document.getElementById('clip-frame-label')!;
  const previewFrame = document.getElementById('clip-preview-frame')!;

  function updateDimsLabel(): void {
    const p = SOCIAL_PRESETS[Number(presetSelect.value)];
    dimsLabel.textContent = `${p.width} × ${p.height} px`;
  }
  updateDimsLabel();
  presetSelect.addEventListener('change', () => { updateDimsLabel(); rebuildPreview(); });

  function showError(msg: string): void {
    loadStatus.textContent = msg;
    loadStatus.className = 'clip-status error';
  }

  function loadReplayData(data: Replay): void {
    replay = data;
    const total = data.turns.length - 1;

    startSlider.max = String(total);
    startSlider.value = '0';
    endSlider.max = String(total);
    endSlider.value = String(total);
    startVal.textContent = '0';
    endVal.textContent = String(total);

    settingsPanel.style.display = '';
    rangePanel.style.display = '';
    exportPanel.style.display = '';
    previewPanel.style.display = '';

    loadStatus.textContent = `Loaded: ${data.match_id} (${total + 1} turns)`;
    loadStatus.className = 'clip-status ok';

    rebuildPreview();
  }

  function rebuildPreview(): void {
    if (!replay) return;
    const preset = SOCIAL_PRESETS[Number(presetSelect.value)];
    const dims = previewDims(preset);
    previewInfo.textContent = `${preset.width}×${preset.height}`;

    // Build or recreate preview canvas
    previewFrame.innerHTML = '';
    previewCanvas = document.createElement('canvas');
    previewFrame.appendChild(previewCanvas);

    // Render the game into a temp canvas, then composite into preview
    const tempCanvas = document.createElement('canvas');
    previewViewer = new ReplayViewer(tempCanvas, { cellSize: 8, showGrid: false });
    previewViewer.loadReplay(replay);

    drawCompositeFrame(previewCanvas, tempCanvas, preset, dims, Number(startSlider.value));
    frameLabel.textContent = `Turn ${startSlider.value}`;
  }

  startSlider.addEventListener('input', () => {
    startVal.textContent = startSlider.value;
    if (Number(startSlider.value) > Number(endSlider.value)) {
      endSlider.value = startSlider.value;
      endVal.textContent = endSlider.value;
    }
    updatePreviewTurn(Number(startSlider.value));
  });

  endSlider.addEventListener('input', () => {
    endVal.textContent = endSlider.value;
    if (Number(endSlider.value) < Number(startSlider.value)) {
      startSlider.value = endSlider.value;
      startVal.textContent = startSlider.value;
    }
  });

  document.getElementById('clip-prev-btn')!.addEventListener('click', () => {
    const cur = Number(startSlider.value);
    const prev = Math.max(0, cur - 1);
    startSlider.value = String(prev);
    startVal.textContent = String(prev);
    updatePreviewTurn(prev);
  });

  document.getElementById('clip-next-btn')!.addEventListener('click', () => {
    const cur = Number(startSlider.value);
    const next = Math.min(Number(startSlider.max), cur + 1);
    startSlider.value = String(next);
    startVal.textContent = String(next);
    updatePreviewTurn(next);
  });

  function updatePreviewTurn(turn: number): void {
    if (!replay || !previewCanvas || !previewViewer) return;
    const preset = SOCIAL_PRESETS[Number(presetSelect.value)];
    const dims = previewDims(preset);
    const tempCanvas = document.createElement('canvas');
    const tv = new ReplayViewer(tempCanvas, { cellSize: 8, showGrid: false });
    tv.loadReplay(replay);
    tv.setTurn(turn);
    drawCompositeFrame(previewCanvas, tempCanvas, preset, dims, turn);
    frameLabel.textContent = `Turn ${turn}`;
  }

  // ── File load ──────────────────────────────────────────────────────────────
  document.getElementById('clip-file-input')!.addEventListener('change', async (e) => {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      loadReplayData(JSON.parse(text) as Replay);
    } catch (err) {
      showError('Failed to parse replay: ' + err);
    }
  });

  document.getElementById('clip-load-url-btn')!.addEventListener('click', async () => {
    const url = (document.getElementById('clip-url-input') as HTMLInputElement).value.trim();
    if (!url) return;
    loadStatus.textContent = 'Loading…';
    loadStatus.className = 'clip-status';
    try {
      const resp = await fetch(url);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      loadReplayData((await resp.json()) as Replay);
    } catch (err) {
      showError('Failed to load URL: ' + err);
    }
  });

  let lastExportExt = '';

  // ── MP4 export ────────────────────────────────────────────────────────────
  document.getElementById('clip-export-mp4')!.addEventListener('click', async () => {
    if (!replay) return;
    await exportVideo(replay, 'mp4');
  });

  // ── GIF export ────────────────────────────────────────────────────────────
  document.getElementById('clip-export-gif')!.addEventListener('click', async () => {
    if (!replay) return;
    
    await exportGIF(replay);
  });

  async function exportVideo(r: Replay, _fmt: string): Promise<void> {
    if (!('MediaRecorder' in window)) {
      alert('MediaRecorder API not supported in this browser. Please use Chrome or Firefox.');
      return;
    }

    const preset = SOCIAL_PRESETS[Number(presetSelect.value)];
    const fps = Number(fpsSelect.value);
    const startTurn = Number(startSlider.value);
    const endTurn = Number(endSlider.value);
    const totalFrames = endTurn - startTurn + 1;

    // Determine preview scale for video (cap at 720p equivalent)
    const scale = Math.min(1, 720 / Math.max(preset.width, preset.height));
    const vw = Math.round(preset.width * scale);
    const vh = Math.round(preset.height * scale);

    const exportCanvas = document.createElement('canvas');
    exportCanvas.width = vw;
    exportCanvas.height = vh;

    const stream = exportCanvas.captureStream(fps);
    const mimeType = MediaRecorder.isTypeSupported('video/webm;codecs=vp9')
      ? 'video/webm;codecs=vp9'
      : 'video/webm';
    const recorder = new MediaRecorder(stream, { mimeType });
    const chunks: Blob[] = [];
    recorder.ondataavailable = (e) => { if (e.data.size > 0) chunks.push(e.data); };

    const tempCanvas = document.createElement('canvas');
    const tv = new ReplayViewer(tempCanvas, { cellSize: 6, showGrid: false });
    tv.loadReplay(r);

    showProgress(0);
    recorder.start();

    const msPerFrame = 1000 / fps;

    for (let t = startTurn; t <= endTurn; t++) {
      tv.setTurn(t);
      drawCompositeFrame(exportCanvas, tempCanvas, preset, { w: vw, h: vh }, t);
      await sleep(msPerFrame);
      updateProgress(((t - startTurn) / totalFrames) * 100);
    }

    recorder.stop();

    await new Promise<void>(res => { recorder.onstop = () => res(); });

    hideProgress();
    const blob = new Blob(chunks, { type: mimeType });
    const filename = `acb-clip-${r.match_id}-${preset.name.replace(/\s+/g, '_')}.webm`;
    // lastExportBlob = blob;
    lastExportExt = 'webm';
    downloadBlob(blob, filename);
    showSharePanel(r, startTurn, endTurn, blob);
  }

  async function exportGIF(r: Replay): Promise<void> {
    const preset = SOCIAL_PRESETS[Number(presetSelect.value)];
    const fps = Number(fpsSelect.value);
    const startTurn = Number(startSlider.value);
    const endTurn = Number(endSlider.value);
    const totalFrames = endTurn - startTurn + 1;

    // Use small scale for GIF to keep file size manageable (max 480px)
    const scale = Math.min(1, 480 / Math.max(preset.width, preset.height));
    const gw = Math.round(preset.width * scale);
    const gh = Math.round(preset.height * scale);

    const frameCanvas = document.createElement('canvas');
    frameCanvas.width = gw;
    frameCanvas.height = gh;
    const frameCtx = frameCanvas.getContext('2d')!;

    const tempCanvas = document.createElement('canvas');
    const tv = new ReplayViewer(tempCanvas, { cellSize: 6, showGrid: false });
    tv.loadReplay(r);

    const encoder = new GIFEncoder(gw, gh, fps);

    showProgress(0);

    for (let t = startTurn; t <= endTurn; t++) {
      tv.setTurn(t);
      drawCompositeFrame(frameCanvas, tempCanvas, preset, { w: gw, h: gh }, t);
      const imgData = frameCtx.getImageData(0, 0, gw, gh);
      encoder.addFrame(imgData);
      updateProgress(((t - startTurn) / totalFrames) * 100);
      // Yield to keep browser responsive
      if ((t - startTurn) % 5 === 0) await sleep(0);
    }

    hideProgress();
    const gif = encoder.encode();
    const blob = new Blob([gif.buffer as ArrayBuffer], { type: 'image/gif' });
    const filename = `acb-clip-${r.match_id}-${preset.name.replace(/\s+/g, '_')}.gif`;
    // lastExportBlob = blob;
    lastExportExt = 'gif';
    downloadBlob(blob, filename);
    showSharePanel(r, startTurn, endTurn, blob);
  }

  function showProgress(pct: number): void {
    const p = document.getElementById('clip-export-progress')!;
    p.classList.remove('hidden');
    setProgress(pct);
  }

  function updateProgress(pct: number): void {
    setProgress(pct);
  }

  function hideProgress(): void {
    document.getElementById('clip-export-progress')!.classList.add('hidden');
  }

  function setProgress(pct: number): void {
    (document.getElementById('clip-progress-fill') as HTMLElement).style.width = `${pct.toFixed(0)}%`;
    (document.getElementById('clip-progress-label') as HTMLElement).textContent = `${pct.toFixed(0)}%`;
  }

  // ── Share panel ──────────────────────────────────────────────────────────

  function generateShareText(r: Replay): string {
    const names = r.players.map(p => p.name);
    const scores = r.result.scores;
    const winnerName = r.players[r.result.winner]?.name ?? 'Unknown';
    const loserIdx = r.players.findIndex((_, i) => i !== r.result.winner);
    const loserName = loserIdx >= 0 ? r.players[loserIdx].name : '';

    if (names.length === 2) {
      return `${winnerName} defeats ${loserName} ${scores[0]}-${scores[1]} on AI Code Battle!`;
    }
    return `${winnerName} wins! ${names.map((n, i) => `${n}: ${scores[i]}`).join(', ')}`;
  }

  function replayURL(matchId: string, startTurn: number, endTurn: number): string {
    return `https://aicodebattle.com/replay/${matchId}#turns=${startTurn}-${endTurn}`;
  }

  function showSharePanel(r: Replay, startTurn: number, endTurn: number, blob: Blob): void {
    const panel = document.getElementById('clip-share-panel')!;
    const textEl = document.getElementById('clip-share-text')!;
    const nativeEl = document.getElementById('clip-share-native')!;

    const text = generateShareText(r);
    const url = replayURL(r.match_id, startTurn, endTurn);
    textEl.textContent = `${text} ${url}`;

    // Web Share API availability
    const file = new File([blob], `acb-clip-${r.match_id}.${lastExportExt}`, { type: blob.type });
    const canShareFiles = 'canShare' in navigator && navigator.canShare({ files: [file] });
    if (canShareFiles || 'share' in navigator) {
      nativeEl.classList.remove('hidden');
    } else {
      nativeEl.classList.add('hidden');
    }

    // Wire share buttons
    const twitterBtn = document.getElementById('share-twitter-btn')!;
    const redditBtn = document.getElementById('share-reddit-btn')!;
    const discordBtn = document.getElementById('share-discord-btn')!;
    const copyBtn = document.getElementById('share-copy-btn')!;
    const nativeBtn = document.getElementById('share-native-btn')!;

    // Clone and replace to remove old listeners
    const newTwitter = twitterBtn.cloneNode(true) as HTMLElement;
    const newReddit = redditBtn.cloneNode(true) as HTMLElement;
    const newDiscord = discordBtn.cloneNode(true) as HTMLElement;
    const newCopy = copyBtn.cloneNode(true) as HTMLElement;
    const newNative = nativeBtn.cloneNode(true) as HTMLElement;
    twitterBtn.replaceWith(newTwitter);
    redditBtn.replaceWith(newReddit);
    discordBtn.replaceWith(newDiscord);
    copyBtn.replaceWith(newCopy);
    nativeBtn.replaceWith(newNative);

    newTwitter.addEventListener('click', () => {
      const tweetText = encodeURIComponent(`${text} ${url}`);
      window.open(`https://twitter.com/intent/tweet?text=${tweetText}`, '_blank', 'noopener');
    });

    newReddit.addEventListener('click', () => {
      const md = `[${text}](${url})`;
      copyToClipboard(md);
      flashToast('Reddit markdown copied!');
    });

    newDiscord.addEventListener('click', () => {
      downloadBlob(blob, `acb-clip-${r.match_id}.${lastExportExt}`);
      copyToClipboard(`${text} ${url}`);
      flashToast('File downloaded — link copied for caption!');
    });

    newCopy.addEventListener('click', () => {
      copyToClipboard(`${text} ${url}`);
      flashToast('Link copied!');
    });

    newNative.addEventListener('click', async () => {
      try {
        const shareData: ShareData = { text: `${text} ${url}`, title: 'AI Code Battle Clip' };
        if (canShareFiles) shareData.files = [file];
        await navigator.share(shareData);
      } catch {
        // User cancelled or not supported — do nothing
      }
    });

    panel.classList.remove('hidden');
  }

  function copyToClipboard(text: string): void {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(text);
    } else {
      const ta = document.createElement('textarea');
      ta.value = text;
      ta.style.position = 'fixed';
      ta.style.left = '-9999px';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    }
  }

  function flashToast(msg: string): void {
    const toast = document.getElementById('share-toast')!;
    toast.textContent = msg;
    toast.classList.remove('hidden');
    setTimeout(() => toast.classList.add('hidden'), 2000);
  }
}

// ─── Composite frame renderer ─────────────────────────────────────────────────
// Renders a game frame onto a target canvas with the chosen social aspect ratio,
// adding letterbox/pillarbox and a title bar.

function drawCompositeFrame(
  target: HTMLCanvasElement,
  gameCanvas: HTMLCanvasElement,
  _preset: SocialPreset,
  dims: { w: number; h: number },
  turn: number,
): void {
  target.width = dims.w;
  target.height = dims.h;

  const ctx = target.getContext('2d')!;
  ctx.fillStyle = '#0f172a';
  ctx.fillRect(0, 0, dims.w, dims.h);

  // Title bar height (proportional)
  const barH = Math.round(dims.h * 0.07);
  const barY = dims.h - barH;

  // Game area (keep game canvas aspect ratio, fit inside dims minus bars)
  const gameW = gameCanvas.width;
  const gameH = gameCanvas.height;
  const avW = dims.w;
  const avH = dims.h - barH * 2;

  const scale = Math.min(avW / gameW, avH / gameH);
  const dw = Math.round(gameW * scale);
  const dh = Math.round(gameH * scale);
  const dx = Math.round((dims.w - dw) / 2);
  const dy = barH + Math.round((avH - dh) / 2);

  ctx.drawImage(gameCanvas, dx, dy, dw, dh);

  // Top bar: title
  ctx.fillStyle = 'rgba(15,23,42,0.85)';
  ctx.fillRect(0, 0, dims.w, barH);

  const fontSize = Math.max(10, Math.round(barH * 0.45));
  ctx.fillStyle = '#f8fafc';
  ctx.font = `600 ${fontSize}px -apple-system, sans-serif`;
  ctx.textAlign = 'left';
  ctx.textBaseline = 'middle';
  ctx.fillText('AI Code Battle', Math.round(dims.w * 0.03), barH / 2);

  // Bottom bar: turn info
  ctx.fillStyle = 'rgba(15,23,42,0.85)';
  ctx.fillRect(0, barY, dims.w, barH);

  ctx.fillStyle = '#94a3b8';
  ctx.font = `${fontSize}px -apple-system, sans-serif`;
  ctx.textAlign = 'right';
  ctx.fillText(`Turn ${turn}`, dims.w - Math.round(dims.w * 0.03), barY + barH / 2);
}

// ─── GIF encoder ─────────────────────────────────────────────────────────────

class GIFEncoder {
  private width: number;
  private height: number;
  private delay: number; // centiseconds per frame
  private palette: Uint8Array; // 256×3 RGB
  private frames: Uint8Array[] = [];

  constructor(width: number, height: number, fps: number) {
    this.width = width;
    this.height = height;
    this.delay = Math.round(100 / fps);
    this.palette = buildGIFPalette();
  }

  addFrame(imgData: ImageData): void {
    const indices = quantizeFrame(imgData, this.palette);
    const lzw = lzwEncode(indices, 8);
    this.frames.push(lzw);
  }

  encode(): Uint8Array {
    const out: number[] = [];

    // GIF89a header
    for (const c of [0x47, 0x49, 0x46, 0x38, 0x39, 0x61]) out.push(c);

    // Logical screen descriptor
    out.push(this.width & 0xFF, (this.width >> 8) & 0xFF);
    out.push(this.height & 0xFF, (this.height >> 8) & 0xFF);
    // Packed: GlobalCT=1, colorRes=7, sort=0, globalCT size=7 (2^8=256 colors)
    out.push(0b11110111);
    out.push(0); // bg color index
    out.push(0); // pixel aspect ratio

    // Global color table (256 × 3 bytes)
    for (let i = 0; i < this.palette.length; i++) out.push(this.palette[i]);

    // Netscape looping extension (loop forever)
    out.push(0x21, 0xFF, 0x0B);
    for (const c of [78,69,84,83,67,65,80,69,50,46,48]) out.push(c); // NETSCAPE2.0
    out.push(0x03, 0x01, 0x00, 0x00, 0x00); // loop count = 0 (infinite)

    // Frames
    for (const frame of this.frames) {
      // Graphic Control Extension
      out.push(0x21, 0xF9, 0x04);
      out.push(0b00000100); // disposal: restore to background
      out.push(this.delay & 0xFF, (this.delay >> 8) & 0xFF);
      out.push(0x00); // transparent color index (none)
      out.push(0x00); // block terminator

      // Image Descriptor
      out.push(0x2C);
      out.push(0, 0, 0, 0); // left, top
      out.push(this.width & 0xFF, (this.width >> 8) & 0xFF);
      out.push(this.height & 0xFF, (this.height >> 8) & 0xFF);
      out.push(0x00); // no local color table, not interlaced

      // LZW minimum code size
      out.push(0x08);

      // LZW data in sub-blocks (max 255 bytes each)
      let i = 0;
      while (i < frame.length) {
        const blockSize = Math.min(255, frame.length - i);
        out.push(blockSize);
        for (let j = 0; j < blockSize; j++) out.push(frame[i + j]);
        i += blockSize;
      }
      out.push(0x00); // block terminator
    }

    // GIF trailer
    out.push(0x3B);

    return new Uint8Array(out);
  }
}

// Build a 256-color palette: 6×6×6 web-safe cube (216) + 40 game-specific colors
function buildGIFPalette(): Uint8Array {
  const buf = new Uint8Array(256 * 3);
  let idx = 0;

  // 216 web-safe colors
  for (let r = 0; r < 6; r++) {
    for (let g = 0; g < 6; g++) {
      for (let b = 0; b < 6; b++) {
        buf[idx++] = r * 51;
        buf[idx++] = g * 51;
        buf[idx++] = b * 51;
      }
    }
  }

  // Game-specific dark theme colors
  const extra: [number, number, number][] = [
    [15, 23, 42],    // bg-primary
    [30, 41, 59],    // bg-secondary
    [51, 65, 85],    // bg-tertiary
    [71, 85, 105],   // border
    [248, 250, 252], // text-primary (near white)
    [148, 163, 184], // text-muted
    [59, 130, 246],  // accent blue (player 0)
    [239, 68, 68],   // error red (player 1)
    [34, 197, 94],   // success green (energy)
    [245, 158, 11],  // warning amber
    [167, 139, 250], // purple
    [96, 165, 250],  // light blue core
    [248, 113, 113], // light red core
    [134, 239, 172], // light green energy
    [251, 191, 36],  // yellow energy
    [17, 24, 39],    // very dark bg
    [31, 41, 55],    // wall color
    [55, 65, 81],    // grid color
    [226, 232, 240], // text-secondary
    [100, 116, 139], // slate-500
    [30, 64, 175],   // blue-800
    [153, 27, 27],   // red-800
    [20, 83, 45],    // green-800
    [120, 53, 15],   // amber-800
    [109, 40, 217],  // violet-700
    [186, 230, 253], // sky-200
    [254, 202, 202], // red-200
    [187, 247, 208], // green-200
    [254, 240, 138], // yellow-200
    [221, 214, 254], // violet-200
    [14, 165, 233],  // sky-500
    [236, 72, 153],  // pink-500
    [168, 85, 247],  // purple-500
    [245, 101, 101], // red-400
    [74, 222, 128],  // green-400
    [251, 211, 141], // amber-300
    [147, 197, 253], // blue-300
    [240, 171, 252], // fuchsia-300
    [0, 0, 0],       // black
    [255, 255, 255], // white
  ];

  for (const [r, g, b] of extra) {
    if (idx >= 256 * 3) break;
    buf[idx++] = r;
    buf[idx++] = g;
    buf[idx++] = b;
  }

  return buf;
}

// Map each RGBA pixel to nearest palette index
function quantizeFrame(imgData: ImageData, palette: Uint8Array): Uint8Array {
  const { data, width, height } = imgData;
  const result = new Uint8Array(width * height);
  const numColors = palette.length / 3;

  for (let i = 0; i < width * height; i++) {
    const r = data[i * 4];
    const g = data[i * 4 + 1];
    const b = data[i * 4 + 2];
    result[i] = nearestPalette(r, g, b, palette, numColors);
  }
  return result;
}

function nearestPalette(r: number, g: number, b: number, palette: Uint8Array, numColors: number): number {
  let bestIdx = 0;
  let bestDist = 0x7FFFFFFF;
  for (let i = 0; i < numColors; i++) {
    const dr = r - palette[i * 3];
    const dg = g - palette[i * 3 + 1];
    const db = b - palette[i * 3 + 2];
    const dist = dr * dr + dg * dg + db * db;
    if (dist < bestDist) {
      bestDist = dist;
      bestIdx = i;
      if (dist === 0) break; // exact match
    }
  }
  return bestIdx;
}

// GIF LZW compression (GIF variant, LSB-first bit packing)
function lzwEncode(pixels: Uint8Array, minCodeSize: number): Uint8Array {
  const clearCode = 1 << minCodeSize;
  const endCode = clearCode + 1;

  let codeSize = minCodeSize + 1;
  let nextCode = endCode + 1;

  const output: number[] = [];
  let buf = 0;
  let bufBits = 0;

  const emit = (code: number) => {
    buf |= code << bufBits;
    bufBits += codeSize;
    while (bufBits >= 8) {
      output.push(buf & 0xFF);
      buf >>>= 8;
      bufBits -= 8;
    }
  };

  // Code table: string → code index
  const table = new Map<string, number>();

  const initTable = () => {
    table.clear();
    for (let i = 0; i < clearCode; i++) {
      table.set(String.fromCharCode(i), i);
    }
    nextCode = endCode + 1;
    codeSize = minCodeSize + 1;
  };

  initTable();
  emit(clearCode);

  if (pixels.length === 0) {
    emit(endCode);
    if (bufBits > 0) output.push(buf & 0xFF);
    return new Uint8Array(output);
  }

  let str = String.fromCharCode(pixels[0]);

  for (let i = 1; i < pixels.length; i++) {
    const c = String.fromCharCode(pixels[i]);
    const concat = str + c;

    if (table.has(concat)) {
      str = concat;
    } else {
      emit(table.get(str)!);

      if (nextCode < 4096) {
        table.set(concat, nextCode++);
        // Increase code size when we've exhausted current range
        if (nextCode >= (1 << codeSize) && codeSize < 12) {
          codeSize++;
        }
      } else {
        // Code table full, emit clear and reset
        emit(clearCode);
        initTable();
      }

      str = c;
    }
  }

  emit(table.get(str)!);
  emit(endCode);

  if (bufBits > 0) output.push(buf & 0xFF);

  return new Uint8Array(output);
}

// ─── Utilities ────────────────────────────────────────────────────────────────

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// ─── Styles ───────────────────────────────────────────────────────────────────

const CLIP_STYLES = `
<style>
.clip-intro { color: var(--text-muted); margin-bottom: 24px; }
.clip-layout { display: flex; gap: 20px; align-items: flex-start; }
.clip-settings-col { width: 320px; flex-shrink: 0; display: flex; flex-direction: column; gap: 16px; }
.clip-preview-col { flex: 1; min-width: 0; }
.clip-panel { background: var(--bg-secondary); border-radius: 8px; padding: 16px; }
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; color: var(--text-primary); }
.preview-info { font-size: 0.75rem; color: var(--text-muted); font-weight: 400; }
.load-controls { display: flex; flex-direction: column; gap: 10px; }
.url-row { display: flex; gap: 8px; }
.url-input { flex: 1; background: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 7px 10px; border-radius: 6px; font-size: 0.875rem; }
.clip-status { font-size: 0.8rem; padding: 8px; border-radius: 4px; margin-top: 8px; }
.clip-status.hidden { display: none; }
.clip-status.ok { background: rgba(34,197,94,0.15); color: var(--success); }
.clip-status.error { background: rgba(239,68,68,0.15); color: var(--error); }
.clip-select, .clip-select-sm { width: 100%; background: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 8px; border-radius: 6px; font-size: 0.875rem; margin-bottom: 8px; }
.clip-select-sm { width: auto; }
.preset-dims { font-size: 0.75rem; color: var(--text-muted); }
.range-grid { display: grid; grid-template-columns: 1fr 2fr; gap: 8px 12px; align-items: center; font-size: 0.875rem; color: var(--text-muted); }
.range-row { display: flex; gap: 8px; align-items: center; }
.range-slider { flex: 1; }
.export-buttons { display: flex; gap: 10px; margin-bottom: 12px; }
.export-buttons .btn { flex: 1; }
.clip-progress.hidden { display: none; }
.progress-bar { height: 8px; background: var(--bg-tertiary); border-radius: 4px; overflow: hidden; margin-bottom: 4px; }
.progress-fill { height: 100%; background: var(--accent); border-radius: 4px; transition: width 0.1s; }
.preview-frame { display: flex; justify-content: center; align-items: center; min-height: 200px; background: var(--bg-primary); border-radius: 6px; padding: 8px; overflow: auto; }
.preview-frame canvas { display: block; max-width: 100%; }
.preview-nav { display: flex; justify-content: center; align-items: center; gap: 16px; margin-top: 12px; }
.frame-label { color: var(--text-muted); font-size: 0.875rem; min-width: 80px; text-align: center; }
.clip-share-panel.hidden { display: none; }
.share-preview-text { font-size: 0.8rem; color: var(--text-muted); background: var(--bg-primary); padding: 8px 10px; border-radius: 6px; margin-bottom: 12px; line-height: 1.4; word-break: break-word; }
.share-buttons { display: flex; gap: 8px; flex-wrap: wrap; }
.share-btn { flex: 1; min-width: 70px; font-size: 0.8rem; padding: 8px 6px; }
.share-twitter { background: #1d9bf0; color: #fff; border-color: #1d9bf0; }
.share-twitter:hover { background: #1a8cd8; }
.share-reddit { background: #ff4500; color: #fff; border-color: #ff4500; }
.share-reddit:hover { background: #e03e00; }
.share-discord { background: #5865f2; color: #fff; border-color: #5865f2; }
.share-discord:hover { background: #4752c4; }
.share-copy { background: var(--bg-tertiary); color: var(--text-primary); border-color: var(--border); }
.share-copy:hover { background: var(--border); }
.share-toast { position: relative; margin-top: 8px; padding: 6px 10px; font-size: 0.8rem; color: var(--success); background: rgba(34,197,94,0.15); border-radius: 4px; text-align: center; }
.share-toast.hidden { display: none; }
@media (max-width: 768px) {
  .clip-layout { flex-direction: column; }
  .clip-settings-col { width: 100%; }
}
</style>
`;
