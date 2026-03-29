// Main SPA entry point with routing
import { router } from './router';
import { renderHomePage } from './pages/home';
import { renderLeaderboardPage } from './pages/leaderboard';
import { renderMatchesPage } from './pages/matches';
import { renderBotsPage } from './pages/bots';
import { renderBotProfilePage } from './pages/bot-profile';
import { renderRegisterPage } from './pages/register';
import { renderEvolutionPage } from './pages/evolution';
import { renderSandboxPage } from './pages/sandbox';
import { renderClipMakerPage } from './pages/clip-maker';
import { renderRivalriesPage } from './pages/rivalries';
import { renderFeedbackPage } from './pages/feedback';
import { renderPlaylistsPage } from './pages/playlists';
import { renderBlogPage, renderBlogPostPage } from './pages/blog';
import { ReplayViewer } from './replay-viewer';
import type { Replay } from './types';

// Route definitions
router
  .on('/', renderHomePage)
  .on('/leaderboard', renderLeaderboardPage)
  .on('/matches', renderMatchesPage)
  .on('/bots', renderBotsPage)
  .on('/bot/:id', renderBotProfilePage)
  .on('/register', renderRegisterPage)
  .on('/evolution', renderEvolutionPage)
  .on('/sandbox', renderSandboxPage)
  .on('/clip-maker', renderClipMakerPage)
  .on('/rivalries', renderRivalriesPage)
  .on('/feedback', renderFeedbackPage)
  .on('/playlists', renderPlaylistsPage)
  .on('/blog', renderBlogPage)
  .on('/blog/:slug', renderBlogPostPage)
  .on('/replay', renderReplayPage)
  .on('/docs', renderDocsPage)
  .notFound(renderNotFoundPage);

// Update active nav link on route change
function updateActiveNavLink(): void {
  const currentPath = router.getCurrentPath();
  document.querySelectorAll('.nav-link').forEach(link => {
    const href = link.getAttribute('href');
    if (href) {
      const linkPath = href.slice(2); // Remove '#/'
      if (currentPath === linkPath || (linkPath !== '' && currentPath.startsWith(linkPath))) {
        link.classList.add('active');
      } else {
        link.classList.remove('active');
      }
    }
  });
}

// Override router navigation to update nav links
const originalNavigate = router.navigate.bind(router);
router.navigate = (path: string) => {
  originalNavigate(path);
  updateActiveNavLink();
};

// Replay viewer page
function renderReplayPage(params: Record<string, string>): void {
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
      .replay-page .page-title {
        margin-bottom: 20px;
      }

      .replay-layout {
        display: flex;
        gap: 20px;
      }

      .replay-main {
        flex: 1;
        min-width: 0;
      }

      .canvas-wrapper {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 10px;
        overflow: auto;
        max-height: 80vh;
      }

      #replay-canvas {
        display: block;
      }

      .no-replay-message {
        color: var(--text-muted);
        text-align: center;
        padding: 60px 20px;
      }

      .replay-sidebar {
        width: 300px;
        flex-shrink: 0;
        display: flex;
        flex-direction: column;
        gap: 15px;
      }

      .panel {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 15px;
      }

      .panel h2 {
        font-size: 1rem;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
        margin-bottom: 12px;
      }

      .load-controls {
        display: flex;
        flex-direction: column;
        gap: 10px;
      }

      .url-input-group {
        display: flex;
        gap: 8px;
      }

      .url-input-group input {
        flex: 1;
        background-color: var(--bg-primary);
        border: 1px solid var(--border);
        color: var(--text-primary);
        padding: 8px;
        border-radius: 6px;
        font-size: 14px;
      }

      .playback-controls {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
        margin-bottom: 12px;
      }

      .playback-controls .btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }

      .slider-group {
        margin-bottom: 10px;
      }

      .slider-group label {
        display: block;
        color: var(--text-muted);
        font-size: 0.875rem;
        margin-bottom: 6px;
      }

      .slider-group input[type="range"] {
        width: 100%;
      }

      .view-options {
        display: flex;
        flex-direction: column;
      }

      .view-options label {
        color: var(--text-muted);
        font-size: 0.875rem;
        margin-bottom: 6px;
      }

      .view-options select {
        background-color: var(--bg-primary);
        border: 1px solid var(--border);
        color: var(--text-primary);
        padding: 8px;
        border-radius: 6px;
        font-size: 14px;
      }

      .accessibility-options {
        display: flex;
        flex-direction: column;
        gap: 8px;
      }

      .checkbox-label {
        display: flex;
        align-items: center;
        gap: 8px;
        cursor: pointer;
        color: var(--text-muted);
        font-size: 0.875rem;
      }

      .checkbox-label input[type="checkbox"] {
        width: 16px;
        height: 16px;
        accent-color: var(--accent);
        cursor: pointer;
      }

      .checkbox-label:hover {
        color: var(--text-primary);
      }

      .match-info dt {
        color: var(--text-muted);
        font-size: 0.75rem;
        text-transform: uppercase;
        margin-top: 10px;
      }

      .match-info dd {
        color: var(--text-primary);
      }

      .event-log {
        max-height: 150px;
        overflow-y: auto;
        font-size: 0.75rem;
        font-family: monospace;
      }

      .event-log .event {
        padding: 4px 0;
        border-bottom: 1px solid var(--bg-tertiary);
      }

      .event-log .event:last-child {
        border-bottom: none;
      }

      .no-events {
        color: var(--text-muted);
      }

      .keyboard-shortcuts {
        font-size: 0.75rem;
        color: var(--text-muted);
      }

      .keyboard-shortcuts kbd {
        background-color: var(--bg-tertiary);
        padding: 2px 6px;
        border-radius: 4px;
        font-family: monospace;
        margin-right: 4px;
      }

      @media (max-width: 900px) {
        .replay-layout {
          flex-direction: column;
        }

        .replay-sidebar {
          width: 100%;
        }
      }
    </style>
  `;

  // Initialize replay viewer
  initReplayViewer(params.url);
}

function initReplayViewer(initialUrl?: string): void {
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

  let viewer = new ReplayViewer(canvas, { cellSize: 10 });

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
    eventLogDiv.innerHTML = events.map(e => {
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
  }

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
      viewer = new ReplayViewer(canvas, { cellSize: size });
      loadReplay(replay);
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

  viewer.onTurnChange = () => { updateUI(); updateEventLog(); };
  viewer.onPlayStateChange = (playing) => { playBtn.textContent = playing ? 'Pause' : 'Play'; };

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

// Docs/Getting Started page
function renderDocsPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="docs-page">
      <h1 class="page-title">Getting Started</h1>

      <div class="docs-content">
        <section>
          <h2>Overview</h2>
          <p>AI Code Battle is a competitive bot programming platform. You write an HTTP server that controls units on a grid world, competing against other bots for supremacy.</p>
        </section>

        <section>
          <h2>Game Basics</h2>
          <ul>
            <li><strong>Grid:</strong> The game is played on a toroidal (wrapping) grid</li>
            <li><strong>Units:</strong> Each player controls bots that move one tile per turn</li>
            <li><strong>Resources:</strong> Collect energy from nodes to spawn new bots</li>
            <li><strong>Objectives:</strong> Capture enemy cores, eliminate opponents, or dominate through numbers</li>
          </ul>
        </section>

        <section>
          <h2>HTTP Protocol</h2>
          <p>Your bot must expose an HTTPS endpoint that accepts POST requests with JSON game state and returns JSON move commands.</p>

          <h3>Request Format</h3>
          <pre><code>{
  "match_id": "abc123",
  "turn": 42,
  "player_id": 0,
  "config": { ... },
  "visible_grid": { ... },
  "my_bots": [
    { "id": "bot-1", "position": {"row": 10, "col": 20} }
  ],
  "my_energy": 5,
  "my_score": 3
}</code></pre>

          <h3>Response Format</h3>
          <pre><code>{
  "moves": [
    { "bot_id": "bot-1", "direction": "N" }
  ]
}</code></pre>

          <h3>Valid Directions</h3>
          <p><code>N</code> (North), <code>E</code> (East), <code>S</code> (South), <code>W</code> (West)</p>
        </section>

        <section>
          <h2>Authentication</h2>
          <p>Requests from the game engine are signed with HMAC-SHA256. The signature is sent in the <code>X-Signature</code> header.</p>
          <p>Format: <code>{match_id}.{turn}.{timestamp}.{sha256(body)}</code></p>
          <p>Your bot should verify signatures using your API key to ensure requests are authentic.</p>
        </section>

        <section>
          <h2>Requirements</h2>
          <ul>
            <li>HTTPS endpoint accessible from the internet</li>
            <li>Response time under 3 seconds per turn</li>
            <li>Handle concurrent requests (multiple matches)</li>
            <li>Return valid JSON for every request</li>
          </ul>
        </section>

        <section>
          <h2>Example Bot</h2>
          <p>See the <a href="https://github.com/aicodebattle/acb/tree/main/bots" target="_blank">example bots</a> in various languages for reference implementations.</p>
        </section>
      </div>
    </div>

    <style>
      .docs-content {
        max-width: 800px;
      }

      .docs-content section {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 20px;
        margin-bottom: 20px;
      }

      .docs-content h2 {
        color: var(--text-primary);
        margin-bottom: 12px;
      }

      .docs-content h3 {
        color: var(--text-primary);
        margin: 16px 0 8px;
        font-size: 1rem;
      }

      .docs-content p {
        color: var(--text-muted);
        margin-bottom: 10px;
      }

      .docs-content ul {
        color: var(--text-muted);
        margin-left: 20px;
      }

      .docs-content li {
        margin-bottom: 6px;
      }

      .docs-content pre {
        background-color: var(--bg-primary);
        border-radius: 6px;
        padding: 16px;
        overflow-x: auto;
        margin: 10px 0;
      }

      .docs-content code {
        font-family: 'Fira Code', 'Monaco', monospace;
        font-size: 0.875rem;
        color: var(--text-secondary);
      }

      .docs-content a {
        color: var(--accent);
      }
    </style>
  `;
}

// 404 page
function renderNotFoundPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="not-found-page">
      <h1>404</h1>
      <p>Page not found</p>
      <a href="#/" class="btn primary">Go Home</a>
    </div>

    <style>
      .not-found-page {
        text-align: center;
        padding: 100px 20px;
      }

      .not-found-page h1 {
        font-size: 4rem;
        color: var(--text-primary);
        margin-bottom: 10px;
      }

      .not-found-page p {
        color: var(--text-muted);
        margin-bottom: 20px;
      }
    </style>
  `;
}

// Start the router
document.addEventListener('DOMContentLoaded', () => {
  updateActiveNavLink();
  router.start();
});

// Update nav on initial load
window.addEventListener('load', () => {
  updateActiveNavLink();
});
