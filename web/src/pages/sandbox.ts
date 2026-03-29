// In-browser bot sandbox: Monaco editor + TS game engine + WASM upload + replay viewer
import { runMatch, defaultConfig, type Config, type BotStrategy, type VisibleState, type Move } from '../engine';
import { ReplayViewer } from '../replay-viewer';
import type { Replay } from '../types';

const WASM_BOT_SPEC = `// ACB WASM Bot Interface Spec (v1.0)
// ─────────────────────────────────────────────────────────────────────────────
// Your WASM file must export a global \`acbBot\` object with two functions:
//
//   acbBot.init(configJSON: string): void
//     Called once before the match starts. Receives the game Config as JSON.
//
//   acbBot.compute_moves(stateJSON: string): string
//     Called each turn. Receives a VisibleState JSON string; must return a
//     JSON array of Move objects:  [{"position":{"row":r,"col":c},"direction":"N"}]
//
// VisibleState schema:
//   { match_id, turn, config, you:{id,energy,score},
//     bots:[{position,owner}], energy:[{row,col}],
//     cores:[{position,owner,active}], walls:[{row,col}], dead:[] }
//
// Config schema:
//   { rows, cols, max_turns, vision_radius2, attack_radius2,
//     spawn_cost, energy_interval }
//
// Move schema:
//   { position:{row,col}, direction:"N"|"E"|"S"|"W"|"" }
//
// Build with: GOOS=js GOARCH=wasm go build -o mybot.wasm ./cmd/mybot/
// See docs/wasm-bot-interface.md for full examples.
`;

const STARTER_CODE = `// Starter bot – modify this code, then click "Run Match"
// The function receives a VisibleState and must return Move[]

function computeMoves(state) {
  const myID = state.you.id;
  const cfg = state.config;

  return state.bots
    .filter(b => b.owner === myID)
    .map(b => {
      // Find nearest energy
      let bestDir = ['N','E','S','W'][Math.floor(Math.random() * 4)];
      let bestDist = Infinity;

      for (const e of state.energy) {
        for (const dir of ['N','E','S','W']) {
          const np = applyDir(b.position, dir, cfg);
          const d = dist2(np, e, cfg);
          if (d < bestDist) { bestDist = d; bestDir = dir; }
        }
      }

      return { position: b.position, direction: bestDir };
    });
}

// Helpers (available in sandbox)
function applyDir(p, dir, cfg) {
  const deltas = { N:[-1,0], S:[1,0], E:[0,1], W:[0,-1] };
  const [dr, dc] = deltas[dir] ?? [0, 0];
  return {
    row: ((p.row + dr) % cfg.rows + cfg.rows) % cfg.rows,
    col: ((p.col + dc) % cfg.cols + cfg.cols) % cfg.cols
  };
}
function dist2(a, b, cfg) {
  let dr = Math.abs(a.row - b.row); let dc = Math.abs(a.col - b.col);
  if (dr > cfg.rows/2) dr = cfg.rows - dr;
  if (dc > cfg.cols/2) dc = cfg.cols - dc;
  return dr*dr + dc*dc;
}
`;

export function renderSandboxPage(_params: Record<string, string>): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = buildHTML();

  // Defer heavy init to avoid blocking render
  requestAnimationFrame(() => initSandbox());
}

function buildHTML(): string {
  return `
    <div class="sandbox-page">
      <h1 class="page-title">Bot Sandbox</h1>
      <p class="sandbox-intro">Write JavaScript bot logic, pick an opponent, and run an in-browser match instantly — no server required.</p>

      <div class="sandbox-layout">
        <!-- Left: editor -->
        <div class="sandbox-editor-col">
          <div class="sandbox-panel">
            <div class="panel-header">
              <span>Bot Code</span>
              <div class="panel-actions">
                <button id="wasm-upload-btn" class="btn small secondary" title="Upload a compiled .wasm bot">Upload WASM</button>
                <input type="file" id="wasm-file-input" accept=".wasm" style="display:none">
                <button id="reset-code-btn" class="btn small secondary">Reset</button>
              </div>
            </div>
            <div id="monaco-container" style="height:420px;border-radius:6px;overflow:hidden;"></div>
            <div id="wasm-status" class="wasm-status hidden"></div>
          </div>

          <div class="sandbox-panel">
            <div class="panel-header"><span>WASM Bot Interface Spec</span>
              <button id="toggle-spec-btn" class="btn small secondary">Show</button>
            </div>
            <pre id="wasm-spec" class="code-block hidden">${escapeHtml(WASM_BOT_SPEC)}</pre>
          </div>
        </div>

        <!-- Right: config + run + viewer -->
        <div class="sandbox-controls-col">
          <div class="sandbox-panel">
            <div class="panel-header"><span>Match Settings</span></div>
            <div class="settings-grid">
              <label>Opponent Strategy</label>
              <select id="opponent-select">
                <option value="random">Random</option>
                <option value="gatherer" selected>Gatherer</option>
                <option value="rusher">Rusher</option>
                <option value="guardian">Guardian</option>
                <option value="swarm">Swarm</option>
                <option value="hunter">Hunter</option>
              </select>

              <label>Grid Size</label>
              <select id="grid-size-select">
                <option value="20">Small (20×20)</option>
                <option value="30" selected>Medium (30×30)</option>
                <option value="40">Large (40×40)</option>
              </select>

              <label>Max Turns</label>
              <select id="max-turns-select">
                <option value="100">100</option>
                <option value="200" selected>200</option>
                <option value="300">300</option>
              </select>

              <label>Playback Speed</label>
              <input type="range" id="speed-slider" min="20" max="500" value="100" class="speed-slider">
              <span id="speed-label" class="speed-label">100ms</span>
            </div>
            <button id="run-btn" class="btn primary run-btn">Run Match</button>
          </div>

          <div class="sandbox-panel" id="result-panel" style="display:none">
            <div class="panel-header"><span>Match Result</span></div>
            <div id="match-result" class="match-result"></div>
          </div>

          <div class="sandbox-panel" id="performance-panel" style="display:none">
            <div class="panel-header"><span>Performance Stats</span></div>
            <div id="perf-stats" class="perf-stats"></div>
          </div>
        </div>
      </div>

      <!-- Replay viewer below -->
      <div id="replay-section" class="replay-section" style="display:none">
        <h2 class="section-title">Replay</h2>
        <div class="replay-layout-sandbox">
          <div class="canvas-wrapper">
            <canvas id="sandbox-canvas"></canvas>
          </div>
          <div class="sandbox-replay-controls">
            <div class="playback-controls">
              <button id="sb-play-btn" class="btn">Play</button>
              <button id="sb-prev-btn" class="btn">Prev</button>
              <button id="sb-next-btn" class="btn">Next</button>
              <button id="sb-reset-btn" class="btn">Reset</button>
              <button id="download-replay-btn" class="btn secondary">Download Replay</button>
            </div>
            <div class="slider-group">
              <label>Turn: <span id="sb-turn-display">0</span> / <span id="sb-total-turns">0</span></label>
              <input type="range" id="sb-turn-slider" min="0" max="0" value="0">
            </div>
            <div id="sb-events" class="event-log"></div>
          </div>
        </div>
      </div>
    </div>
    ${SANDBOX_STYLES}
  `;
}

function initSandbox(): void {
  let monacoEditor: any = null;
  let currentCode = STARTER_CODE;
  let wasmStrategy: BotStrategy | null = null;
  let lastReplay: any = null;
  let viewer: ReplayViewer | null = null;

  // ── Monaco editor ───────────────────────────────────────────────────────
  loadMonaco().then(monaco => {
    monacoEditor = monaco.editor.create(
      document.getElementById('monaco-container')!,
      {
        value: STARTER_CODE,
        language: 'javascript',
        theme: 'vs-dark',
        minimap: { enabled: false },
        fontSize: 13,
        lineNumbers: 'on',
        scrollBeyondLastLine: false,
        automaticLayout: true,
        wordWrap: 'on',
      },
    );
    monacoEditor.onDidChangeModelContent(() => {
      currentCode = monacoEditor.getValue();
    });
  }).catch(() => {
    // Monaco unavailable – use plain textarea fallback
    const container = document.getElementById('monaco-container')!;
    container.innerHTML = `<textarea id="code-textarea" style="width:100%;height:100%;background:#1e1e1e;color:#d4d4d4;font-family:monospace;font-size:13px;border:none;padding:10px;resize:none;">${escapeHtml(STARTER_CODE)}</textarea>`;
    const ta = document.getElementById('code-textarea') as HTMLTextAreaElement;
    ta.addEventListener('input', () => { currentCode = ta.value; });
  });

  // ── WASM upload ─────────────────────────────────────────────────────────
  document.getElementById('wasm-upload-btn')!.addEventListener('click', () => {
    document.getElementById('wasm-file-input')!.click();
  });

  document.getElementById('wasm-file-input')!.addEventListener('change', async (e) => {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    const status = document.getElementById('wasm-status')!;
    status.textContent = `Loading ${file.name}…`;
    status.className = 'wasm-status';

    try {
      wasmStrategy = await loadWasmBot(file);
      status.textContent = `WASM bot loaded: ${file.name}`;
      status.className = 'wasm-status ok';
      (document.getElementById('run-btn') as HTMLButtonElement).textContent = 'Run Match (WASM)';
    } catch (err) {
      status.textContent = `Failed to load WASM: ${err}`;
      status.className = 'wasm-status error';
    }
  });

  // ── Spec toggle ─────────────────────────────────────────────────────────
  document.getElementById('toggle-spec-btn')!.addEventListener('click', () => {
    const spec = document.getElementById('wasm-spec')!;
    const btn = document.getElementById('toggle-spec-btn')!;
    spec.classList.toggle('hidden');
    btn.textContent = spec.classList.contains('hidden') ? 'Show' : 'Hide';
  });

  // ── Reset code ──────────────────────────────────────────────────────────
  document.getElementById('reset-code-btn')!.addEventListener('click', () => {
    currentCode = STARTER_CODE;
    if (monacoEditor) monacoEditor.setValue(STARTER_CODE);
    wasmStrategy = null;
    const status = document.getElementById('wasm-status')!;
    status.className = 'wasm-status hidden';
    (document.getElementById('run-btn') as HTMLButtonElement).textContent = 'Run Match';
  });

  // ── Speed slider ────────────────────────────────────────────────────────
  document.getElementById('speed-slider')!.addEventListener('input', (e) => {
    const val = (e.target as HTMLInputElement).value;
    document.getElementById('speed-label')!.textContent = `${val}ms`;
    if (viewer) viewer.setSpeed(Number(val));
  });

  // ── Run match ───────────────────────────────────────────────────────────
  document.getElementById('run-btn')!.addEventListener('click', async () => {
    const btn = document.getElementById('run-btn') as HTMLButtonElement;
    btn.disabled = true;
    btn.textContent = 'Running…';

    try {
      const opponent = (document.getElementById('opponent-select') as HTMLSelectElement).value;
      const gridSize = Number((document.getElementById('grid-size-select') as HTMLSelectElement).value);
      const maxTurns = Number((document.getElementById('max-turns-select') as HTMLSelectElement).value);

      const cfg: Config = {
        ...defaultConfig(),
        rows: gridSize,
        cols: gridSize,
        max_turns: maxTurns,
      };

      // Build user strategy from code or WASM
      const userStrategy: BotStrategy = wasmStrategy ?? buildUserStrategy(currentCode);

      const t0 = performance.now();
      const { replay, result } = runMatch(cfg, userStrategy, opponent);
      const elapsed = performance.now() - t0;

      lastReplay = replay;

      // Show result
      const resultPanel = document.getElementById('result-panel')!;
      resultPanel.style.display = '';
      document.getElementById('match-result')!.innerHTML = formatResult(result, replay);

      // Performance panel
      const perfPanel = document.getElementById('performance-panel')!;
      perfPanel.style.display = '';
      document.getElementById('perf-stats')!.innerHTML = `
        <div class="perf-row"><span>Match duration (JS)</span><span>${elapsed.toFixed(1)} ms</span></div>
        <div class="perf-row"><span>Turns played</span><span>${result.turns}</span></div>
        <div class="perf-row"><span>Your bots alive</span><span>${result.bots_alive[0]}</span></div>
        <div class="perf-row"><span>Opponent bots alive</span><span>${result.bots_alive[1]}</span></div>
      `;

      // Show replay
      document.getElementById('replay-section')!.style.display = '';
      initReplayViewer(replay as any);

    } catch (err) {
      alert('Error running match: ' + err);
    } finally {
      btn.disabled = false;
      btn.textContent = wasmStrategy ? 'Run Match (WASM)' : 'Run Match';
    }
  });

  // ── Download replay ─────────────────────────────────────────────────────
  document.getElementById('download-replay-btn')?.addEventListener('click', () => {
    if (!lastReplay) return;
    const blob = new Blob([JSON.stringify(lastReplay, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `sandbox-replay-${Date.now()}.json`;
    a.click();
    URL.revokeObjectURL(url);
  });

  function initReplayViewer(replay: Replay): void {
    const canvas = document.getElementById('sandbox-canvas') as HTMLCanvasElement;
    const speed = Number((document.getElementById('speed-slider') as HTMLInputElement).value);
    viewer = new ReplayViewer(canvas, { cellSize: 12 });
    viewer.loadReplay(replay);
    viewer.setSpeed(speed);

    const turnDisplay = document.getElementById('sb-turn-display')!;
    const totalTurns = document.getElementById('sb-total-turns')!;
    const slider = document.getElementById('sb-turn-slider') as HTMLInputElement;
    const eventsDiv = document.getElementById('sb-events')!;

    totalTurns.textContent = String(viewer.getTotalTurns());
    slider.max = String(viewer.getTotalTurns() - 1);

    viewer.onTurnChange = () => {
      turnDisplay.textContent = String(viewer!.getTurn());
      slider.value = String(viewer!.getTurn());
      const events = viewer!.getTurnEvents();
      eventsDiv.innerHTML = events.length === 0
        ? '<div class="no-events">No events</div>'
        : events.map(ev => `<div class="event"><span style="color:#fbbf24">${ev.type.replace(/_/g,' ')}</span></div>`).join('');
    };

    document.getElementById('sb-play-btn')!.addEventListener('click', () => viewer!.togglePlay());
    document.getElementById('sb-prev-btn')!.addEventListener('click', () => { viewer!.setTurn(viewer!.getTurn() - 1); });
    document.getElementById('sb-next-btn')!.addEventListener('click', () => { viewer!.setTurn(viewer!.getTurn() + 1); });
    document.getElementById('sb-reset-btn')!.addEventListener('click', () => { viewer!.pause(); viewer!.setTurn(0); });

    slider.addEventListener('input', () => viewer!.setTurn(parseInt(slider.value, 10)));
  }
}

// ────────────────────────────────────────────────────────────────────────────
// User strategy builder (sandboxed eval)
// ────────────────────────────────────────────────────────────────────────────

function buildUserStrategy(code: string): BotStrategy {
  // Wrap the user's computeMoves function; catch errors gracefully
  return (state: VisibleState): Move[] => {
    try {
      // Create a sandboxed function using the user code
      const fn = new Function('state', `
        ${code}
        if (typeof computeMoves !== 'function') {
          throw new Error('computeMoves function not found');
        }
        return computeMoves(state);
      `);
      const result = fn(state);
      if (!Array.isArray(result)) return [];
      return result as Move[];
    } catch (err) {
      console.warn('User strategy error:', err);
      return [];
    }
  };
}

// ────────────────────────────────────────────────────────────────────────────
// WASM bot loader
// ────────────────────────────────────────────────────────────────────────────

async function loadWasmBot(file: File): Promise<BotStrategy> {
  const buffer = await file.arrayBuffer();

  // Try to instantiate the WASM module
  let acbBotExport: { init: (c: string) => void; compute_moves: (s: string) => string } | null = null;

  try {
    // Standard WASM (non-Go)
    const { instance } = await WebAssembly.instantiate(buffer, {
      env: { memory: new WebAssembly.Memory({ initial: 256 }) },
    });
    acbBotExport = {
      init: (instance.exports.init as (c: string) => void) ?? (() => {}),
      compute_moves: instance.exports.compute_moves as (s: string) => string,
    };
  } catch {
    // Likely a Go WASM – requires wasm_exec.js runtime
    // Check if Go runtime is available
    if (typeof (globalThis as any).Go !== 'undefined') {
      const go = new (globalThis as any).Go();
      const { instance } = await WebAssembly.instantiate(buffer, go.importObject);
      go.run(instance);
      // After go.run, acbBot global should be set
      acbBotExport = (globalThis as any).acbBot;
    } else {
      throw new Error('Go WASM runtime not loaded. Add <script src="/wasm/wasm_exec.js"> to the page.');
    }
  }

  if (!acbBotExport?.compute_moves) {
    throw new Error('WASM module does not export acbBot.compute_moves');
  }

  return (state: VisibleState): Move[] => {
    try {
      const result = acbBotExport!.compute_moves(JSON.stringify(state));
      return JSON.parse(result) as Move[];
    } catch {
      return [];
    }
  };
}

// ────────────────────────────────────────────────────────────────────────────
// Monaco loader (CDN)
// ────────────────────────────────────────────────────────────────────────────

function loadMonaco(): Promise<any> {
  return new Promise((resolve, reject) => {
    if ((globalThis as any).monaco) { resolve((globalThis as any).monaco); return; }

    // Load AMD loader then monaco
    const loaderScript = document.createElement('script');
    loaderScript.src = 'https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs/loader.js';
    loaderScript.onload = () => {
      (globalThis as any).require.config({
        paths: { vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs' },
      });
      (globalThis as any).require(['vs/editor/editor.main'], (monaco: any) => {
        resolve(monaco);
      });
    };
    loaderScript.onerror = reject;
    document.head.appendChild(loaderScript);
  });
}

// ────────────────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────────────────

function formatResult(result: any, replay: any): string {
  const p0Name = replay.players[0]?.name ?? 'Your Bot';
  const p1Name = replay.players[1]?.name ?? 'Opponent';
  const winnerName = result.winner === 0 ? p0Name : result.winner === 1 ? p1Name : 'Draw';
  const winnerClass = result.winner === 0 ? 'win' : result.winner === 1 ? 'loss' : 'draw';
  return `
    <div class="result-banner ${winnerClass}">
      <strong>${result.winner >= 0 ? winnerName + ' wins!' : 'Draw'}</strong>
      <span>${result.reason}</span>
    </div>
    <div class="result-stats">
      <div>${p0Name}: ${result.scores[0]} pts, ${result.bots_alive[0]} bots alive</div>
      <div>${p1Name}: ${result.scores[1]} pts, ${result.bots_alive[1]} bots alive</div>
    </div>
  `;
}

function escapeHtml(s: string): string {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

// ────────────────────────────────────────────────────────────────────────────
// Styles
// ────────────────────────────────────────────────────────────────────────────

const SANDBOX_STYLES = `
<style>
.sandbox-intro { color: var(--text-muted); margin-bottom: 24px; }
.sandbox-layout { display: flex; gap: 20px; }
.sandbox-editor-col { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 16px; }
.sandbox-controls-col { width: 300px; flex-shrink: 0; display: flex; flex-direction: column; gap: 16px; }
.sandbox-panel { background: var(--bg-secondary); border-radius: 8px; padding: 16px; }
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; color: var(--text-primary); }
.panel-actions { display: flex; gap: 8px; }
.settings-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 12px; align-items: center; font-size: 0.875rem; color: var(--text-muted); margin-bottom: 16px; }
.settings-grid select, .settings-grid input { background: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 6px; border-radius: 4px; font-size: 0.875rem; }
.speed-slider { width: 100%; }
.speed-label { color: var(--text-muted); font-size: 0.75rem; }
.run-btn { width: 100%; padding: 12px; font-size: 1rem; }
.wasm-status { font-size: 0.8rem; padding: 8px; border-radius: 4px; margin-top: 8px; }
.wasm-status.hidden { display: none; }
.wasm-status.ok { background: rgba(34,197,94,0.15); color: var(--success); }
.wasm-status.error { background: rgba(239,68,68,0.15); color: var(--error); }
.code-block { background: var(--bg-primary); padding: 12px; border-radius: 6px; font-size: 0.75rem; font-family: monospace; white-space: pre; overflow-x: auto; color: var(--text-muted); }
.code-block.hidden { display: none; }
.match-result .result-banner { padding: 12px 16px; border-radius: 6px; margin-bottom: 10px; display: flex; justify-content: space-between; align-items: center; }
.result-banner.win { background: rgba(34,197,94,0.15); color: var(--success); }
.result-banner.loss { background: rgba(239,68,68,0.15); color: var(--error); }
.result-banner.draw { background: rgba(245,158,11,0.15); color: var(--warning); }
.result-stats { font-size: 0.875rem; color: var(--text-muted); display: flex; flex-direction: column; gap: 4px; }
.perf-stats .perf-row { display: flex; justify-content: space-between; font-size: 0.875rem; color: var(--text-muted); padding: 4px 0; border-bottom: 1px solid var(--bg-tertiary); }
.section-title { font-size: 1.25rem; color: var(--text-primary); margin: 24px 0 16px; }
.replay-section { margin-top: 8px; }
.replay-layout-sandbox { display: flex; gap: 20px; }
.canvas-wrapper { background: var(--bg-secondary); border-radius: 8px; padding: 10px; overflow: auto; flex: 1; }
.sandbox-replay-controls { width: 260px; flex-shrink: 0; background: var(--bg-secondary); border-radius: 8px; padding: 16px; display: flex; flex-direction: column; gap: 12px; }
.playback-controls { display: flex; flex-wrap: wrap; gap: 6px; }
.slider-group label { display: block; color: var(--text-muted); font-size: 0.875rem; margin-bottom: 4px; }
.slider-group input[type=range] { width: 100%; }
.event-log { max-height: 150px; overflow-y: auto; font-size: 0.75rem; font-family: monospace; }
.event-log .event { padding: 3px 0; border-bottom: 1px solid var(--bg-tertiary); }
.no-events { color: var(--text-muted); }
@media (max-width: 900px) {
  .sandbox-layout { flex-direction: column; }
  .sandbox-controls-col { width: 100%; }
  .replay-layout-sandbox { flex-direction: column; }
  .sandbox-replay-controls { width: 100%; }
}
</style>
`;
