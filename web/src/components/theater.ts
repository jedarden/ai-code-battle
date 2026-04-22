// Theater mode — fullscreen replay viewing per §16.17
// Controls auto-hide after 3s of mouse inactivity, reappear on mousemove.
// ESC exits; F key toggles. Works on mobile via Fullscreen API.

export const THEATER_STYLES = `
/* ─── Theater Mode (§16.17) ────────────────────────────────────────────────── */

.theater-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  background: var(--bg-tertiary, #1e293b);
  border: 1px solid var(--border, #334155);
  border-radius: 6px;
  color: var(--text-secondary, #94a3b8);
  cursor: pointer;
  transition: background-color 0.15s, color 0.15s;
  font-size: 16px;
  line-height: 1;
}
.theater-btn:hover {
  background: var(--bg-secondary, #0f172a);
  color: var(--text-primary, #e2e8f0);
}
.theater-btn:focus-visible {
  outline: 2px solid var(--accent, #3b82f6);
  outline-offset: 2px;
}

/* Full-screen theater overlay */
.theater-overlay {
  position: fixed;
  inset: 0;
  z-index: 9000;
  background: #000;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 300ms ease;
  cursor: none;
}
.theater-overlay.visible {
  opacity: 1;
}

/* Canvas scales to viewport, letterboxed */
.theater-canvas-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  width: 100%;
  overflow: hidden;
}
.theater-canvas-wrap canvas {
  display: block;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

/* Controls bar — semi-transparent, fades after 3s */
.theater-controls {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  background: linear-gradient(transparent, rgba(0,0,0,0.85));
  opacity: 1;
  transition: opacity 400ms ease;
  cursor: default;
  user-select: none;
}
.theater-overlay.controls-hidden .theater-controls {
  opacity: 0;
  pointer-events: none;
}

.theater-controls .theater-ctrl-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  padding: 0;
  background: none;
  border: none;
  border-radius: 4px;
  color: #e2e8f0;
  cursor: pointer;
  font-size: 18px;
  transition: background-color 0.15s;
}
.theater-controls .theater-ctrl-btn:hover {
  background: rgba(255,255,255,0.15);
}
.theater-controls .theater-ctrl-btn:focus-visible {
  outline: 2px solid #3b82f6;
  outline-offset: 2px;
}

.theater-score {
  color: #e2e8f0;
  font-size: 0.85rem;
  font-family: monospace;
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 8px;
}
.theater-score .player-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.theater-turn-info {
  color: #94a3b8;
  font-size: 0.8rem;
  font-family: monospace;
  white-space: nowrap;
  margin-left: auto;
}

/* Thin win-prob bars at the top edge */
.theater-winprob-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  display: flex;
  height: 4px;
  opacity: 1;
  transition: opacity 400ms ease;
}
.theater-overlay.controls-hidden .theater-winprob-bar {
  opacity: 0;
}
.theater-winprob-segment {
  height: 100%;
  transition: width 300ms ease;
}

/* Vignette pulse on critical moment */
.theater-vignette {
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0;
  box-shadow: inset 0 0 120px 40px rgba(0,0,0,0.8);
  transition: opacity 400ms ease;
}
.theater-vignette.pulse {
  opacity: 1;
}

/* Speed indicator (shows current speed label) */
.theater-speed-label {
  color: #94a3b8;
  font-size: 0.75rem;
  font-family: monospace;
}

/* Theater exit hint (brief, fades out) */
.theater-exit-hint {
  position: absolute;
  top: 16px;
  right: 16px;
  color: #94a3b8;
  font-size: 0.75rem;
  opacity: 0;
  transition: opacity 300ms ease;
  pointer-events: none;
}
.theater-overlay.controls-hidden .theater-exit-hint {
  opacity: 0;
}
.theater-overlay:not(.controls-hidden) .theater-exit-hint {
  opacity: 0.7;
}

/* Reduced motion */
@media (prefers-reduced-motion: reduce) {
  .theater-overlay,
  .theater-controls,
  .theater-winprob-bar,
  .theater-vignette {
    transition: none;
  }
}
`;

const AUTO_HIDE_MS = 3000;
const VIGNETTE_PULSE_MS = 600;

export interface TheaterOptions {
  getScoreText: () => string;
  getPlayerColors: () => string[];
  getWinProb: () => number[];
  getTurn: () => number;
  getTotalTurns: () => number;
  getIsPlaying: () => boolean;
  getSpeed: () => number;
  togglePlay: () => void;
  setTurn: (t: number) => void;
  exitTheater: () => void;
  onCriticalMoment?: () => void;
}

export class TheaterMode {
  private overlay: HTMLDivElement;
  private canvasWrap!: HTMLDivElement;
  private controls!: HTMLDivElement;
  private winProbBar!: HTMLDivElement;
  private vignette!: HTMLDivElement;
  private exitHint!: HTMLDivElement;
  private canvas: HTMLCanvasElement;

  private scoreEl!: HTMLSpanElement;
  private turnInfoEl!: HTMLSpanElement;
  private speedLabelEl!: HTMLSpanElement;
  private playBtn!: HTMLButtonElement;

  private hideTimer: ReturnType<typeof setTimeout> | null = null;
  private vignetteTimer: ReturnType<typeof setTimeout> | null = null;
  private rafId: number | null = null;
  private active = false;
  private origParent: HTMLElement | null = null;
  private origNextSibling: Node | null = null;
  private fullscreenChangeHandler: () => void;

  private opts: TheaterOptions;

  constructor(canvas: HTMLCanvasElement, opts: TheaterOptions) {
    this.canvas = canvas;
    this.opts = opts;
    this.overlay = document.createElement('div');
    this.overlay.className = 'theater-overlay';
    this.fullscreenChangeHandler = () => this.onFullscreenChange();
    this.buildDOM();
  }

  /** Returns true if theater mode is currently active. */
  isActive(): boolean {
    return this.active;
  }

  /** Enter theater mode. */
  enter(): void {
    if (this.active) return;
    this.active = true;

    // Remember original position
    this.origParent = this.canvas.parentElement!;
    this.origNextSibling = this.canvas.nextSibling;

    // Move canvas into theater overlay
    this.canvasWrap.appendChild(this.canvas);
    document.body.appendChild(this.overlay);

    // Force layout then animate in
    requestAnimationFrame(() => {
      this.overlay.classList.add('visible');
    });

    // Request fullscreen via Fullscreen API
    this.requestFullscreen();

    // Start UI update loop
    this.startUILoop();

    // Auto-hide controls after inactivity
    this.resetHideTimer();

    // Attach listeners
    this.overlay.addEventListener('mousemove', this.onMouseMove);
    this.overlay.addEventListener('mousedown', this.onMouseMove);
    this.overlay.addEventListener('touchstart', this.onTouch, { passive: true });
    document.addEventListener('keydown', this.onKeyDown);

    document.addEventListener('fullscreenchange', this.fullscreenChangeHandler);
    document.addEventListener('webkitfullscreenchange', this.fullscreenChangeHandler);

    // Initial UI
    this.updateUI();
  }

  /** Exit theater mode. */
  exit(): void {
    if (!this.active) return;
    this.active = false;

    // Cancel fullscreen if active
    this.exitFullscreen();

    // Remove listeners
    this.overlay.removeEventListener('mousemove', this.onMouseMove);
    this.overlay.removeEventListener('mousedown', this.onMouseMove);
    this.overlay.removeEventListener('touchstart', this.onTouch);
    document.removeEventListener('keydown', this.onKeyDown);
    document.removeEventListener('fullscreenchange', this.fullscreenChangeHandler);
    document.removeEventListener('webkitfullscreenchange', this.fullscreenChangeHandler);

    this.stopUILoop();
    this.clearHideTimer();

    // Fade out
    this.overlay.classList.remove('visible');
    this.overlay.classList.remove('controls-hidden');

    // Move canvas back after animation
    setTimeout(() => {
      if (this.origParent) {
        if (this.origNextSibling) {
          this.origParent.insertBefore(this.canvas, this.origNextSibling);
        } else {
          this.origParent.appendChild(this.canvas);
        }
      }
      this.overlay.remove();
      this.origParent = null;
      this.origNextSibling = null;
    }, 300);
  }

  /** Toggle theater mode. */
  toggle(): void {
    if (this.active) this.exit();
    else this.enter();
  }

  /** Trigger a vignette pulse (called externally on critical moments). */
  pulseVignette(): void {
    this.vignette.classList.add('pulse');
    if (this.vignetteTimer) clearTimeout(this.vignetteTimer);
    this.vignetteTimer = setTimeout(() => {
      this.vignette.classList.remove('pulse');
    }, VIGNETTE_PULSE_MS);
  }

  /** Update score text (called externally). */
  updateUI(): void {
    this.scoreEl.textContent = this.opts.getScoreText();
    const turn = this.opts.getTurn();
    const total = this.opts.getTotalTurns();
    this.turnInfoEl.textContent = `Turn ${turn + 1}/${total}`;
    this.playBtn.textContent = this.opts.getIsPlaying() ? '⏸' : '▶';
    const speed = this.opts.getSpeed();
    const label = speed <= 31 ? '16x' : speed <= 62 ? '8x' : speed <= 125 ? '4x' : speed <= 250 ? '2x' : '1x';
    this.speedLabelEl.textContent = label;
    this.updateWinProbBar();
    this.updatePlayerDots();
  }

  destroy(): void {
    if (this.active) this.exit();
  }

  // ── Private ────────────────────────────────────────────────────────────────

  private buildDOM(): void {
    // Vignette
    this.vignette = document.createElement('div');
    this.vignette.className = 'theater-vignette';
    this.overlay.appendChild(this.vignette);

    // Win prob bar
    this.winProbBar = document.createElement('div');
    this.winProbBar.className = 'theater-winprob-bar';
    this.overlay.appendChild(this.winProbBar);

    // Exit hint
    this.exitHint = document.createElement('div');
    this.exitHint.className = 'theater-exit-hint';
    this.exitHint.textContent = 'Press ESC or F to exit';
    this.overlay.appendChild(this.exitHint);

    // Canvas wrapper
    this.canvasWrap = document.createElement('div');
    this.canvasWrap.className = 'theater-canvas-wrap';
    this.overlay.appendChild(this.canvasWrap);

    // Controls bar
    this.controls = document.createElement('div');
    this.controls.className = 'theater-controls';

    const prevBtn = document.createElement('button');
    prevBtn.className = 'theater-ctrl-btn';
    prevBtn.innerHTML = '&#9664;';
    prevBtn.title = 'Previous turn';
    prevBtn.setAttribute('aria-label', 'Previous turn');
    prevBtn.addEventListener('click', (e) => { e.stopPropagation(); this.opts.setTurn(this.opts.getTurn() - 1); this.updateUI(); });

    this.playBtn = document.createElement('button');
    this.playBtn.className = 'theater-ctrl-btn';
    this.playBtn.innerHTML = '&#9654;';
    this.playBtn.title = 'Play/Pause';
    this.playBtn.setAttribute('aria-label', 'Play or pause');
    this.playBtn.addEventListener('click', (e) => { e.stopPropagation(); this.opts.togglePlay(); this.updateUI(); });

    const nextBtn = document.createElement('button');
    nextBtn.className = 'theater-ctrl-btn';
    nextBtn.innerHTML = '&#9654;&#9654;';
    nextBtn.title = 'Next turn';
    nextBtn.setAttribute('aria-label', 'Next turn');
    nextBtn.addEventListener('click', (e) => { e.stopPropagation(); this.opts.setTurn(this.opts.getTurn() + 1); this.updateUI(); });

    this.scoreEl = document.createElement('span');
    this.scoreEl.className = 'theater-score';

    this.speedLabelEl = document.createElement('span');
    this.speedLabelEl.className = 'theater-speed-label';

    this.turnInfoEl = document.createElement('span');
    this.turnInfoEl.className = 'theater-turn-info';

    this.controls.appendChild(prevBtn);
    this.controls.appendChild(this.playBtn);
    this.controls.appendChild(nextBtn);
    this.controls.appendChild(this.scoreEl);
    this.controls.appendChild(this.speedLabelEl);
    this.controls.appendChild(this.turnInfoEl);

    this.overlay.appendChild(this.controls);
  }

  private updateWinProbBar(): void {
    const probs = this.opts.getWinProb();
    if (!probs || probs.length === 0) {
      this.winProbBar.style.display = 'none';
      return;
    }
    this.winProbBar.style.display = '';
    const colors = this.opts.getPlayerColors();
    // Ensure segments exist
    while (this.winProbBar.children.length < probs.length) {
      const seg = document.createElement('div');
      seg.className = 'theater-winprob-segment';
      this.winProbBar.appendChild(seg);
    }
    while (this.winProbBar.children.length > probs.length) {
      this.winProbBar.removeChild(this.winProbBar.lastChild!);
    }
    for (let i = 0; i < probs.length; i++) {
      const seg = this.winProbBar.children[i] as HTMLElement;
      seg.style.width = `${(probs[i] * 100).toFixed(1)}%`;
      seg.style.backgroundColor = colors[i] || '#888';
    }
  }

  private updatePlayerDots(): void {
    const colors = this.opts.getPlayerColors();
    let html = '';
    for (let i = 0; i < colors.length; i++) {
      html += `<span class="player-dot" style="background:${colors[i]}"></span>`;
    }
    // Prepend dots before text
    const text = this.opts.getScoreText();
    this.scoreEl.innerHTML = html + text;
  }

  private requestFullscreen(): void {
    const el = this.overlay as any;
    if (el.requestFullscreen) {
      el.requestFullscreen().catch(() => { /* user may deny */ });
    } else if (el.webkitRequestFullscreen) {
      el.webkitRequestFullscreen();
    }
  }

  private exitFullscreen(): void {
    const doc = document as any;
    if (doc.fullscreenElement || doc.webkitFullscreenElement) {
      if (doc.exitFullscreen) {
        doc.exitFullscreen().catch(() => {});
      } else if (doc.webkitExitFullscreen) {
        doc.webkitExitFullscreen();
      }
    }
  }

  private onFullscreenChange(): void {
    const doc = document as any;
    const isFs = !!(doc.fullscreenElement || doc.webkitFullscreenElement);
    if (!isFs && this.active) {
      // User pressed ESC at browser level — exit theater
      this.exit();
      this.opts.exitTheater();
    }
  }

  private resetHideTimer(): void {
    this.clearHideTimer();
    this.overlay.classList.remove('controls-hidden');
    this.hideTimer = setTimeout(() => {
      this.overlay.classList.add('controls-hidden');
    }, AUTO_HIDE_MS);
  }

  private clearHideTimer(): void {
    if (this.hideTimer !== null) {
      clearTimeout(this.hideTimer);
      this.hideTimer = null;
    }
  }

  private readonly onMouseMove = (): void => {
    this.resetHideTimer();
    this.updateUI();
  };

  private readonly onTouch = (): void => {
    this.resetHideTimer();
    this.updateUI();
  };

  private readonly onKeyDown = (e: KeyboardEvent): void => {
    if (!this.active) return;
    switch (e.code) {
      case 'KeyF':
      case 'Escape':
        e.preventDefault();
        e.stopPropagation();
        this.exit();
        this.opts.exitTheater();
        break;
      case 'Space':
        e.preventDefault();
        e.stopPropagation();
        this.opts.togglePlay();
        this.updateUI();
        this.resetHideTimer();
        break;
      case 'ArrowLeft':
        e.preventDefault();
        e.stopPropagation();
        this.opts.setTurn(this.opts.getTurn() - 1);
        this.updateUI();
        this.resetHideTimer();
        break;
      case 'ArrowRight':
        e.preventDefault();
        e.stopPropagation();
        this.opts.setTurn(this.opts.getTurn() + 1);
        this.updateUI();
        this.resetHideTimer();
        break;
    }
  };

  private startUILoop(): void {
    const tick = () => {
      if (!this.active) return;
      this.updateUI();
      this.rafId = requestAnimationFrame(tick);
    };
    this.rafId = requestAnimationFrame(tick);
  }

  private stopUILoop(): void {
    if (this.rafId !== null) {
      cancelAnimationFrame(this.rafId);
      this.rafId = null;
    }
  }
}
