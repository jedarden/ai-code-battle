// In-browser WASM game sandbox per §13.1
// Two match engines:
//   - Go WASM engine (production-accurate, loaded from /wasm/engine.wasm)
//   - TypeScript engine (instant, always available as fallback)
// Two user modes:
//   - Quick-start: Monaco editor with JS/TS starter → eval → run match
//   - Full: Upload compiled .wasm file → validate interface → run match
// Multi-opponent: Select 1–3 AI opponents for 2–4 player matches
// Replay viewer: Canvas-based with fog-of-war toggle, view modes
import { runMultiMatch, defaultConfig, BUILTIN_STRATEGIES, type Config, type BotStrategy, type VisibleState, type Move, type Replay } from '../engine';
import { ReplayViewer } from '../replay-viewer';
import type { ViewMode } from '../types';

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

// ─── Console logger ────────────────────────────────────────────────────────

interface ConsoleEntry {
  turn: number;
  level: 'log' | 'warn' | 'error';
  message: string;
}

// ─── Mobile detection ──────────────────────────────────────────────────────

function isMobile(): boolean {
  return /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent)
    || (window.innerWidth < 900 && 'ontouchstart' in window);
}

// ─── Go WASM Engine Loader ─────────────────────────────────────────────────

type GoEngine = {
  loadState(s: string): { ok: boolean; error?: string };
  step(m: string): { state: string; events: string; turn: number; result?: string };
  runMatch(c: string): { replay: string; result: string; error?: string };
  addPlayer(name: string, fn: (stateJSON: string) => string): { ok: boolean; index: number; error?: string };
  clearPlayers(): { ok: boolean };
  runMatchMulti(c: string): { replay: string; result: string; error?: string };
  version: string;
};

let goEngine: GoEngine | null = null;
let goEngineLoading = false;
let goEngineLoadPromise: Promise<GoEngine | null> | null = null;

async function loadGoEngine(): Promise<GoEngine | null> {
  if (goEngine) return goEngine;
  if (goEngineLoading && goEngineLoadPromise) return goEngineLoadPromise;

  goEngineLoading = true;
  goEngineLoadPromise = (async () => {
    try {
      // Load wasm_exec.js if not already loaded
      if (!(globalThis as any).Go) {
        await new Promise<void>((resolve, reject) => {
          const script = document.createElement('script');
          script.src = '/wasm/wasm_exec.js';
          script.onload = () => resolve();
          script.onerror = () => reject(new Error('Failed to load wasm_exec.js'));
          document.head.appendChild(script);
        });
      }

      const go = new (globalThis as any).Go();
      const response = await fetch('/wasm/engine.wasm');
      const buffer = await response.arrayBuffer();
      const { instance } = await WebAssembly.instantiate(buffer, go.importObject);
      go.run(instance);

      goEngine = (globalThis as any).acbEngine as GoEngine;
      return goEngine;
    } catch (err) {
      console.warn('Go WASM engine load failed, using TS engine fallback:', err);
      return null;
    } finally {
      goEngineLoading = false;
    }
  })();

  return goEngineLoadPromise;
}

export function renderSandboxPage(_params: Record<string, string>): void {
  const app = document.getElementById('app');
  if (!app) return;

  if (isMobile()) {
    app.innerHTML = buildMobileHTML();
    return;
  }

  app.innerHTML = buildHTML();
  requestAnimationFrame(() => initSandbox());
}

function buildMobileHTML(): string {
  return `
    <div class="sandbox-page">
      <h1 class="page-title">Bot Sandbox</h1>
      <div class="mobile-notice">
        <div class="mobile-icon">🖥️</div>
        <h2>Desktop Required</h2>
        <p>The Bot Sandbox requires a desktop browser for the code editor and replay viewer.</p>
        <p style="margin-top:16px;color:var(--text-muted);font-size:0.875rem;">
          Scan this page's QR code on your phone to open it on your desktop, or visit
          <strong>aicodebattle.com/#/compete/sandbox</strong> on a computer.
        </p>
        <div style="margin-top:24px;">
          <a href="#/compete" class="btn primary">Back to Compete</a>
        </div>
      </div>
    </div>
    <style>
    .mobile-notice {
      text-align: center;
      padding: 60px 20px;
      background: var(--bg-secondary);
      border-radius: 12px;
      max-width: 400px;
      margin: 40px auto;
    }
    .mobile-icon { font-size: 3rem; margin-bottom: 16px; }
    .mobile-notice h2 { color: var(--text-primary); margin-bottom: 8px; }
    .mobile-notice p { color: var(--text-muted); }
    </style>
  `;
}

function buildHTML(): string {
  return `
    <div class="sandbox-page">
      <h1 class="page-title">Bot Sandbox</h1>
      <p class="sandbox-intro">Write bot logic, pick opponents, and run in-browser matches instantly — no server required.</p>

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
            <div id="monaco-container" style="height:400px;border-radius:6px;overflow:hidden;"></div>
            <div id="wasm-status" class="wasm-status hidden"></div>
          </div>

          <div class="sandbox-panel">
            <div class="panel-header"><span>Console</span>
              <button id="clear-console-btn" class="btn small secondary">Clear</button>
            </div>
            <div id="console-output" class="console-output"></div>
          </div>

          <div class="sandbox-panel collapsible">
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
              <label>Engine</label>
              <select id="engine-select">
                <option value="auto" selected>Auto (Go WASM → TS fallback)</option>
                <option value="wasm">Go WASM Engine</option>
                <option value="ts">TypeScript Engine (instant)</option>
              </select>
              <div id="engine-status" class="engine-status">Engine: loading…</div>
              <div></div>

              <label>Grid Size</label>
              <select id="grid-size-select">
                <option value="20">Small (20x20)</option>
                <option value="30" selected>Medium (30x30)</option>
                <option value="40">Large (40x40)</option>
              </select>

              <label>Max Turns</label>
              <select id="max-turns-select">
                <option value="100">100</option>
                <option value="200" selected>200</option>
                <option value="300">300</option>
                <option value="500">500</option>
              </select>

              <label>Map Seed</label>
              <input type="number" id="seed-input" placeholder="Random" class="seed-input">

              <label>Playback Speed</label>
              <div class="speed-row">
                <input type="range" id="speed-slider" min="20" max="500" value="100" class="speed-slider">
                <span id="speed-label" class="speed-label">100ms</span>
              </div>
            </div>

            <div class="opponents-section">
              <div class="panel-header" style="margin-bottom:8px"><span>Opponents</span>
                <button id="add-opponent-btn" class="btn small secondary">+ Add</button>
              </div>
              <div id="opponents-list"></div>
            </div>

            <button id="run-btn" class="btn primary run-btn">Run Match</button>
          </div>

          <div class="sandbox-panel" id="result-panel" style="display:none">
            <div class="panel-header"><span>Match Result</span></div>
            <div id="match-result" class="match-result"></div>
          </div>

          <div class="sandbox-panel" id="performance-panel" style="display:none">
            <div class="panel-header"><span>Performance</span></div>
            <div id="perf-stats" class="perf-stats"></div>
          </div>
        </div>
      </div>

      <!-- Replay viewer -->
      <div id="replay-section" class="replay-section" style="display:none">
        <div class="replay-header">
          <h2 class="section-title">Replay</h2>
          <div class="replay-controls-top">
            <label class="fog-toggle">
              <input type="checkbox" id="fog-toggle"> Fog of War (Player <select id="fog-player-select"><option value="0">0</option></select>)
            </label>
            <select id="view-mode-select" class="btn small secondary">
              <option value="standard">Standard</option>
              <option value="dots">Dots</option>
              <option value="influence">Influence</option>
              <option value="voronoi">Voronoi</option>
            </select>
            <button id="download-replay-btn" class="btn small secondary">Download Replay</button>
          </div>
        </div>
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

// ─── Opponent slots ────────────────────────────────────────────────────────

const MAX_OPPONENTS = 3;
const STRATEGY_OPTIONS = [
  { value: 'random', label: 'Random' },
  { value: 'gatherer', label: 'Gatherer' },
  { value: 'rusher', label: 'Rusher' },
  { value: 'guardian', label: 'Guardian' },
  { value: 'swarm', label: 'Swarm' },
  { value: 'hunter', label: 'Hunter' },
];

function buildOpponentRow(index: number, defaultStrategy: string): string {
  const opts = STRATEGY_OPTIONS.map(s =>
    `<option value="${s.value}"${s.value === defaultStrategy ? ' selected' : ''}>${s.label}</option>`
  ).join('');
  return `
    <div class="opponent-row" data-idx="${index}">
      <span class="opponent-label">Opponent ${index + 1}</span>
      <select class="opponent-select">${opts}</select>
      <button class="btn small secondary remove-opponent-btn" title="Remove">✕</button>
    </div>`;
}

// ─── Init ──────────────────────────────────────────────────────────────────

function initSandbox(): void {
  let monacoEditor: any = null;
  let currentCode = STARTER_CODE;
  let wasmStrategy: BotStrategy | null = null;
  let lastReplay: any = null;
  let viewer: ReplayViewer | null = null;
  const consoleEntries: ConsoleEntry[] = [];

  const consoleDiv = document.getElementById('console-output')!;
  const opponentsList = document.getElementById('opponents-list')!;
  const engineStatusDiv = document.getElementById('engine-status')!;

  // Start with one opponent (gatherer)
  opponentsList.innerHTML = buildOpponentRow(0, 'gatherer');

  // Start loading Go WASM engine in background
  loadGoEngine().then(engine => {
    if (engine) {
      engineStatusDiv.textContent = `Engine: Go WASM v${engine.version}`;
      engineStatusDiv.style.color = 'var(--success)';
    } else {
      engineStatusDiv.textContent = 'Engine: TypeScript (WASM unavailable)';
      engineStatusDiv.style.color = 'var(--text-muted)';
    }
  });

  function logToConsole(level: ConsoleEntry['level'], message: string, turn = -1): void {
    consoleEntries.push({ turn, level, message });
    if (consoleEntries.length > 500) consoleEntries.shift();
    const colors: Record<string, string> = { log: 'var(--text-muted)', warn: 'var(--warning)', error: 'var(--error)' };
    const prefix = turn >= 0 ? `[T${turn}] ` : '';
    consoleDiv.innerHTML += `<div class="console-line" style="color:${colors[level]}">${prefix}${escapeHtml(message)}</div>`;
    consoleDiv.scrollTop = consoleDiv.scrollHeight;
  }

  logToConsole('log', 'Sandbox ready. Write code or upload a WASM bot, then click Run Match.');

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
    logToConsole('log', `Loading WASM file: ${file.name} (${(file.size / 1024).toFixed(1)} KB)`);

    try {
      wasmStrategy = await loadWasmBot(file);
      status.textContent = `WASM bot loaded: ${file.name}`;
      status.className = 'wasm-status ok';
      logToConsole('log', `WASM bot interface validated: init() + compute_moves() found.`);
      (document.getElementById('run-btn') as HTMLButtonElement).textContent = 'Run Match (WASM)';
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      status.textContent = `Failed: ${msg}`;
      status.className = 'wasm-status error';
      logToConsole('error', `WASM load failed: ${msg}`);
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
    logToConsole('log', 'Code reset to starter template.');
  });

  // ── Console clear ───────────────────────────────────────────────────────
  document.getElementById('clear-console-btn')!.addEventListener('click', () => {
    consoleEntries.length = 0;
    consoleDiv.innerHTML = '';
  });

  // ── Opponent management ─────────────────────────────────────────────────
  function getOpponentCount(): number {
    return opponentsList.querySelectorAll('.opponent-row').length;
  }

  document.getElementById('add-opponent-btn')!.addEventListener('click', () => {
    const count = getOpponentCount();
    if (count >= MAX_OPPONENTS) return;
    const defaults = ['gatherer', 'rusher', 'swarm'];
    opponentsList.insertAdjacentHTML('beforeend', buildOpponentRow(count, defaults[count] || 'random'));
    updateAddButton();
  });

  opponentsList.addEventListener('click', (e) => {
    const btn = (e.target as HTMLElement).closest('.remove-opponent-btn');
    if (!btn) return;
    if (getOpponentCount() <= 1) return;
    const row = btn.closest('.opponent-row')!;
    row.remove();
    opponentsList.querySelectorAll('.opponent-row').forEach((row, i) => {
      row.querySelector('.opponent-label')!.textContent = `Opponent ${i + 1}`;
      (row as HTMLElement).dataset.idx = String(i);
    });
    updateAddButton();
  });

  function updateAddButton(): void {
    const btn = document.getElementById('add-opponent-btn') as HTMLButtonElement;
    btn.disabled = getOpponentCount() >= MAX_OPPONENTS;
    btn.style.opacity = getOpponentCount() >= MAX_OPPONENTS ? '0.5' : '1';
  }
  updateAddButton();

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
    logToConsole('log', 'Starting match…');

    try {
      const gridSize = Number((document.getElementById('grid-size-select') as HTMLSelectElement).value);
      const maxTurns = Number((document.getElementById('max-turns-select') as HTMLSelectElement).value);
      const seedStr = (document.getElementById('seed-input') as HTMLInputElement).value;
      const seed = seedStr ? Number(seedStr) : undefined;
      const enginePref = (document.getElementById('engine-select') as HTMLSelectElement).value;

      const cfg: Config = {
        ...defaultConfig(),
        rows: gridSize,
        cols: gridSize,
        max_turns: maxTurns,
      };

      // Collect strategies: user + opponents
      const userStrategy: BotStrategy = wasmStrategy ?? buildUserStrategy(currentCode, (msg, turn) => logToConsole('error', msg, turn));
      const opponentSelects = opponentsList.querySelectorAll<HTMLSelectElement>('.opponent-select');
      const strategies: (BotStrategy | string)[] = [userStrategy];
      for (const sel of opponentSelects) {
        strategies.push(sel.value);
      }

      const numPlayers = strategies.length;
      logToConsole('log', `Match config: ${gridSize}x${gridSize}, ${maxTurns} turns, ${numPlayers} players${seed !== undefined ? `, seed=${seed}` : ''}`);

      // Determine which engine to use
      const useGoWasm = enginePref === 'wasm' || (enginePref === 'auto' && goEngine !== null);
      let replay: any;
      let result: any;
      let engineUsed: string;

      const t0 = performance.now();

      if (useGoWasm && goEngine) {
        // Use Go WASM engine with JS callback strategies
        logToConsole('log', `Using Go WASM engine v${goEngine.version}`);
        engineUsed = 'Go WASM';

        goEngine.clearPlayers();
        for (let i = 0; i < strategies.length; i++) {
          const strategy = strategies[i];
          const name = i === 0
            ? (wasmStrategy ? 'Uploaded Bot' : 'Your Bot')
            : typeof strategy === 'string' ? strategy : `Bot ${i}`;
          const fn = typeof strategy === 'string'
            ? (stateJSON: string): string => {
                try {
                  const state = JSON.parse(stateJSON) as VisibleState;
                  const builtinFn = BUILTIN_STRATEGIES[strategy];
                  if (!builtinFn) return '[]';
                  const moves = builtinFn(state);
                  return JSON.stringify(moves);
                } catch { return '[]'; }
              }
            : (stateJSON: string): string => {
                try {
                  const state = JSON.parse(stateJSON) as VisibleState;
                  const moves = strategy(state);
                  return JSON.stringify(moves);
                } catch { return '[]'; }
              };
          goEngine.addPlayer(name, fn);
        }

        const configJSON = JSON.stringify({
          config: { ...cfg, cores_per_player: 1 },
          seed: seed ?? 0,
        });

        const matchResult = goEngine.runMatchMulti(configJSON);
        if (matchResult.error) {
          throw new Error(`Go WASM engine error: ${matchResult.error}`);
        }

        replay = JSON.parse(matchResult.replay);
        result = JSON.parse(matchResult.result);
      } else {
        // Use TypeScript engine
        if (enginePref === 'wasm') {
          logToConsole('warn', 'Go WASM engine not available, falling back to TypeScript engine');
        }
        engineUsed = 'TypeScript';
        const matchOutput = runMultiMatch(cfg, strategies, seed);
        replay = matchOutput.replay;
        result = matchOutput.result;
      }

      const elapsed = performance.now() - t0;
      lastReplay = replay;

      logToConsole('log', `Match complete in ${elapsed.toFixed(1)}ms (${engineUsed}) — ${result.turns} turns, winner: ${result.winner >= 0 ? replay.players[result.winner].name : 'Draw'} (${result.reason})`);

      // Show result
      const resultPanel = document.getElementById('result-panel')!;
      resultPanel.style.display = '';
      document.getElementById('match-result')!.innerHTML = formatResult(result, replay, numPlayers);

      // Performance panel
      const perfPanel = document.getElementById('performance-panel')!;
      perfPanel.style.display = '';
      const perfRows = [
        `<div class="perf-row"><span>Engine</span><span>${engineUsed}</span></div>`,
        `<div class="perf-row"><span>Match duration</span><span>${elapsed.toFixed(1)} ms</span></div>`,
        `<div class="perf-row"><span>Turns played</span><span>${result.turns}</span></div>`,
      ];
      for (let i = 0; i < numPlayers; i++) {
        perfRows.push(`<div class="perf-row"><span>${replay.players[i].name}</span><span>${result.scores[i]} pts, ${result.bots_alive[i]} bots</span></div>`);
      }
      document.getElementById('perf-stats')!.innerHTML = perfRows.join('');

      // Show replay
      document.getElementById('replay-section')!.style.display = '';
      initReplayViewer(replay);

    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      logToConsole('error', `Match error: ${msg}`);
      console.error('Sandbox match error:', err);
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

  // ── Replay viewer setup ─────────────────────────────────────────────────
  function initReplayViewer(replay: Replay): void {
    const canvas = document.getElementById('sandbox-canvas') as HTMLCanvasElement;
    const speed = Number((document.getElementById('speed-slider') as HTMLInputElement).value);

    if (viewer) viewer.destroy();
    viewer = new ReplayViewer(canvas, { cellSize: 12 });
    viewer.loadReplay(replay as any);
    viewer.setSpeed(speed);

    const fogPlayerSelect = document.getElementById('fog-player-select') as HTMLSelectElement;
    fogPlayerSelect.innerHTML = replay.players.map((p, i) =>
      `<option value="${i}">${i} (${p.name})</option>`
    ).join('');

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
        : events.map(ev => `<div class="event"><span style="color:#fbbf24">${ev.type.replace(/_/g, ' ')}</span></div>`).join('');
    };

    document.getElementById('sb-play-btn')!.onclick = () => viewer!.togglePlay();
    document.getElementById('sb-prev-btn')!.onclick = () => { viewer!.setTurn(viewer!.getTurn() - 1); };
    document.getElementById('sb-next-btn')!.onclick = () => { viewer!.setTurn(viewer!.getTurn() + 1); };
    document.getElementById('sb-reset-btn')!.onclick = () => { viewer!.pause(); viewer!.setTurn(0); };
    slider.oninput = () => viewer!.setTurn(parseInt(slider.value, 10));

    (document.getElementById('fog-toggle') as HTMLInputElement).onchange = (e) => {
      const checked = (e.target as HTMLInputElement).checked;
      if (checked) {
        const pid = parseInt(fogPlayerSelect.value);
        viewer!.setFogOfWar(pid);
      } else {
        viewer!.setFogOfWar(null);
      }
    };
    fogPlayerSelect.onchange = () => {
      if ((document.getElementById('fog-toggle') as HTMLInputElement).checked) {
        viewer!.setFogOfWar(parseInt(fogPlayerSelect.value));
      }
    };

    (document.getElementById('view-mode-select') as HTMLSelectElement).onchange = (e) => {
      viewer!.setViewMode((e.target as HTMLSelectElement).value as ViewMode);
    };
  }
}

// ────────────────────────────────────────────────────────────────────────────
// User strategy builder (sandboxed eval with error reporting)
// ────────────────────────────────────────────────────────────────────────────

function buildUserStrategy(code: string, onError: (msg: string, turn: number) => void): BotStrategy {
  let turnCounter = 0;
  return (state: VisibleState): Move[] => {
    turnCounter++;
    try {
      const fn = new Function('state', `
        ${code}
        if (typeof computeMoves !== 'function') {
          throw new Error('computeMoves function not found — define a computeMoves(state) function');
        }
        return computeMoves(state);
      `);
      const result = fn(state);
      if (!Array.isArray(result)) {
        onError(`computeMoves returned ${typeof result} instead of array`, turnCounter);
        return [];
      }
      return result as Move[];
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      onError(`Turn ${turnCounter}: ${msg}`, turnCounter);
      return [];
    }
  };
}

// ────────────────────────────────────────────────────────────────────────────
// WASM bot loader (supports both standard WASM and Go WASM)
// ────────────────────────────────────────────────────────────────────────────

async function loadWasmBot(file: File): Promise<BotStrategy> {
  const buffer = await file.arrayBuffer();
  let acbBotExport: { init: (c: string) => void; compute_moves: (s: string) => string } | null = null;

  try {
    // Standard WASM (Rust, C, AssemblyScript) — exports compute_moves directly
    const { instance } = await WebAssembly.instantiate(buffer, {
      env: { memory: new WebAssembly.Memory({ initial: 256 }) },
    });
    const computeFn = instance.exports.compute_moves as ((ptr: number, len: number) => number) | undefined;

    if (computeFn) {
      const memory = instance.exports.memory as WebAssembly.Memory | undefined;
      if (memory) {
        acbBotExport = createPointerBasedBridge(instance, memory);
      }
    }

    if (!acbBotExport) {
      throw new Error('WASM module does not export acbBot.compute_moves or compatible functions');
    }
  } catch (primaryErr) {
    // Likely a Go WASM — requires wasm_exec.js runtime
    if (typeof (globalThis as any).Go !== 'undefined') {
      const go = new (globalThis as any).Go();
      const { instance } = await WebAssembly.instantiate(buffer, go.importObject);
      go.run(instance);
      acbBotExport = (globalThis as any).acbBot;
      if (!acbBotExport) {
        throw new Error('Go WASM ran but did not set global acbBot — check your bot builds with cmd/acb-wasm/botmain template');
      }
    } else {
      throw new Error('Not a standard WASM bot and Go WASM runtime (wasm_exec.js) not loaded.');
    }
  }

  if (!acbBotExport?.compute_moves) {
    throw new Error('WASM module does not export a valid acbBot.compute_moves function');
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

function createPointerBasedBridge(
  instance: WebAssembly.Instance,
  memory: WebAssembly.Memory,
): { init: (c: string) => void; compute_moves: (s: string) => string } {
  const encoder = new TextEncoder();
  const decoder = new TextDecoder();

  function writeString(str: string): [number, number] {
    const bytes = encoder.encode(str);
    const ptr = (instance.exports.allocate as (len: number) => number)?.(bytes.length)
      ?? (instance.exports.malloc as (len: number) => number)?.(bytes.length);
    if (!ptr) throw new Error('WASM does not export allocate or malloc');
    const buf = new Uint8Array(memory.buffer);
    buf.set(bytes, ptr);
    return [ptr, bytes.length];
  }

  function readString(ptr: number): string {
    const buf = new Uint8Array(memory.buffer);
    let end = ptr;
    while (end < buf.length && buf[end] !== 0) end++;
    return decoder.decode(buf.slice(ptr, end));
  }

  return {
    init: (configJSON: string) => {
      const [ptr, len] = writeString(configJSON);
      (instance.exports.init as (p: number, l: number) => void)(ptr, len);
    },
    compute_moves: (stateJSON: string): string => {
      const [ptr, len] = writeString(stateJSON);
      const resultPtr = (instance.exports.compute_moves as (p: number, l: number) => number)(ptr, len);
      const result = readString(resultPtr);
      (instance.exports.free_result as (p: number) => void)?.(resultPtr);
      return result;
    },
  };
}

// ────────────────────────────────────────────────────────────────────────────
// Monaco loader (CDN, ~4MB lazy-loaded)
// ────────────────────────────────────────────────────────────────────────────

function loadMonaco(): Promise<any> {
  return new Promise((resolve, reject) => {
    if ((globalThis as any).monaco) { resolve((globalThis as any).monaco); return; }

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

function formatResult(result: any, replay: any, _numPlayers: number): string {
  const winnerName = result.winner >= 0 ? replay.players[result.winner].name : 'Draw';
  const winnerClass = result.winner === 0 ? 'win' : result.winner >= 0 ? 'loss' : 'draw';
  const scoreRows = replay.players.map((p: any, i: number) =>
    `<div class="result-player"><span class="player-dot" style="background:${PLAYER_COLORS[i % PLAYER_COLORS.length]}"></span>${p.name}: ${result.scores[i]} pts, ${result.bots_alive[i]} bots alive</div>`
  ).join('');
  return `
    <div class="result-banner ${winnerClass}">
      <strong>${result.winner >= 0 ? winnerName + ' wins!' : 'Draw'}</strong>
      <span>${result.reason}</span>
    </div>
    <div class="result-stats">${scoreRows}</div>
  `;
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

const PLAYER_COLORS = ['#3b82f6', '#ef4444', '#22c55e', '#f59e0b'];

// ────────────────────────────────────────────────────────────────────────────
// Styles
// ────────────────────────────────────────────────────────────────────────────

const SANDBOX_STYLES = `
<style>
.sandbox-intro { color: var(--text-muted); margin-bottom: 24px; }
.sandbox-layout { display: flex; gap: 20px; }
.sandbox-editor-col { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 16px; }
.sandbox-controls-col { width: 320px; flex-shrink: 0; display: flex; flex-direction: column; gap: 16px; }
.sandbox-panel { background: var(--bg-secondary); border-radius: 8px; padding: 16px; }
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; color: var(--text-primary); }
.panel-actions { display: flex; gap: 8px; }
.settings-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 12px; align-items: center; font-size: 0.875rem; color: var(--text-muted); margin-bottom: 16px; }
.settings-grid select, .settings-grid input { background: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 6px; border-radius: 4px; font-size: 0.875rem; }
.engine-status { font-size: 0.75rem; color: var(--text-muted); padding: 2px 0; }
.seed-input { width: 100%; }
.speed-row { display: flex; align-items: center; gap: 8px; }
.speed-slider { flex: 1; }
.speed-label { color: var(--text-muted); font-size: 0.75rem; min-width: 40px; }
.opponents-section { margin-bottom: 16px; }
.opponent-row { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.opponent-label { font-size: 0.8rem; color: var(--text-muted); min-width: 80px; }
.opponent-select { flex: 1; background: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 4px 8px; border-radius: 4px; font-size: 0.8rem; }
.run-btn { width: 100%; padding: 12px; font-size: 1rem; }
.wasm-status { font-size: 0.8rem; padding: 8px; border-radius: 4px; margin-top: 8px; }
.wasm-status.hidden { display: none; }
.wasm-status.ok { background: rgba(34,197,94,0.15); color: var(--success); }
.wasm-status.error { background: rgba(239,68,68,0.15); color: var(--error); }
.code-block { background: var(--bg-primary); padding: 12px; border-radius: 6px; font-size: 0.75rem; font-family: monospace; white-space: pre; overflow-x: auto; color: var(--text-muted); max-height: 300px; overflow-y: auto; }
.code-block.hidden { display: none; }
.console-output { background: #0d1117; border-radius: 6px; padding: 8px 12px; font-family: 'Menlo', 'Monaco', 'Courier New', monospace; font-size: 0.75rem; line-height: 1.6; max-height: 160px; overflow-y: auto; min-height: 60px; }
.console-line { padding: 1px 0; word-break: break-all; }
.match-result .result-banner { padding: 12px 16px; border-radius: 6px; margin-bottom: 10px; display: flex; justify-content: space-between; align-items: center; }
.result-banner.win { background: rgba(34,197,94,0.15); color: var(--success); }
.result-banner.loss { background: rgba(239,68,68,0.15); color: var(--error); }
.result-banner.draw { background: rgba(245,158,11,0.15); color: var(--warning); }
.result-stats { font-size: 0.875rem; color: var(--text-muted); display: flex; flex-direction: column; gap: 4px; }
.result-player { display: flex; align-items: center; gap: 8px; }
.player-dot { width: 10px; height: 10px; border-radius: 50%; display: inline-block; }
.perf-stats .perf-row { display: flex; justify-content: space-between; font-size: 0.875rem; color: var(--text-muted); padding: 4px 0; border-bottom: 1px solid var(--bg-tertiary); }
.replay-header { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; }
.section-title { font-size: 1.25rem; color: var(--text-primary); margin: 24px 0 16px; }
.replay-section { margin-top: 8px; }
.replay-controls-top { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-top: 16px; }
.fog-toggle { display: flex; align-items: center; gap: 6px; font-size: 0.8rem; color: var(--text-muted); cursor: pointer; }
.fog-toggle select { background: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 2px 4px; border-radius: 3px; font-size: 0.75rem; }
.replay-layout-sandbox { display: flex; gap: 20px; margin-top: 16px; }
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
  .replay-header { flex-direction: column; align-items: flex-start; }
}
</style>
`;
