// Standalone replay viewer page - lazy loaded from app.ts
import type { Replay, GameEvent, DebugInfo, Position, ViewMode } from '../types';
import { fetchCommentary } from '../api-types';
import {
  AnnotationOverlay,
  createAnnotationForm,
  fetchFeedback,
  loadLocalAnnotations,
  ANNOTATION_OVERLAY_STYLES,
  type Annotation,
} from '../components/annotation';
import {
  EventTimeline,
  EVENT_TIMELINE_STYLES,
} from '../components/event-timeline';
import {
  computeAllDensities,
  computeSpeedSchedule,
  createDirectorState,
  tickDirectorSpeed,
  loadDirectorConfig,
  saveDirectorConfig,
  formatDirectorLabel,
  type DirectorConfig,
  type DirectorState,
  type DurationPreset,
} from '../components/director';
import { THEATER_STYLES, TheaterMode } from '../components/theater';

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
          <div class="canvas-wrapper" style="position:relative">
            <canvas id="replay-canvas" style="touch-action:none"></canvas>
            <div id="no-replay" class="no-replay-message">Load a replay file to view</div>
            <button id="theater-btn" class="theater-btn" aria-label="Toggle theater mode" title="Theater mode (F)" style="position:absolute;top:8px;right:8px">&#x26F6;</button>
          </div>

          <!-- Mobile compact controls bar — CSS hides on tablet+ -->
          <div class="mobile-replay-controls" id="mobile-controls">
            <div class="mobile-playback-bar">
              <button id="mobile-reset-btn" class="btn small" aria-label="Reset to start" disabled>&#9612;&#9612;</button>
              <button id="mobile-prev-btn" class="btn small" aria-label="Previous turn" disabled>&#9664;</button>
              <button id="mobile-play-btn" class="btn small primary" aria-label="Play or pause" disabled>&#9654;</button>
              <button id="mobile-next-btn" class="btn small" aria-label="Next turn" disabled>&#9654;&#9654;</button>
              <span id="mobile-turn-info" class="mobile-speed-display">T: 0/0</span>
              <button id="mobile-speed-btn" class="btn small secondary" aria-label="Cycle playback speed">100ms</button>
            </div>
            <input type="range" id="mobile-turn-slider" min="0" max="0" value="0"
              style="width:100%;margin-top:4px" disabled aria-label="Turn scrubber">
          </div>

          <!-- Mobile event timeline ribbon — CSS hides on tablet+ -->
          <div class="mobile-event-timeline" id="mobile-timeline" aria-label="Event timeline">
            <span style="color:var(--text-muted);font-size:0.75rem;padding:4px 8px">Load a replay</span>
          </div>

          <!-- Desktop event timeline with annotation badges (hidden on mobile) -->
          <div class="event-timeline-container" id="event-timeline-container" style="display:none"></div>

          <div id="win-prob-section" class="win-prob-section" style="display:none">
            <div class="win-prob-header">
              <span class="win-prob-title">Win Probability</span>
              <div class="critical-moment-nav">
                <button id="prev-critical-btn" class="btn" title="Previous critical moment ([)" disabled>&#9664; Prev</button>
                <span id="critical-moment-info" class="critical-moment-info">&#8212;</span>
                <button id="next-critical-btn" class="btn" title="Next critical moment (])" disabled>Next &#9654;</button>
              </div>
            </div>
            <div id="win-prob-container" class="win-prob-container"></div>
            <div id="win-prob-legend" class="win-prob-legend"></div>
          </div>
        </div>

        <div id="commentary-bar" class="commentary-bar" style="display:none">
          <div class="commentary-content">
            <span id="commentary-text" class="commentary-text"></span>
          </div>
          <button id="commentary-toggle" class="btn small secondary" title="Toggle AI commentary">💬</button>
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
            <div class="speed-selector-group">
              <label for="speed-select">Speed Preset:</label>
              <select id="speed-select">
                <option value="500">1x</option>
                <option value="250">2x</option>
                <option value="125">4x</option>
                <option value="62">8x</option>
                <option value="31">16x</option>
                <option value="director">Director</option>
              </select>
            </div>
            <div id="director-options" class="director-options" style="display:none">
              <div class="slider-group">
                <label>Target Duration:</label>
                <div class="duration-presets" id="duration-presets">
                  <button class="btn small secondary duration-btn" data-duration="30">30s</button>
                  <button class="btn small secondary duration-btn active" data-duration="60">1min</button>
                  <button class="btn small secondary duration-btn" data-duration="120">2min</button>
                  <button class="btn small secondary duration-btn" data-duration="300">5min</button>
                </div>
              </div>
              <div class="director-status" id="director-status">Director 16x</div>
            </div>
          </div>

          <div class="panel">
            <h2>View Options</h2>
            <div class="view-options">
              <label for="view-mode-select">View Mode:</label>
              <select id="view-mode-select">
                <option value="standard">Standard</option>
                <option value="dots">Dots</option>
                <option value="voronoi">Territory (Voronoi)</option>
                <option value="influence">Influence Gradient</option>
              </select>
              <label for="fog-select" style="margin-top: 10px;">Fog of War:</label>
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
              <label for="follow-zoom-select" style="margin-top: 10px;">Follow Zoom:</label>
              <select id="follow-zoom-select">
                <option value="2">2x</option>
                <option value="3" selected>3x</option>
                <option value="4">4x</option>
                <option value="5">5x</option>
                <option value="6">6x</option>
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

          <div class="panel annotation-panel" id="annotation-panel">
            <h2>Annotations</h2>
            <div id="annotation-overlay-container"></div>
            <div id="annotation-form-container"></div>
          </div>

          <div class="keyboard-shortcuts">
            <kbd>Space</kbd> Play/Pause
            <kbd>←</kbd><kbd>→</kbd> Step
            <kbd>[</kbd><kbd>]</kbd> Prev/Next Critical
            <kbd>Home</kbd><kbd>End</kbd> First/Last
            <kbd>1</kbd>-<kbd>6</kbd> Follow Bot
            <kbd>0</kbd>/<kbd>Esc</kbd> Exit Follow
            <kbd>F</kbd> Theater Mode
          </div>
        </div>
      </div>
    </div>

    <!-- Floating view mode toggle — CSS hides on tablet+desktop -->
    <button id="mobile-view-mode-btn" class="mobile-view-mode-toggle"
      aria-label="Switch view mode" title="Switch view mode">&#128065;</button>

    <!-- Debug telemetry bottom sheet (mobile) / sidebar panel (desktop) -->
    <div id="debug-panel" class="panel debug-panel" style="display:none">
      <div class="debug-panel-header" id="debug-panel-toggle-btn" role="button" tabindex="0"
           aria-expanded="false" aria-controls="debug-panel-body">
        <h2 style="margin:0">Debug Telemetry</h2>
        <span id="debug-panel-chevron" class="debug-chevron" aria-hidden="true">▾</span>
      </div>
      <div id="debug-panel-body" class="debug-panel-body">
        <div id="debug-player-toggles" class="debug-player-toggles"></div>
        <div id="debug-info-display" class="debug-info-display">
          <div class="no-debug-data">No debug data for this turn</div>
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
      .speed-selector-group { margin-bottom: 10px; }
      .speed-selector-group label { display: block; color: var(--text-muted); font-size: 0.875rem; margin-bottom: 6px; }
      .speed-selector-group select { width: 100%; background-color: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 8px; border-radius: 6px; font-size: 14px; }
      .director-options { margin-top: 10px; padding-top: 10px; border-top: 1px solid var(--bg-tertiary); }
      .duration-presets { display: flex; gap: 6px; flex-wrap: wrap; }
      .duration-presets .btn { flex: 1; min-width: 40px; text-align: center; font-size: 0.75rem; }
      .duration-presets .btn.active { background-color: var(--accent); color: white; }
      .director-status { text-align: center; color: var(--accent); font-size: 0.8rem; font-weight: 600; padding: 6px 0; font-family: monospace; }
      .win-prob-section { background-color: var(--bg-secondary); border-radius: 8px; padding: 12px; margin-top: 10px; }
      .win-prob-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; flex-wrap: wrap; gap: 8px; }
      .win-prob-title { color: var(--text-muted); font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; font-weight: 600; }
      .critical-moment-nav { display: flex; align-items: center; gap: 8px; }
      .critical-moment-nav .btn { padding: 4px 10px; font-size: 0.75rem; }
      .critical-moment-nav .btn:disabled { opacity: 0.4; cursor: not-allowed; }
      .critical-moment-info { color: var(--text-muted); font-size: 0.8rem; max-width: 280px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .win-prob-container { width: 100%; overflow: hidden; border-radius: 4px; }
      .win-prob-legend { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 6px; font-size: 0.75rem; font-family: monospace; }
      .commentary-bar { background-color: var(--bg-secondary); border-radius: 8px; padding: 8px 12px; margin-top: 10px; display: flex; align-items: center; gap: 10px; min-height: 40px; }
      .commentary-content { flex: 1; min-width: 0; }
      .commentary-text { color: var(--text-secondary); font-size: 0.875rem; line-height: 1.4; display: block; }
      .commentary-text.type-setup { color: #94a3b8; }
      .commentary-text.type-action { color: #e2e8f0; }
      .commentary-text.type-reaction { color: #fbbf24; }
      .commentary-text.type-climax { color: #f97316; font-weight: 600; }
      .commentary-text.type-denouement { color: #94a3b8; font-style: italic; }
      .commentary-toggle { flex-shrink: 0; opacity: 0.6; transition: opacity 0.2s; }
      .commentary-toggle.active { opacity: 1; background-color: var(--accent); color: white; }
      @media (max-width: 900px) {
        .replay-layout { flex-direction: column; }
        .replay-sidebar { width: 100%; }
      }
      /* Debug telemetry panel */
      .debug-panel { padding: 0; overflow: hidden; }
      .debug-panel-header { display: flex; align-items: center; justify-content: space-between; padding: 15px; cursor: pointer; user-select: none; }
      .debug-panel-header:hover { background-color: var(--bg-tertiary); }
      .debug-panel-header h2 { font-size: 1rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }
      .debug-chevron { color: var(--text-muted); font-size: 1rem; transition: transform 0.2s; }
      .debug-panel.expanded .debug-chevron { transform: rotate(180deg); }
      .debug-panel-body { display: none; padding: 0 15px 15px; }
      .debug-panel.expanded .debug-panel-body { display: block; }
      .debug-player-toggles { display: flex; flex-direction: column; gap: 6px; margin-bottom: 12px; }
      .debug-player-toggle { display: flex; align-items: center; gap: 8px; cursor: pointer; color: var(--text-muted); font-size: 0.875rem; }
      .debug-player-toggle input[type="checkbox"] { width: 16px; height: 16px; accent-color: var(--accent); cursor: pointer; }
      .debug-player-dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
      .debug-info-display { display: flex; flex-direction: column; gap: 10px; }
      .debug-player-info { background-color: var(--bg-tertiary); border-radius: 6px; padding: 10px; }
      .debug-player-name { font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase; margin-bottom: 6px; font-weight: 600; }
      .debug-reasoning { color: var(--text-secondary); font-size: 0.8rem; line-height: 1.5; margin-bottom: 8px; }
      .debug-targets { display: flex; flex-direction: column; gap: 4px; }
      .debug-target-item { font-size: 0.75rem; font-family: monospace; color: var(--text-muted); display: flex; align-items: center; gap: 6px; }
      .debug-target-priority { opacity: 0.7; }
      .no-debug-data { color: var(--text-muted); font-size: 0.8rem; font-style: italic; }
      /* Mobile: bottom sheet */
      @media (max-width: 900px) {
        .debug-panel {
          position: fixed !important;
          bottom: 0;
          left: 0;
          right: 0;
          z-index: 200;
          border-radius: 12px 12px 0 0 !important;
          max-height: 70vh;
          overflow-y: auto;
          transform: translateY(calc(100% - 52px));
          transition: transform 0.3s ease;
          box-shadow: 0 -4px 24px rgba(0, 0, 0, 0.4);
        }
        .debug-panel.expanded {
          transform: translateY(0);
        }
        .debug-panel-header::before {
          content: '';
          display: block;
          width: 36px;
          height: 4px;
          background: var(--border, #374151);
          border-radius: 2px;
          position: absolute;
          top: 8px;
          left: 50%;
          transform: translateX(-50%);
        }
        .debug-panel-header { position: relative; padding-top: 20px; }
      }
      /* Annotation panel */
      .annotation-panel { padding: 0; overflow: hidden; }
      .annotation-panel > h2 { padding: 15px 15px 0; }
      #annotation-overlay-container { padding: 0 15px; }
      #annotation-form-container { padding: 0 15px 15px; border-top: 1px solid var(--bg-tertiary); margin-top: 8px; padding-top: 10px; }
      .annotation-canvas-hint { position: absolute; bottom: 8px; left: 8px; background: rgba(0,0,0,0.7); color: #94a3b8; font-size: 0.65rem; padding: 3px 8px; border-radius: 4px; pointer-events: none; opacity: 0; transition: opacity 0.3s; }
      .annotation-canvas-hint.visible { opacity: 1; }
    </style>
    <style>${ANNOTATION_OVERLAY_STYLES}</style>
    <style>${EVENT_TIMELINE_STYLES}</style>
    <style>${THEATER_STYLES}</style>
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
  const winProbLegend = document.getElementById('win-prob-legend') as HTMLDivElement;
  const prevCriticalBtn = document.getElementById('prev-critical-btn') as HTMLButtonElement;
  const nextCriticalBtn = document.getElementById('next-critical-btn') as HTMLButtonElement;
  const criticalMomentInfo = document.getElementById('critical-moment-info') as HTMLSpanElement;
  const commentaryBar = document.getElementById('commentary-bar') as HTMLDivElement;
  const commentaryText = document.getElementById('commentary-text') as HTMLSpanElement;
  const commentaryToggle = document.getElementById('commentary-toggle') as HTMLButtonElement;
  const debugPanel = document.getElementById('debug-panel') as HTMLDivElement;
  const debugPanelToggleBtn = document.getElementById('debug-panel-toggle-btn') as HTMLDivElement;
  const debugPlayerToggles = document.getElementById('debug-player-toggles') as HTMLDivElement;
  const debugInfoDisplay = document.getElementById('debug-info-display') as HTMLDivElement;

  // Mobile controls
  const mobilePlayBtn = document.getElementById('mobile-play-btn') as HTMLButtonElement;
  const mobilePrevBtn = document.getElementById('mobile-prev-btn') as HTMLButtonElement;
  const mobileNextBtn = document.getElementById('mobile-next-btn') as HTMLButtonElement;
  const mobileResetBtn = document.getElementById('mobile-reset-btn') as HTMLButtonElement;
  const mobileTurnInfo = document.getElementById('mobile-turn-info') as HTMLSpanElement;
  const mobileTurnSlider = document.getElementById('mobile-turn-slider') as HTMLInputElement;
  const mobileSpeedBtn = document.getElementById('mobile-speed-btn') as HTMLButtonElement;
  const mobileTimeline = document.getElementById('mobile-timeline') as HTMLDivElement;
  const viewModeSelect = document.getElementById('view-mode-select') as HTMLSelectElement;
  const mobileViewModeBtn = document.getElementById('mobile-view-mode-btn') as HTMLButtonElement;

  let viewer = new ReplayViewerClass(canvas, { cellSize: 10 });
  let criticalMoments: Array<{turn: number; delta: number; description: string}> = [];
  let commentaryEnabled = true;
  let debugPanelExpanded = false;

  // Theater mode
  const theaterBtn = document.getElementById('theater-btn') as HTMLButtonElement;
  const theater = new TheaterMode(canvas, {
    getScoreText: () => {
      const replay = viewer.getReplay() as Replay | null;
      if (!replay) return '';
      return replay.players.map((p: any, i: number) => `${p.name}: ${replay.result.scores?.[i] ?? 0}`).join('  ');
    },
    getPlayerColors: () => {
      const replay = viewer.getReplay() as Replay | null;
      if (!replay) return [];
      const palettes = ['#332288', '#88ccee', '#44aa99', '#117733', '#999933', '#ddcc77'];
      return replay.players.map((_: any, i: number) => palettes[i] ?? '#888888');
    },
    getWinProb: () => {
      const replay = viewer.getReplay() as Replay | null;
      if (!replay?.win_prob) return [];
      const turn = viewer.getTurn();
      return replay.win_prob[turn] ?? [];
    },
    getTurn: () => viewer.getTurn(),
    getTotalTurns: () => viewer.getTotalTurns(),
    getIsPlaying: () => viewer.getIsPlaying(),
    getSpeed: () => {
      const el = document.getElementById('speed-slider') as HTMLInputElement;
      return el ? parseInt(el.value, 10) : 100;
    },
    togglePlay: () => viewer.togglePlay(),
    setTurn: (t: number) => { viewer.setTurn(t); updateUI(); updateEventLog(); },
    exitTheater: () => {},
    onCriticalMoment: () => theater.pulseVignette(),
  });
  theaterBtn.addEventListener('click', () => theater.toggle());

  // Director mode state
  let directorState: DirectorState = createDirectorState();
  let directorConfig: DirectorConfig = loadDirectorConfig();
  let directorSchedule: ReturnType<typeof computeSpeedSchedule> = [];
  let directorAnimFrame: number | null = null;

  // Director UI elements
  const speedSelect = document.getElementById('speed-select') as HTMLSelectElement;
  const directorOptions = document.getElementById('director-options') as HTMLDivElement;
  const directorStatus = document.getElementById('director-status') as HTMLDivElement;
  const durationPresets = document.getElementById('duration-presets') as HTMLDivElement;

  // Mobile speed cycling
  const SPEED_STEPS = [1000, 500, 200, 100, 50, 20];
  let mobileSpeedIdx = 3; // default 100ms

  // View mode cycling
  const VIEW_MODES: Array<'standard' | 'dots' | 'voronoi' | 'influence'> = ['standard', 'dots', 'voronoi', 'influence'];
  const VIEW_MODE_ICONS: Record<string, string> = { standard: '\u{1F5FA}', dots: '··', voronoi: '⬡', influence: '◎' };

  // Pinch-to-zoom pointer state
  const activePointers = new Map<number, PointerEvent>();
  let pinchStartDist = 0;
  let pinchStartCellSize = 10;

  function enableControls(): void {
    playBtn.disabled = false;
    prevBtn.disabled = false;
    nextBtn.disabled = false;
    resetBtn.disabled = false;
    turnSlider.disabled = false;
    noReplayDiv.style.display = 'none';
  }

  function enableMobileControls(): void {
    mobilePlayBtn.disabled = false;
    mobilePrevBtn.disabled = false;
    mobileNextBtn.disabled = false;
    mobileResetBtn.disabled = false;
    mobileTurnSlider.disabled = false;
  }

  function updateMobileUI(): void {
    const turn = viewer.getTurn();
    const total = viewer.getTotalTurns();
    mobileTurnInfo.textContent = `T: ${turn}/${total - 1}`;
    mobileTurnSlider.value = String(turn);
    mobilePlayBtn.textContent = viewer.getIsPlaying() ? '⏸' : '▶';
  }

  function buildMobileTimeline(replay: Replay): void {
    const eventTurns: number[] = [];
    replay.turns.forEach((t: any, i: number) => {
      if (t.events && t.events.length > 0) eventTurns.push(i);
    });

    if (eventTurns.length === 0) {
      mobileTimeline.innerHTML = '<span style="color:var(--text-muted);font-size:0.75rem;padding:4px 8px">No events</span>';
      return;
    }

    const currentTurn = viewer.getTurn();
    mobileTimeline.innerHTML = eventTurns.map(turn => {
      const active = turn === currentTurn ? ' active' : '';
      return `<button class="mobile-event-dot${active}" data-turn="${turn}" aria-label="Turn ${turn}"><span style="font-size:0.65rem">${turn}</span></button>`;
    }).join('');

    mobileTimeline.querySelectorAll<HTMLElement>('.mobile-event-dot').forEach(dot => {
      dot.addEventListener('click', () => {
        const t = parseInt(dot.dataset.turn!, 10);
        viewer.setTurn(t);
        updateUI();
        updateEventLog();
        updateMobileUI();
        updateMobileTimeline();
      });
    });
  }

  function updateMobileTimeline(): void {
    const currentTurn = viewer.getTurn();
    mobileTimeline.querySelectorAll<HTMLElement>('.mobile-event-dot').forEach(dot => {
      const t = parseInt(dot.dataset.turn!, 10);
      dot.classList.toggle('active', t === currentTurn);
    });
    const activeDot = mobileTimeline.querySelector<HTMLElement>('.mobile-event-dot.active');
    if (activeDot) {
      activeDot.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' });
    }
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
    enableMobileControls();
    updateMatchInfo(replay);
    turnSlider.max = String(viewer.getTotalTurns() - 1);
    mobileTurnSlider.max = String(viewer.getTotalTurns() - 1);
    updateUI();
    updateEventLog();
    updateMobileUI();
    buildMobileTimeline(replay);
    initWinProb(replay);
    initDirector(replay);
    loadCommentary(replay.match_id);
    initDebugPanel(replay);
    initAnnotations(replay);

    const hasAnyDebug = replay.turns.some(t => t.debug && Object.keys(t.debug).length > 0);
    if (hasAnyDebug) {
      debugPanel.style.display = '';
      // On mobile, default collapsed; on desktop, default expanded
      if (window.innerWidth > 900) {
        debugPanelExpanded = true;
        debugPanel.classList.add('expanded');
        debugPanelToggleBtn.setAttribute('aria-expanded', 'true');
      } else {
        debugPanelExpanded = false;
        debugPanel.classList.remove('expanded');
        debugPanelToggleBtn.setAttribute('aria-expanded', 'false');
      }
    } else {
      debugPanel.style.display = 'none';
    }
  }

  async function loadCommentary(matchId: string): Promise<void> {
    const commentary = await fetchCommentary(matchId);
    if (commentary && commentary.entries.length > 0) {
      viewer.setCommentary(commentary);
      commentaryBar.style.display = 'flex';
      commentaryToggle.classList.add('active');
      commentaryEnabled = true;
    } else {
      viewer.setCommentary(null);
      commentaryBar.style.display = 'none';
    }
  }

  function initDebugPanel(replay: Replay): void {
    const playerColors = [
      '#332288', '#88ccee', '#44aa99', '#117733', '#999933', '#ddcc77',
    ];

    debugPlayerToggles.innerHTML = '';
    replay.players.forEach((player, idx) => {
      const color = playerColors[idx] || '#888';
      const label = document.createElement('label');
      label.className = 'debug-player-toggle';
      label.innerHTML = `
        <input type="checkbox" id="debug-toggle-p${idx}" checked>
        <span class="debug-player-dot" style="background:${color}"></span>
        ${player.name}
      `;
      const checkbox = label.querySelector('input') as HTMLInputElement;
      checkbox.addEventListener('change', () => {
        viewer.setShowDebug(true);
        viewer.setDebugPlayerEnabled(idx, checkbox.checked);
        updateDebugDisplay(viewer.getDebugForCurrentTurn?.() ?? null);
      });
      debugPlayerToggles.appendChild(label);
    });

    viewer.setShowDebug(true);
  }

  // ── Annotation overlay integration (§16.8) ──────────────────────────────────────

  let annotationOverlay: AnnotationOverlay | null = null;
  let eventTimeline: EventTimeline | null = null;
  let allAnnotations: Annotation[] = [];
  let clickedGridPosition: Position | undefined;
  let canvasAnnotationHint: HTMLDivElement | null = null;

  function syncAnnotationsToViewer(): void {
    // Push annotations to the canvas renderer for marker drawing
    viewer.setAnnotations(allAnnotations);
    // Push annotations to the event timeline for badge rendering
    eventTimeline?.setAnnotations(allAnnotations);
  }

  function initAnnotations(replay: Replay): void {
    const overlayContainer = document.getElementById('annotation-overlay-container');
    const formContainer = document.getElementById('annotation-form-container');
    if (!overlayContainer || !formContainer) return;

    // Initialize EventTimeline (desktop)
    const timelineContainer = document.getElementById('event-timeline-container');
    if (timelineContainer) {
      eventTimeline = new EventTimeline(timelineContainer, {
        onTurnClick: (turn: number) => {
          viewer.setTurn(turn);
          updateUI();
          updateEventLog();
        },
      });
      // Extract events from replay turns and feed to timeline
      const timelineTurns = replay.turns.map((t: any, i: number) => ({
        turn: i,
        events: t.events ?? [],
      }));
      eventTimeline.setEvents(timelineTurns);
      timelineContainer.style.display = '';
    }

    annotationOverlay = new AnnotationOverlay(overlayContainer, {
      onTurnClick: (turn: number) => {
        viewer.setTurn(turn);
        updateUI();
        updateEventLog();
        updateAnnotationOverlay();
      },
    });

    // Load both server and local annotations
    const local = loadLocalAnnotations(replay.match_id);
    fetchFeedback(replay.match_id).then(remote => {
      allAnnotations = [...remote, ...local];
      annotationOverlay?.loadAnnotations(replay.match_id, allAnnotations, replay.turns.length);
      syncAnnotationsToViewer();
    }).catch(() => {
      allAnnotations = local;
      annotationOverlay?.loadAnnotations(replay.match_id, allAnnotations, replay.turns.length);
      syncAnnotationsToViewer();
    });

    // Create the annotation form
    createAnnotationForm(formContainer, () => viewer.getTurn(), () => replay.match_id, () => clickedGridPosition);

    // Listen for new annotations from the form
    formContainer.addEventListener('annotation-added', ((e: CustomEvent) => {
      const ann = e.detail as Annotation;
      allAnnotations.push(ann);
      annotationOverlay?.addAnnotation(ann);
      syncAnnotationsToViewer();
      clickedGridPosition = undefined;
      if (canvasAnnotationHint) canvasAnnotationHint.classList.remove('visible');
    }) as EventListener);

    // Add canvas hint overlay
    const canvasWrapper = canvas.parentElement;
    if (canvasWrapper && !canvasWrapper.querySelector('.annotation-canvas-hint')) {
      canvasAnnotationHint = document.createElement('div');
      canvasAnnotationHint.className = 'annotation-canvas-hint';
      canvasAnnotationHint.textContent = 'Click canvas to pin annotation position';
      canvasWrapper.style.position = 'relative';
      canvasWrapper.appendChild(canvasAnnotationHint);
    }

    updateAnnotationOverlay();
  }

  function updateAnnotationOverlay(): void {
    if (annotationOverlay) {
      annotationOverlay.setCurrentTurn(viewer.getTurn());
    }
    if (eventTimeline) {
      eventTimeline.setCurrentTurn(viewer.getTurn());
    }
  }

  // Handle canvas clicks for follow player selection and annotation position
  canvas.addEventListener('click', (e: MouseEvent) => {
    if (!viewer.getReplay()) return;
    const replay = viewer.getReplay();
    if (!replay) return;

    const rect = canvas.getBoundingClientRect();
    const x = (e.clientX - rect.left) * (canvas.width / rect.width);
    const y = (e.clientY - rect.top) * (canvas.height / rect.height);
    const cellSize = viewer.getCellSize();
    const mapRows = replay.map.rows;
    const mapHeight = mapRows * cellSize;
    const overlayY = mapHeight + 4;
    const overlayPadding = 8;
    const lineHeight = 20;

    // Check if click is on score overlay (follow player selection)
    if (y >= overlayY + overlayPadding && x < replay.map.cols * cellSize) {
      const relY = y - overlayY - overlayPadding;
      const playerIdx = Math.floor(relY / lineHeight);
      if (playerIdx >= 0 && playerIdx < replay.players.length) {
        viewer.setFollowPlayer(viewer.getFollowPlayer() === playerIdx ? null : playerIdx);
        return;
      }
    }

    // Only accept clicks within the map area (not score overlay)
    const col = Math.floor(x / cellSize);
    const row = Math.floor(y / cellSize);

    if (row >= 0 && row < mapRows && col >= 0 && col < replay.map.cols) {
      clickedGridPosition = { row, col };
      if (canvasAnnotationHint) {
        canvasAnnotationHint.textContent = `Pinned: (${row}, ${col})`;
        canvasAnnotationHint.classList.add('visible');
      }
    }
  });

  function updateDebugDisplay(debug: Record<number, DebugInfo> | null): void {
    if (!debug || Object.keys(debug).length === 0) {
      debugInfoDisplay.innerHTML = '<div class="no-debug-data">No debug data for this turn</div>';
      return;
    }

    const playerColors = [
      '#332288', '#88ccee', '#44aa99', '#117733', '#999933', '#ddcc77',
    ];
    const replay = viewer.getReplay() as Replay | null;

    let html = '';
    for (const [playerId, info] of Object.entries(debug)) {
      const idx = parseInt(playerId, 10);
      if (viewer.getDebugPlayerEnabled(idx) === false) continue;
      const color = playerColors[idx] || '#888';
      const playerName = replay?.players[idx]?.name ?? `Player ${idx}`;
      const hasData = !!(info.reasoning || (info.targets && info.targets.length > 0));
      if (!hasData) continue;

      html += `<div class="debug-player-info">
        <div class="debug-player-name" style="color:${color}">${playerName}</div>`;

      if (info.reasoning) {
        html += `<div class="debug-reasoning">${escapeHtml(info.reasoning)}</div>`;
      }

      if (info.targets && info.targets.length > 0) {
        html += '<div class="debug-targets">';
        for (const t of info.targets) {
          const pct = t.priority !== undefined ? Math.round(t.priority * 100) : 100;
          const label = t.label ? escapeHtml(t.label) : `(${t.position.row},${t.position.col})`;
          html += `<div class="debug-target-item">
            <span style="color:${t.color || color}">●</span>
            ${label}
            <span class="debug-target-priority">${pct}%</span>
          </div>`;
        }
        html += '</div>';
      }

      html += '</div>';
    }

    debugInfoDisplay.innerHTML = html || '<div class="no-debug-data">No debug data for this turn</div>';
  }

  function escapeHtml(str: string): string {
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function toggleDebugPanel(): void {
    debugPanelExpanded = !debugPanelExpanded;
    debugPanel.classList.toggle('expanded', debugPanelExpanded);
    debugPanelToggleBtn.setAttribute('aria-expanded', String(debugPanelExpanded));
  }

  // ── Director Mode (§16.10) ──────────────────────────────────────────────────

  function initDirector(replay: Replay): void {
    const densities = computeAllDensities(replay);
    directorSchedule = computeSpeedSchedule(densities, directorConfig.targetDuration);
    directorState = createDirectorState();

    // Apply saved duration preset selection
    updateDurationPresetUI(directorConfig.targetDuration);

    if (speedSelect.value === 'director') {
      enableDirector();
    }
  }

  function enableDirector(): void {
    directorState.enabled = true;
    directorState.pauseReason = 'none';
    viewer.setDirectorMode(true);
    directorOptions.style.display = '';
    updateDirectorSpeed();
    startDirectorTick();
  }

  function disableDirector(): void {
    directorState.enabled = false;
    viewer.setDirectorMode(false);
    directorOptions.style.display = 'none';
    stopDirectorTick();
  }

  function startDirectorTick(): void {
    stopDirectorTick();
    function tick() {
      if (!directorState.enabled) return;
      const now = performance.now();
      const turn = viewer.getTurn();
      const ms = tickDirectorSpeed(directorState, directorSchedule, turn, now);
      viewer.setDirectorSpeed(ms);
      updateDirectorStatusUI();
      directorAnimFrame = requestAnimationFrame(tick);
    }
    directorAnimFrame = requestAnimationFrame(tick);
  }

  function stopDirectorTick(): void {
    if (directorAnimFrame !== null) {
      cancelAnimationFrame(directorAnimFrame);
      directorAnimFrame = null;
    }
  }

  function updateDirectorSpeed(): void {
    if (!directorState.enabled) return;
    const now = performance.now();
    const turn = viewer.getTurn();
    const ms = tickDirectorSpeed(directorState, directorSchedule, turn, now);
    viewer.setDirectorSpeed(ms);
    updateDirectorStatusUI();
  }

  function updateDirectorStatusUI(): void {
    if (!directorState.enabled) {
      directorStatus.textContent = '';
      return;
    }
    const transitioning = directorState.easeStartTime > 0 &&
      (performance.now() - directorState.easeStartTime) < 500;
    directorStatus.textContent = formatDirectorLabel(
      directorState.currentMultiplier,
      directorState.targetMultiplier,
      transitioning,
    );
  }

  function updateDurationPresetUI(target: DurationPreset): void {
    durationPresets.querySelectorAll<HTMLElement>('.duration-btn').forEach(btn => {
      const val = parseInt(btn.dataset.duration!, 10) as DurationPreset;
      btn.classList.toggle('active', val === target);
    });
  }

  function setDurationPreset(target: DurationPreset): void {
    directorConfig.targetDuration = target;
    saveDirectorConfig(directorConfig);
    updateDurationPresetUI(target);
    // Recompute schedule with new target
    const replay = viewer.getReplay();
    if (replay) {
      const densities = computeAllDensities(replay);
      directorSchedule = computeSpeedSchedule(densities, directorConfig.targetDuration);
      directorState = createDirectorState();
      directorState.enabled = true;
    }
  }

  function initWinProb(replay: Replay): void {
    if (!replay.win_prob || replay.win_prob.length === 0) {
      winProbSection.style.display = 'none';
      return;
    }

    // Map win_prob: number[][] → WinProbPoint[] (one probs array per turn)
    const points = replay.win_prob.map((probs: number[], t: number) => ({
      turn: t,
      probs: probs.slice(),  // copy to avoid mutation
    }));

    criticalMoments = replay.critical_moments ?? [];

    // Build player colors array matching the viewer's palette
    const playerColors = replay.players.map((_: any, idx: number) => {
      const palettes = [
        '#332288', '#88ccee', '#44aa99', '#117733', '#999933', '#ddcc77',
        '#882255', '#cc6677',
      ];
      return palettes[idx] ?? '#888888';
    });

    viewer.setWinProbabilityData(points);
    viewer.setCriticalMoments(criticalMoments);
    viewer.setWinProbPlayerColors(playerColors);

    winProbSection.style.display = 'block';

    // Dynamic legend: one entry per player
    winProbLegend.innerHTML = replay.players.map((player: any, idx: number) => {
      const color = playerColors[idx];
      const dash = idx === 0 ? '&#8212;' : '--';
      return `<span style="color:${color}">${dash} ${player.name}</span>`;
    }).join(' ');

    winProbContainer.innerHTML = '';
    viewer.createWinProbSparkline(winProbContainer, 800, 70, (turn: number) => {
      viewer.setTurn(turn);
      updateUI();
      updateEventLog();
      updateCriticalMomentNav();
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

  function navigateToPrevCriticalMoment(): void {
    const currentTurn = viewer.getTurn();
    const prev = [...criticalMoments].reverse().find((m: any) => m.turn < currentTurn);
    if (prev) {
      viewer.setTurn(prev.turn);
      updateUI();
      updateEventLog();
      criticalMomentInfo.textContent = prev.description;
    }
  }

  function navigateToNextCriticalMoment(): void {
    const currentTurn = viewer.getTurn();
    const next = criticalMoments.find((m: any) => m.turn > currentTurn);
    if (next) {
      viewer.setTurn(next.turn);
      updateUI();
      updateEventLog();
      criticalMomentInfo.textContent = next.description;
    }
  }

  prevCriticalBtn.addEventListener('click', navigateToPrevCriticalMoment);
  nextCriticalBtn.addEventListener('click', navigateToNextCriticalMoment);

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
    if (directorState.enabled) directorState.pauseReason = 'scrubbing';
    viewer.setTurn(parseInt(turnSlider.value, 10));
    updateUI();
    updateEventLog();
  });
  turnSlider.addEventListener('change', () => {
    if (directorState.enabled) directorState.pauseReason = 'none';
  });

  speedSlider.addEventListener('input', () => {
    const speed = parseInt(speedSlider.value, 10);
    viewer.setSpeed(speed);
    speedDisplay.textContent = String(speed);
    // If user manually drags speed slider, switch off Director mode
    if (directorState.enabled) {
      speedSelect.value = String(speed);
      disableDirector();
    }
  });

  speedSelect.addEventListener('change', () => {
    const val = speedSelect.value;
    if (val === 'director') {
      enableDirector();
      // Update the slider to reflect current director speed
      speedSlider.value = String(Math.round(directorState.easedMsPerTurn));
      speedDisplay.textContent = 'Director';
    } else {
      disableDirector();
      const speed = parseInt(val, 10);
      viewer.setSpeed(speed);
      speedSlider.value = String(speed);
      speedDisplay.textContent = String(speed);
    }
  });

  durationPresets.addEventListener('click', (e) => {
    const btn = (e.target as HTMLElement).closest('.duration-btn') as HTMLElement | null;
    if (!btn) return;
    const duration = parseInt(btn.dataset.duration!, 10) as DurationPreset;
    setDurationPreset(duration);
  });

  fogSelect.addEventListener('change', () => {
    const value = fogSelect.value;
    viewer.setFogOfWar(value === '' ? null : parseInt(value, 10));
  });

  viewModeSelect.addEventListener('change', () => {
    viewer.setViewMode(viewModeSelect.value as ViewMode);
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

  const followZoomSelect = document.getElementById('follow-zoom-select') as HTMLSelectElement;
  followZoomSelect.addEventListener('change', () => {
    viewer.setFollowZoom(parseInt(followZoomSelect.value, 10));
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

  debugPanelToggleBtn.addEventListener('click', toggleDebugPanel);
  debugPanelToggleBtn.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleDebugPanel(); }
  });

  viewer.onTurnChange = () => {
    updateUI();
    updateEventLog();
    updateCriticalMomentNav();
    updateMobileUI();
    updateMobileTimeline();
    viewer.refreshWinProbSparkline();
    updateAnnotationOverlay();
  };
  viewer.onDebugChange = (debug: Record<number, DebugInfo> | null) => {
    updateDebugDisplay(debug);
  };
  viewer.onPlayStateChange = (playing: boolean) => {
    playBtn.textContent = playing ? 'Pause' : 'Play';
    mobilePlayBtn.textContent = playing ? '⏸' : '▶';
  };
  viewer.onCommentaryChange = (entry: { turn: number; text: string; type: string } | null) => {
    if (!entry || !commentaryEnabled) {
      commentaryText.textContent = '';
      return;
    }
    commentaryText.textContent = entry.text;
    commentaryText.className = `commentary-text type-${entry.type}`;
  };

  // ── Mobile controls ─────────────────────────────────────────────────────────
  mobilePlayBtn.addEventListener('click', () => viewer.togglePlay());
  mobilePrevBtn.addEventListener('click', () => {
    viewer.setTurn(viewer.getTurn() - 1);
    updateUI(); updateEventLog(); updateMobileUI(); updateMobileTimeline();
  });
  mobileNextBtn.addEventListener('click', () => {
    viewer.setTurn(viewer.getTurn() + 1);
    updateUI(); updateEventLog(); updateMobileUI(); updateMobileTimeline();
  });
  mobileResetBtn.addEventListener('click', () => {
    viewer.pause(); viewer.setTurn(0);
    updateUI(); updateEventLog(); updateMobileUI(); updateMobileTimeline();
  });
  mobileTurnSlider.addEventListener('input', () => {
    if (directorState.enabled) directorState.pauseReason = 'scrubbing';
    viewer.setTurn(parseInt(mobileTurnSlider.value, 10));
    updateUI(); updateEventLog(); updateMobileUI(); updateMobileTimeline();
  });
  mobileTurnSlider.addEventListener('change', () => {
    if (directorState.enabled) directorState.pauseReason = 'none';
  });
  mobileSpeedBtn.addEventListener('click', () => {
    mobileSpeedIdx = (mobileSpeedIdx + 1) % SPEED_STEPS.length;
    const speed = SPEED_STEPS[mobileSpeedIdx];
    viewer.setSpeed(speed);
    speedDisplay.textContent = String(speed);
    speedSlider.value = String(speed);
    mobileSpeedBtn.textContent = `${speed}ms`;
  });

  // Floating view mode toggle
  mobileViewModeBtn.addEventListener('click', () => {
    const current = viewer.getViewMode();
    const idx = VIEW_MODES.indexOf(current as any);
    const next = VIEW_MODES[(idx + 1) % VIEW_MODES.length];
    viewer.setViewMode(next);
    mobileViewModeBtn.textContent = VIEW_MODE_ICONS[next] ?? '👁';
  });

  // ── Canvas touch gestures ────────────────────────────────────────────────────
  // Tap = play/pause; horizontal swipe = prev/next turn; two-finger pinch = zoom

  let tapStartX = 0;
  let tapStartY = 0;
  let tapStartTime = 0;

  canvas.addEventListener('pointerdown', (e: PointerEvent) => {
    activePointers.set(e.pointerId, e);
    canvas.setPointerCapture(e.pointerId);

    if (activePointers.size === 1) {
      tapStartX = e.clientX;
      tapStartY = e.clientY;
      tapStartTime = Date.now();
    } else if (activePointers.size === 2) {
      const pts = [...activePointers.values()];
      const dx = pts[0].clientX - pts[1].clientX;
      const dy = pts[0].clientY - pts[1].clientY;
      pinchStartDist = Math.sqrt(dx * dx + dy * dy);
      pinchStartCellSize = viewer.getCellSize();
    }
  });

  canvas.addEventListener('pointermove', (e: PointerEvent) => {
    activePointers.set(e.pointerId, e);

    if (activePointers.size === 2) {
      const pts = [...activePointers.values()];
      const dx = pts[0].clientX - pts[1].clientX;
      const dy = pts[0].clientY - pts[1].clientY;
      const dist = Math.sqrt(dx * dx + dy * dy);
      if (pinchStartDist > 0) {
        const newSize = Math.round(pinchStartCellSize * (dist / pinchStartDist));
        viewer.setCellSize(newSize);
      }
    }
  });

  canvas.addEventListener('pointerup', (e: PointerEvent) => {
    const wasOne = activePointers.size === 1;
    const endX = e.clientX;
    const endY = e.clientY;
    activePointers.delete(e.pointerId);

    if (wasOne) {
      const dx = endX - tapStartX;
      const dy = endY - tapStartY;
      const elapsed = Date.now() - tapStartTime;
      const dist = Math.sqrt(dx * dx + dy * dy);

      if (elapsed < 300 && dist < 12) {
        // Tap: play/pause
        if (viewer.getReplay()) viewer.togglePlay();
      } else if (elapsed < 500 && Math.abs(dx) > 40 && Math.abs(dy) < 50) {
        // Horizontal swipe: scrub turn
        if (!viewer.getReplay()) return;
        if (dx < 0) {
          viewer.setTurn(viewer.getTurn() + 1);
        } else {
          viewer.setTurn(viewer.getTurn() - 1);
        }
        updateUI(); updateEventLog(); updateMobileUI(); updateMobileTimeline();
      }
    }

    if (activePointers.size < 2) {
      pinchStartDist = 0;
    }
  });

  canvas.addEventListener('pointercancel', (e: PointerEvent) => {
    activePointers.delete(e.pointerId);
    if (activePointers.size < 2) pinchStartDist = 0;
  });

  // Commentary toggle
  commentaryToggle.addEventListener('click', () => {
    commentaryEnabled = !commentaryEnabled;
    viewer.setCommentaryEnabled(commentaryEnabled);
    commentaryToggle.classList.toggle('active', commentaryEnabled);
    if (!commentaryEnabled) {
      commentaryText.textContent = '';
    }
  });

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
      case 'BracketLeft':
        e.preventDefault();
        navigateToPrevCriticalMoment();
        break;
      case 'BracketRight':
        e.preventDefault();
        navigateToNextCriticalMoment();
        break;
      case 'KeyF':
        e.preventDefault();
        theater.toggle();
        break;
      case 'Digit0':
      case 'Escape':
        if (theater.isActive()) break; // let theater handle its own escape
        e.preventDefault();
        viewer.setFollowPlayer(null);
        break;
      case 'Digit1': case 'Digit2': case 'Digit3':
      case 'Digit4': case 'Digit5': case 'Digit6':
        e.preventDefault();
        const followIdx = parseInt(e.code.replace('Digit', ''), 10) - 1;
        const replay = viewer.getReplay();
        if (replay && followIdx < replay.players.length) {
          viewer.setFollowPlayer(viewer.getFollowPlayer() === followIdx ? null : followIdx);
        }
        break;
    }
  });

  // Load from URL param if provided
  if (initialUrl) {
    urlInput.value = initialUrl;
    loadUrlBtn.click();
  }
}
