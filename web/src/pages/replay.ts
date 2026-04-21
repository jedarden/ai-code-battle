// Standalone replay viewer page - lazy loaded from app.ts
import type { Replay, GameEvent } from '../types';

const loadReplayViewer = () => import('../replay-viewer');

export function renderReplayPage(params: Record<string, string>): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="replay-page">
      <h1 class="page-title">Replay Viewer</h1>
      <div id="replay-loading" style="text-align: center; padding: 60px 20px; color: var(--text-muted);">
        Loading replay viewer...
      </div>
    </div>
  `;

  loadReplayViewer().then(({ ReplayViewer }) => {
    initReplayViewerWithClass(ReplayViewer, params.url);
  });
}

function initReplayViewerWithClass(ReplayViewerClass: any, initialUrl?: string): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="replay-page">
      <h1 class="page-title">Replay Viewer</h1>

      <div class="replay-layout">
        <div class="replay-main">
          <div class="canvas-wrapper">
            <canvas id="replay-canvas"></canvas>
            <div id="no-replay" class="no-replay-message">Load a replay file to view</div>
          </div>
          <div id="win-prob-section" class="win-prob-section" style="display:none">
            <div class="win-prob-header">
              <span class="win-prob-title">Win Probability</span>
              <div class="critical-moment-nav">
                <button id="prev-critical-btn" class="btn" title="Previous critical moment" disabled>&#9664; Prev</button>
                <span id="critical-moment-info" class="critical-moment-info">&#8212;</span>
                <button id="next-critical-btn" class="btn" title="Next critical moment" disabled>Next &#9654;</button>
              </div>
            </div>
            <div id="win-prob-container" class="win-prob-container"></div>
            <div class="win-prob-legend">
              <span id="wp-p0-label" class="wp-legend-p0">&#8212; Player 0</span>
              <span id="wp-p1-label" class="wp-legend-p1">-- Player 1</span>
            </div>
          </div>
        </div>

        <div class="replay-sidebar">
          <div class="panel">
            <h2>Load Replay</h2>
            <div class="load-controls">
              <div class="file-input-wrapper">
                <label class="btn secondary" for="file-input">Choose File</label>
                <input type="file" id="file-input" accept=".json" style="display: none;">
              </div>
              <div class="url-input-group">
                <input type="text" id="url-input" placeholder="Or enter URL...">
                <button id="load-url-btn" class="btn primary">Load</button>
              </div>
            </div>
          </div>

          <div class="panel">
            <h2>Playback</h2>
            <div class="playback-controls">
              <button id="play-btn" class="btn" disabled>Play</button>
              <button id="prev-btn" class="btn" disabled>Prev</button>
              <button id="next-btn" class="btn" disabled>Next</button>
              <button id="reset-btn" class="btn" disabled>Reset</button>
            </div>
            <div class="slider-group">
              <label>Turn: <span id="turn-display">0</span> / <span id="total-turns">0</span></label>
              <input type="range" id="turn-slider" min="0" max="0" value="0" disabled>
            </div>
            <div class="slider-group">
              <label>Speed: <span id="speed-display">100</span>ms/turn</label>
              <input type="range" id="speed-slider" min="20" max="1000" value="100">
            </div>
          </div>

          <div class="panel">
            <h2>View Options</h2>
            <div class="view-options">
              <label for="fog-select">Fog of War:</label>
              <select id="fog-select">
                <option value="">Disabled (full view)</option>
              </select>
              <label for="cell-size-select" style="margin-top: 10px;">Cell Size:</label>
              <select id="cell-size-select">
                <option value="6">Small (6px)</option>
                <option value="8">Medium (8px)</option>
                <option value="10" selected>Large (10px)</option>
                <option value="12">X-Large (12px)</option>
              </select>
            </div>
          </div>

          <div class="panel">
            <h2>Accessibility</h2>
            <div class="accessibility-options">
              <label class="checkbox-label">
                <input type="checkbox" id="color-blind-toggle" checked>
                Color-blind safe palette
              </label>
              <label class="checkbox-label">
                <input type="checkbox" id="shapes-toggle" checked>
                Shapes per player
              </label>
              <label class="checkbox-label">
                <input type="checkbox" id="high-contrast-toggle">
                High contrast mode
              </label>
              <label class="checkbox-label">
                <input type="checkbox" id="reduced-motion-toggle">
                Reduced motion
              </label>
            </div>
          </div>

          <div class="panel">
            <h2>Match Info</h2>
            <dl class="match-info">
              <dt>Match ID</dt>
              <dd id="info-match-id">-</dd>
              <dt>Winner</dt>
              <dd id="info-winner">-</dd>
              <dt>Turns</dt>
              <dd id="info-turns">-</dd>
              <dt>Reason</dt>
              <dd id="info-reason">-</dd>
            </dl>
          </div>

          <div class="panel">
            <h2>Events This Turn</h2>
            <div class="event-log" id="event-log">
              <div class="no-events">No events</div>
            </div>
          </div>

          <div class="keyboard-shortcuts">
            <kbd>Space</kbd> Play/Pause
            <kbd>←</kbd><kbd>→</kbd> Step
            <kbd>Home</kbd><kbd>End</kbd> First/Last
          </div>
        </div>
      </div>
    </div>

    <style>
      .replay-page .page-title { margin-bottom: 20px; }
      .replay-layout { display: flex; gap: 20px; }
      .replay-main { flex: 1; min-width: 0; }
      .canvas-wrapper { background-color: var(--bg-secondary); border-radius: 8px; padding: 10px; overflow: auto; max-height: 80vh; }
      #replay-canvas { display: block; }
      .no-replay-message { color: var(--text-muted); text-align: center; padding: 60px 20px; }
      .replay-sidebar { width: 300px; flex-shrink: 0; display: flex; flex-direction: column; gap: 15px; }
      .panel { background-color: var(--bg-secondary); border-radius: 8px; padding: 15px; }
      .panel h2 { font-size: 1rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 12px; }
      .load-controls { display: flex; flex-direction: column; gap: 10px; }
      .url-input-group { display: flex; gap: 8px; }
      .url-input-group input { flex: 1; background-color: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 8px; border-radius: 6px; font-size: 14px; }
      .playback-controls { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
      .playback-controls .btn:disabled { opacity: 0.5; cursor: not-allowed; }
      .slider-group { margin-bottom: 10px; }
      .slider-group label { display: block; color: var(--text-muted); font-size: 0.875rem; margin-bottom: 6px; }
      .slider-group input[type="range"] { width: 100%; }
      .view-options { display: flex; flex-direction: column; }
      .view-options label { color: var(--text-muted); font-size: 0.875rem; margin-bottom: 6px; }
      .view-options select { background-color: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 8px; border-radius: 6px; font-size: 14px; }
      .accessibility-options { display: flex; flex-direction: column; gap: 8px; }
      .checkbox-label { display: flex; align-items: center; gap: 8px; cursor: pointer; color: var(--text-muted); font-size: 0.875rem; }
      .checkbox-label input[type="checkbox"] { width: 16px; height: 16px; accent-color: var(--accent); cursor: pointer; }
      .checkbox-label:hover { color: var(--text-primary); }
      .match-info dt { color: var(--text-muted); font-size: 0.75rem; text-transform: uppercase; margin-top: 10px; }
      .match-info dd { color: var(--text-primary); }
      .event-log { max-height: 150px; overflow-y: auto; font-size: 0.75rem; font-family: monospace; }
      .event-log .event { padding: 4px 0; border-bottom: 1px solid var(--bg-tertiary); }
      .event-log .event:last-child { border-bottom: none; }
      .no-events { color: var(--text-muted); }
      .keyboard-shortcuts { font-size: 0.75rem; color: var(--text-muted); }
      .keyboard-shortcuts kbd { background-color: var(--bg-tertiary); padding: 2px 6px; border-radius: 4px; font-family: monospace; margin-right: 4px; }
      .win-prob-section { background-color: var(--bg-secondary); border-radius: 8px; padding: 12px; margin-top: 10px; }
      .win-prob-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; flex-wrap: wrap; gap: 8px; }
      .win-prob-title { color: var(--text-muted); font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; font-weight: 600; }
      .critical-moment-nav { display: flex; align-items: center; gap: 8px; }
      .critical-moment-nav .btn { padding: 4px 10px; font-size: 0.75rem; }
      .critical-moment-nav .btn:disabled { opacity: 0.4; cursor: not-allowed; }
      .critical-moment-info { color: var(--text-muted); font-size: 0.8rem; max-width: 280px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .win-prob-container { width: 100%; overflow: hidden; border-radius: 4px; }
      .win-prob-legend { display: flex; gap: 16px; margin-top: 6px; font-size: 0.75rem; }
      .wp-legend-p0 { color: #3b82f6; }
      .wp-legend-p1 { color: #ef4444; }
      @media (max-width: 900px) {
        .replay-layout { flex-direction: column; }
        .replay-sidebar { width: 100%; }
      }
    </style>
  `;

  initReplayViewer(ReplayViewerClass, initialUrl);
}

function initReplayViewer(ReplayViewerClass: any, initialUrl?: string): void {
  const canvas = document.getElementById('replay-canvas') as HTMLCanvasElement;
  const noReplayDiv = document.getElementById('no-replay') as HTMLDivElement;
  const fileInput = document.getElementById('file-input') as HTMLInputElement;
  const urlInput = document.getElementById('url-input') as HTMLInputElement;
  const loadUrlBtn = document.getElementById('load-url-btn') as HTMLButtonElement;
  const playBtn = document.getElementById('play-btn') as HTMLButtonElement;
  const prevBtn = document.getElementById('prev-btn') as HTMLButtonElement;
  const nextBtn = document.getElementById('next-btn') as HTMLButtonElement;
  const resetBtn = document.getElementById('reset-btn') as HTMLButtonElement;
  const turnDisplay = document.getElementById('turn-display') as HTMLSpanElement;
  const totalTurnsSpan = document.getElementById('total-turns') as HTMLSpanElement;
  const turnSlider = document.getElementById('turn-slider') as HTMLInputElement;
  const speedDisplay = document.getElementById('speed-display') as HTMLSpanElement;
  const speedSlider = document.getElementById('speed-slider') as HTMLInputElement;
  const fogSelect = document.getElementById('fog-select') as HTMLSelectElement;
  const cellSizeSelect = document.getElementById('cell-size-select') as HTMLSelectElement;
  const eventLogDiv = document.getElementById('event-log') as HTMLDivElement;
  const infoMatchId = document.getElementById('info-match-id') as HTMLElement;
  const infoWinner = document.getElementById('info-winner') as HTMLElement;
  const infoTurns = document.getElementById('info-turns') as HTMLElement;
  const infoReason = document.getElementById('info-reason') as HTMLElement;
  const winProbSection = document.getElementById('win-prob-section') as HTMLDivElement;
  const winProbContainer = document.getElementById('win-prob-container') as HTMLDivElement;
  const prevCriticalBtn = document.getElementById('prev-critical-btn') as HTMLButtonElement;
  const nextCriticalBtn = document.getElementById('next-critical-btn') as HTMLButtonElement;
  const criticalMomentInfo = document.getElementById('critical-moment-info') as HTMLSpanElement;
  const wpP0Label = document.getElementById('wp-p0-label') as HTMLSpanElement;
  const wpP1Label = document.getElementById('wp-p1-label') as HTMLSpanElement;

  let viewer = new ReplayViewerClass(canvas, { cellSize: 10 });
  let criticalMoments: Array<{turn: number; delta: number; description: string}> = [];

  function enableControls(): void {
    playBtn.disabled = false;
    prevBtn.disabled = false;
    nextBtn.disabled = false;
    resetBtn.disabled = false;
    turnSlider.disabled = false;
    noReplayDiv.style.display = 'none';
  }

  function updateUI(): void {
    turnDisplay.textContent = String(viewer.getTurn());
    totalTurnsSpan.textContent = String(viewer.getTotalTurns());
    turnSlider.value = String(viewer.getTurn());
    playBtn.textContent = 'Pause';
    if (!viewer.getReplay() || viewer.isAtEnd()) {
      playBtn.textContent = 'Play';
    }
  }

  function updateEventLog(): void {
    const events = viewer.getTurnEvents();
    if (events.length === 0) {
      eventLogDiv.innerHTML = '<div class="no-events">No events</div>';
      return;
    }
    eventLogDiv.innerHTML = events.map((e: GameEvent) => {
      const type = e.type.replace(/_/g, ' ');
      return `<div class="event"><span style="color: #fbbf24;">${type}</span></div>`;
    }).join('');
  }

  function updateMatchInfo(replay: Replay): void {
    infoMatchId.textContent = replay.match_id;
    infoTurns.textContent = String(replay.result.turns);
    infoReason.textContent = replay.result.reason;

    if (replay.result.winner >= 0 && replay.result.winner < replay.players.length) {
      infoWinner.textContent = replay.players[replay.result.winner].name;
    } else if (replay.result.winner === -1) {
      infoWinner.textContent = 'Draw';
    } else {
      infoWinner.textContent = 'Player ' + replay.result.winner;
    }

    fogSelect.innerHTML = '<option value="">Disabled (full view)</option>';
    replay.players.forEach((player, idx) => {
      const option = document.createElement('option');
      option.value = String(idx);
      option.textContent = player.name;
      fogSelect.appendChild(option);
    });
  }

  function loadReplay(replay: Replay): void {
    viewer.loadReplay(replay);
    enableControls();
    updateMatchInfo(replay);
    turnSlider.max = String(viewer.getTotalTurns() - 1);
    updateUI();
    updateEventLog();
    initWinProb(replay);
  }

  function initWinProb(replay: Replay): void {
    if (!replay.win_prob || replay.win_prob.length === 0) {
      winProbSection.style.display = 'none';
      return;
    }

    const points = replay.win_prob.map((pair: any, t: number) => ({
      turn: t,
      p0WinProb: pair[0] ?? 0.5,
      p1WinProb: pair[1] ?? 0.5,
      drawProb: Math.max(0, 1 - (pair[0] ?? 0.5) - (pair[1] ?? 0.5)),
    }));

    criticalMoments = replay.critical_moments ?? [];

    viewer.setWinProbabilityData(points);
    viewer.setCriticalMoments(criticalMoments);

    winProbSection.style.display = 'block';

    if (replay.players.length >= 1) wpP0Label.textContent = `— ${replay.players[0].name}`;
    if (replay.players.length >= 2) wpP1Label.textContent = `-- ${replay.players[1].name}`;

    winProbContainer.innerHTML = '';
    viewer.createWinProbSparkline(winProbContainer, 800, 70, (turn: number) => {
      viewer.setTurn(turn);
      updateUI();
      updateEventLog();
    });

    updateCriticalMomentNav();
  }

  function updateCriticalMomentNav(): void {
    const hasMoments = criticalMoments.length > 0;
    prevCriticalBtn.disabled = !hasMoments;
    nextCriticalBtn.disabled = !hasMoments;

    if (hasMoments) {
      const currentTurn = viewer.getTurn();
      const atMoment = criticalMoments.find((m: any) => m.turn === currentTurn);
      if (atMoment) {
        criticalMomentInfo.textContent = atMoment.description;
      } else {
        criticalMomentInfo.textContent = `${criticalMoments.length} critical moment${criticalMoments.length !== 1 ? 's' : ''}`;
      }
    } else {
      criticalMomentInfo.textContent = '—';
    }
  }

  prevCriticalBtn.addEventListener('click', () => {
    const currentTurn = viewer.getTurn();
    const prev = [...criticalMoments].reverse().find((m: any) => m.turn < currentTurn);
    if (prev) {
      viewer.setTurn(prev.turn);
      updateUI();
      updateEventLog();
      criticalMomentInfo.textContent = prev.description;
    }
  });

  nextCriticalBtn.addEventListener('click', () => {
    const currentTurn = viewer.getTurn();
    const next = criticalMoments.find((m: any) => m.turn > currentTurn);
    if (next) {
      viewer.setTurn(next.turn);
      updateUI();
      updateEventLog();
      criticalMomentInfo.textContent = next.description;
    }
  });

  fileInput.addEventListener('change', async (e) => {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      const replay = JSON.parse(text) as Replay;
      loadReplay(replay);
    } catch (err) {
      alert('Failed to load replay: ' + err);
    }
  });

  loadUrlBtn.addEventListener('click', async () => {
    const url = urlInput.value.trim();
    if (!url) return;
    try {
      const response = await fetch(url);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const replay = await response.json() as Replay;
      loadReplay(replay);
    } catch (err) {
      alert('Failed to load replay from URL: ' + err);
    }
  });

  playBtn.addEventListener('click', () => viewer.togglePlay());
  prevBtn.addEventListener('click', () => { viewer.setTurn(viewer.getTurn() - 1); updateUI(); updateEventLog(); });
  nextBtn.addEventListener('click', () => { viewer.setTurn(viewer.getTurn() + 1); updateUI(); updateEventLog(); });
  resetBtn.addEventListener('click', () => { viewer.pause(); viewer.setTurn(0); updateUI(); updateEventLog(); });

  turnSlider.addEventListener('input', () => {
    viewer.setTurn(parseInt(turnSlider.value, 10));
    updateUI();
    updateEventLog();
  });

  speedSlider.addEventListener('input', () => {
    const speed = parseInt(speedSlider.value, 10);
    viewer.setSpeed(speed);
    speedDisplay.textContent = String(speed);
  });

  fogSelect.addEventListener('change', () => {
    const value = fogSelect.value;
    viewer.setFogOfWar(value === '' ? null : parseInt(value, 10));
  });

  cellSizeSelect.addEventListener('change', () => {
    const size = parseInt(cellSizeSelect.value, 10);
    const replay = viewer.getReplay();
    if (replay) {
      const prevTurn = viewer.getTurn();
      viewer.destroy();
      viewer = new ReplayViewerClass(canvas, { cellSize: size });
      loadReplay(replay);
      viewer.setTurn(prevTurn);
      updateUI();
    }
  });

  // Accessibility toggle handlers
  const colorBlindToggle = document.getElementById('color-blind-toggle') as HTMLInputElement;
  const shapesToggle = document.getElementById('shapes-toggle') as HTMLInputElement;
  const highContrastToggle = document.getElementById('high-contrast-toggle') as HTMLInputElement;
  const reducedMotionToggle = document.getElementById('reduced-motion-toggle') as HTMLInputElement;

  function updateAccessibility(): void {
    viewer.setAccessibility({
      colorBlindSafe: colorBlindToggle.checked,
      showShapes: shapesToggle.checked,
      highContrast: highContrastToggle.checked,
      reducedMotion: reducedMotionToggle.checked,
    });
  }

  colorBlindToggle.addEventListener('change', updateAccessibility);
  shapesToggle.addEventListener('change', updateAccessibility);
  highContrastToggle.addEventListener('change', updateAccessibility);
  reducedMotionToggle.addEventListener('change', updateAccessibility);

  // Initialize accessibility from system preferences
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    reducedMotionToggle.checked = true;
    updateAccessibility();
  }

  viewer.onTurnChange = () => {
    updateUI();
    updateEventLog();
    if (criticalMoments.length > 0) updateCriticalMomentNav();
  };
  viewer.onPlayStateChange = (playing: boolean) => { playBtn.textContent = playing ? 'Pause' : 'Play'; };

  document.addEventListener('keydown', (e) => {
    if (!viewer.getReplay()) return;
    switch (e.code) {
      case 'Space':
        e.preventDefault();
        viewer.togglePlay();
        break;
      case 'ArrowLeft':
        e.preventDefault();
        viewer.setTurn(viewer.getTurn() - 1);
        updateUI();
        updateEventLog();
        break;
      case 'ArrowRight':
        e.preventDefault();
        viewer.setTurn(viewer.getTurn() + 1);
        updateUI();
        updateEventLog();
        break;
      case 'Home':
        e.preventDefault();
        viewer.setTurn(0);
        updateUI();
        updateEventLog();
        break;
      case 'End':
        e.preventDefault();
        viewer.setTurn(viewer.getTotalTurns() - 1);
        updateUI();
        updateEventLog();
        break;
    }
  });

  // Load from URL param if provided
  if (initialUrl) {
    urlInput.value = initialUrl;
    loadUrlBtn.click();
  }
}
