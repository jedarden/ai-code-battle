// Picture-in-Picture replay mini-player (§16.13)
// When the user navigates away from a replay page, the canvas is reparented
// into a fixed-position floating container. Playback continues uninterrupted.
// Navigating back to the same replay reparents the canvas inline.

export const PIP_STYLES = `
.pip-container {
  position: fixed;
  bottom: 16px;
  right: 16px;
  width: 240px;
  background: #111827;
  border-radius: 8px;
  box-shadow: 0 4px 24px rgba(0,0,0,0.6);
  z-index: 1000;
  overflow: hidden;
  transition: transform 300ms ease-out, opacity 300ms ease-out;
  will-change: transform;
  user-select: none;
}
.pip-container.pip-minimizing {
  transform: scale(0.8) translateY(20px);
  opacity: 0;
}
.pip-container.pip-expanding {
  animation: pip-expand-in 300ms ease-out forwards;
}
@keyframes pip-expand-in {
  from { transform: scale(0.8) translateY(20px); opacity: 0; }
  to   { transform: scale(1) translateY(0); opacity: 1; }
}

.pip-canvas-wrapper {
  width: 100%;
  height: 180px;
  background: #0f172a;
  cursor: pointer;
  position: relative;
}
.pip-canvas-wrapper canvas {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: contain;
}

.pip-controls {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  background: #1e293b;
  font-size: 11px;
  color: #94a3b8;
  font-family: monospace;
}
.pip-btn {
  background: none;
  border: none;
  color: #e2e8f0;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
  line-height: 1;
}
.pip-btn:hover {
  background: rgba(255,255,255,0.1);
}
.pip-score {
  flex: 1;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #cbd5e1;
}
.pip-close {
  background: none;
  border: none;
  color: #64748b;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 14px;
  line-height: 1;
}
.pip-close:hover {
  background: rgba(255,255,255,0.1);
  color: #f87171;
}
.pip-turn {
  font-size: 10px;
  color: #64748b;
}

/* Dragging state */
.pip-container.pip-dragging {
  transition: none;
  cursor: grabbing;
}

/* Mobile: smaller, above bottom tab bar */
@media (max-width: 640px) {
  .pip-container {
    width: 150px;
    bottom: 70px;
    right: 10px;
  }
  .pip-canvas-wrapper {
    height: 112px;
  }
  .pip-controls {
    padding: 4px 6px;
    font-size: 10px;
  }
}
`;

export interface PipState {
  /** The match ID currently in PIP */
  matchId: string;
  /** The canvas element (still owned by ReplayViewer) */
  canvas: HTMLCanvasElement;
  /** The original parent to restore to */
  originalParent: HTMLElement;
  /** Score text for display */
  getScoreText: () => string;
  /** Get current turn */
  getTurn: () => number;
  /** Get total turns */
  getTotalTurns: () => number;
  /** Get playing state */
  getIsPlaying: () => boolean;
  /** Toggle play/pause */
  togglePlay: () => void;
  /** Called when user clicks PIP to return to full view */
  onReturn: () => void;
  /** Called when user closes PIP */
  onClose: () => void;
}

let pipContainer: HTMLElement | null = null;
let pipState: PipState | null = null;
let pipScoreEl: HTMLElement | null = null;
let pipTurnEl: HTMLElement | null = null;
let pipPlayBtn: HTMLButtonElement | null = null;
let pipStyleInjected = false;
let pipTurnInterval: number | null = null;

// Drag state
let dragOffsetX = 0;
let dragOffsetY = 0;
let isDragging = false;

function injectPipStyles(): void {
  if (pipStyleInjected) return;
  const style = document.createElement('style');
  style.id = 'pip-styles';
  style.textContent = PIP_STYLES;
  document.head.appendChild(style);
  pipStyleInjected = true;
}

function updatePipTurnDisplay(): void {
  if (!pipState || !pipTurnEl || !pipPlayBtn) return;
  const turn = pipState.getTurn();
  const total = pipState.getTotalTurns();
  pipTurnEl.textContent = `T:${turn}/${total - 1}`;
  pipPlayBtn.textContent = pipState.getIsPlaying() ? '⏸' : '▶';
}

function startTurnPolling(): void {
  stopTurnPolling();
  pipTurnInterval = window.setInterval(updatePipTurnDisplay, 250);
}

function stopTurnPolling(): void {
  if (pipTurnInterval !== null) {
    clearInterval(pipTurnInterval);
    pipTurnInterval = null;
  }
}

function createPipContainer(): HTMLElement {
  const container = document.createElement('div');
  container.className = 'pip-container pip-expanding';
  container.setAttribute('role', 'complementary');
  container.setAttribute('aria-label', 'Mini replay player');

  container.innerHTML = `
    <div class="pip-canvas-wrapper" id="pip-canvas-slot"></div>
    <div class="pip-controls">
      <button class="pip-btn" id="pip-play-btn" aria-label="Play or pause">▶</button>
      <span class="pip-score" id="pip-score">-</span>
      <span class="pip-turn" id="pip-turn">T:0/0</span>
      <button class="pip-btn" id="pip-return-btn" aria-label="Return to full view" title="Return to full view">⤢</button>
      <button class="pip-close" id="pip-close-btn" aria-label="Close mini player">&times;</button>
    </div>
  `;

  return container;
}

/**
 * Activate PIP: reparent the canvas into the floating container.
 * Call from the router's beforeNavigate hook when leaving a replay page
 * that has an active viewer.
 */
export function activatePip(state: PipState): void {
  // Only one PIP at a time
  if (pipState) {
    closePip();
  }

  injectPipStyles();
  pipState = state;

  pipContainer = createPipContainer();
  document.body.appendChild(pipContainer);

  const slot = pipContainer.querySelector('#pip-canvas-slot') as HTMLElement;
  pipScoreEl = pipContainer.querySelector('#pip-score');
  pipTurnEl = pipContainer.querySelector('#pip-turn');
  pipPlayBtn = pipContainer.querySelector('#pip-play-btn');
  const returnBtn = pipContainer.querySelector('#pip-return-btn') as HTMLButtonElement;
  const closeBtn = pipContainer.querySelector('#pip-close-btn') as HTMLButtonElement;

  // Move canvas into PIP container
  slot.appendChild(state.canvas);

  // Wire controls
  if (pipScoreEl) pipScoreEl.textContent = state.getScoreText();
  updatePipTurnDisplay();

  pipPlayBtn?.addEventListener('click', (e) => {
    e.stopPropagation();
    state.togglePlay();
    updatePipTurnDisplay();
  });

  returnBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    state.onReturn();
  });

  closeBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    state.onClose();
  });

  // Click canvas wrapper to return
  slot.addEventListener('click', () => {
    state.onReturn();
  });

  // Make PIP draggable
  setupDrag(pipContainer);

  startTurnPolling();
}

/**
 * Restore the canvas back to its original inline parent.
 * Call when navigating back to the same replay page.
 */
export function restorePip(): { canvas: HTMLCanvasElement; originalParent: HTMLElement } | null {
  if (!pipState || !pipContainer) return null;

  stopTurnPolling();

  const { canvas, originalParent } = pipState;

  // Animate out
  pipContainer.classList.add('pip-minimizing');

  const container = pipContainer;

  // Immediately reparent canvas back (the animation is on the container shell)
  originalParent.appendChild(canvas);

  // Clean up after animation
  setTimeout(() => {
    container.remove();
  }, 300);

  pipContainer = null;
  pipState = null;
  pipScoreEl = null;
  pipTurnEl = null;
  pipPlayBtn = null;

  return { canvas, originalParent };
}

/**
 * Close PIP and stop playback entirely.
 */
export function closePip(): void {
  if (!pipState || !pipContainer) return;

  stopTurnPolling();

  const container = pipContainer;
  container.classList.add('pip-minimizing');

  setTimeout(() => {
    container.remove();
  }, 300);

  pipState = null;
  pipContainer = null;
  pipScoreEl = null;
  pipTurnEl = null;
  pipPlayBtn = null;
}

/**
 * Check if PIP is currently active, optionally for a specific match.
 */
export function isPipActive(matchId?: string): boolean {
  if (!pipState) return false;
  if (matchId) return pipState.matchId === matchId;
  return true;
}

/**
 * Get the current PIP match ID, or null.
 */
export function getPipMatchId(): string | null {
  return pipState?.matchId ?? null;
}

/**
 * Get the stored canvas and original parent for rehydrating a returning replay page.
 */
export function getPipRestoreData(): { canvas: HTMLCanvasElement; originalParent: HTMLElement } | null {
  if (!pipState) return null;
  return { canvas: pipState.canvas, originalParent: pipState.originalParent };
}

// ── Drag logic ──────────────────────────────────────────────────────────────────

function setupDrag(container: HTMLElement): void {
  const controls = container.querySelector('.pip-controls') as HTMLElement;
  if (!controls) return;

  controls.addEventListener('pointerdown', onDragStart);
}

function onDragStart(e: PointerEvent): void {
  if (!pipContainer) return;
  // Don't drag if clicking a button
  if ((e.target as HTMLElement).tagName === 'BUTTON') return;

  isDragging = true;
  pipContainer.classList.add('pip-dragging');

  const rect = pipContainer.getBoundingClientRect();
  dragOffsetX = e.clientX - rect.left;
  dragOffsetY = e.clientY - rect.top;

  // Switch from bottom-right positioning to top-left for easier drag math
  pipContainer.style.left = rect.left + 'px';
  pipContainer.style.top = rect.top + 'px';
  pipContainer.style.right = 'auto';
  pipContainer.style.bottom = 'auto';

  document.addEventListener('pointermove', onDragMove);
  document.addEventListener('pointerup', onDragEnd);
  e.preventDefault();
}

function onDragMove(e: PointerEvent): void {
  if (!isDragging || !pipContainer) return;
  const x = Math.max(0, Math.min(window.innerWidth - pipContainer.offsetWidth, e.clientX - dragOffsetX));
  const y = Math.max(0, Math.min(window.innerHeight - pipContainer.offsetHeight, e.clientY - dragOffsetY));
  pipContainer.style.left = x + 'px';
  pipContainer.style.top = y + 'px';
}

function onDragEnd(): void {
  isDragging = false;
  if (pipContainer) pipContainer.classList.remove('pip-dragging');
  document.removeEventListener('pointermove', onDragMove);
  document.removeEventListener('pointerup', onDragEnd);
}
