// Mobile Playlist Carousel — full-screen swipeable cards (§16.16)
// On mobile viewport (<768px), playlist entry opens this TikTok-style carousel.
// Desktop unaffected — keeps the grid layout from playlists.ts.

import type { Playlist, PlaylistMatch } from '../api-types';
import type { Replay } from '../types';
import {
  computeAllDensities,
  computeSpeedSchedule,
  createDirectorState,
  tickDirectorSpeed,
  type DirectorState,
  type DurationPreset,
} from './director';

const loadReplayViewer = () => import('../replay-viewer');

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// ── Touch tracking for live 60fps swipe ─────────────────────────────────────

interface TouchTracker {
  startX: number;
  startY: number;
  startTime: number;
  tracking: boolean;
  locked: 'none' | 'vertical' | 'horizontal';
  currentDeltaY: number;
  currentDeltaX: number;
}

// ── Carousel component ─────────────────────────────────────────────────────

export interface CarouselOptions {
  playlist: Playlist;
  startIndex?: number;
  onClose: () => void;
  autoAdvanceDelay?: number; // ms, default 3000
}

const DEFAULT_AUTO_ADVANCE_DELAY = 3000;
const METADATA_PANEL_WIDTH = 280;
const TRANSITION_MS = 300;
const R2_BASE = '/r2';
const B2_FALLBACK = 'https://b2.aicodebattle.com';
const SWIPE_THRESHOLD = 50; // min px to trigger advance
const VELOCITY_THRESHOLD = 0.3; // px/ms — fast flick triggers even below threshold
const REDUCED_MOTION = typeof window !== 'undefined'
  && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

export class PlaylistCarousel {
  private overlay: HTMLDivElement;
  private styleEl: HTMLStyleElement;
  private playlist: Playlist;
  private currentIndex: number;
  private onClose: () => void;
  private autoAdvanceDelay: number;

  // Per-card DOM
  private carouselInner: HTMLDivElement;
  private canvas: HTMLCanvasElement;
  private headerBar: HTMLDivElement;
  private scoreBar: HTMLDivElement;
  private eventHint: HTMLDivElement;
  private swipeHint: HTMLDivElement;
  private metadataPanel: HTMLDivElement;
  private closeBtn: HTMLButtonElement;
  private countdownRing: HTMLDivElement;

  // Replay viewer
  private viewer: InstanceType<typeof import('../replay-viewer').ReplayViewer> | null = null;

  // Director state
  private directorState: DirectorState = createDirectorState();
  private directorSchedule: ReturnType<typeof computeSpeedSchedule> = [];
  private directorAnimFrame: number | null = null;

  // Preloading
  private preloadedReplays = new Map<number, Replay>();

  // Auto-advance timer
  private autoAdvanceTimer: ReturnType<typeof setTimeout> | null = null;
  private countdownAnimFrame: number | null = null;

  // Metadata panel state
  private metadataOpen = false;

  // Transition state
  private transitioning = false;

  // Touch tracking for live swipe
  private touch: TouchTracker = {
    startX: 0, startY: 0, startTime: 0,
    tracking: false, locked: 'none',
    currentDeltaY: 0, currentDeltaX: 0,
  };

  constructor(opts: CarouselOptions) {
    this.playlist = opts.playlist;
    this.currentIndex = opts.startIndex ?? 0;
    this.onClose = opts.onClose;
    this.autoAdvanceDelay = opts.autoAdvanceDelay ?? DEFAULT_AUTO_ADVANCE_DELAY;

    // Create overlay
    this.overlay = document.createElement('div');
    this.overlay.className = 'carousel-overlay';
    this.overlay.innerHTML = CAROUSEL_HTML;
    document.body.appendChild(this.overlay);
    document.body.style.overflow = 'hidden';

    // Inject styles
    this.styleEl = document.createElement('style');
    this.styleEl.textContent = CAROUSEL_CSS;
    document.head.appendChild(this.styleEl);

    // Grab refs
    this.carouselInner = this.overlay.querySelector('.carousel-card')!;
    this.canvas = this.overlay.querySelector('.carousel-canvas')!;
    this.headerBar = this.overlay.querySelector('.carousel-header')!;
    this.scoreBar = this.overlay.querySelector('.carousel-score-bar')!;
    this.eventHint = this.overlay.querySelector('.carousel-event-hint')!;
    this.swipeHint = this.overlay.querySelector('.carousel-swipe-hint')!;
    this.metadataPanel = this.overlay.querySelector('.carousel-metadata-panel')!;
    this.closeBtn = this.overlay.querySelector('.carousel-close-btn')!;
    this.countdownRing = this.overlay.querySelector('.carousel-countdown-ring')!;

    // Close button
    this.closeBtn.addEventListener('click', () => this.destroy());

    // Touch events with live tracking
    this.carouselInner.addEventListener('touchstart', (e) => this.onTouchStart(e), { passive: true });
    this.carouselInner.addEventListener('touchmove', (e) => this.onTouchMove(e), { passive: false });
    this.carouselInner.addEventListener('touchend', (e) => this.onTouchEnd(e), { passive: true });
    this.carouselInner.addEventListener('touchcancel', () => this.onTouchCancel(), { passive: true });

    // Tap on canvas = play/pause
    this.canvas.addEventListener('click', () => {
      if (this.viewer?.getReplay()) this.viewer.togglePlay();
    });

    // Initialize
    this.init();
  }

  private async init(): Promise<void> {
    const { ReplayViewer } = await loadReplayViewer();
    this.viewer = new ReplayViewer(this.canvas, {
      cellSize: 6,
      animationSpeed: 100,
    });

    this.viewer.onTurnChange = () => {
      if (!this.viewer) return;
      if (this.viewer.getIsPlaying() && this.viewer.getTurn() >= this.viewer.getTotalTurns() - 1) {
        this.viewer.pause();
        this.onReplayEnd();
      }
    };

    this.viewer.onPlayStateChange = () => { /* keep callback wired */ };

    await this.loadCard(this.currentIndex);

    // Fade swipe hint after 3s
    setTimeout(() => {
      if (this.swipeHint) this.swipeHint.classList.add('carousel-hint-fade');
    }, 3000);
  }

  // ── Touch handling with live 60fps tracking ──────────────────────────────

  private onTouchStart(e: TouchEvent): void {
    if (e.touches.length !== 1 || this.transitioning) return;
    this.touch.startX = e.touches[0].clientX;
    this.touch.startY = e.touches[0].clientY;
    this.touch.startTime = Date.now();
    this.touch.tracking = true;
    this.touch.locked = 'none';
    this.touch.currentDeltaY = 0;
    this.touch.currentDeltaX = 0;
  }

  private onTouchMove(e: TouchEvent): void {
    if (!this.touch.tracking || this.transitioning) return;
    if (e.touches.length !== 1) return;

    const dx = e.touches[0].clientX - this.touch.startX;
    const dy = e.touches[0].clientY - this.touch.startY;

    // Lock axis on first significant movement
    if (this.touch.locked === 'none' && (Math.abs(dx) > 10 || Math.abs(dy) > 10)) {
      this.touch.locked = Math.abs(dx) > Math.abs(dy) ? 'horizontal' : 'vertical';
    }

    if (this.touch.locked === 'vertical') {
      this.touch.currentDeltaY = dy;
      // Apply live transform for visual feedback
      if (!REDUCED_MOTION) {
        this.carouselInner.style.transition = 'none';
        this.carouselInner.style.transform = `translateY(${dy}px)`;
        this.carouselInner.style.opacity = String(1 - Math.min(Math.abs(dy) / 400, 0.4));
      }
      // Prevent scroll-through
      e.preventDefault();
    } else if (this.touch.locked === 'horizontal') {
      if (this.metadataOpen) {
        // Drag metadata panel closed
        const clampedDx = Math.max(0, Math.min(METADATA_PANEL_WIDTH, -dx));
        this.touch.currentDeltaX = clampedDx;
        if (!REDUCED_MOTION) {
          this.metadataPanel.style.transition = 'none';
          this.metadataPanel.style.transform = `translateX(${METADATA_PANEL_WIDTH - clampedDx}px)`;
          this.carouselInner.style.transition = 'none';
          this.carouselInner.style.transform = `translateX(${-METADATA_PANEL_WIDTH + clampedDx}px)`;
        }
      } else {
        this.touch.currentDeltaX = dx;
        if (!REDUCED_MOTION) {
          // Peek metadata panel
          const reveal = Math.max(0, Math.min(METADATA_PANEL_WIDTH, dx));
          const ratio = reveal / METADATA_PANEL_WIDTH;
          this.metadataPanel.style.transition = 'none';
          this.metadataPanel.style.transform = `translateX(${METADATA_PANEL_WIDTH - reveal}px)`;
          this.carouselInner.style.transition = 'none';
          this.carouselInner.style.transform = `translateX(${-reveal}px)`;
          this.metadataPanel.style.opacity = String(ratio);
        }
      }
    }
  }

  private onTouchEnd(_e: TouchEvent): void {
    if (!this.touch.tracking) return;
    this.touch.tracking = false;

    const dy = this.touch.currentDeltaY;
    const dx = this.touch.currentDeltaX;
    const dt = Date.now() - this.touch.startTime;
    const velocityY = Math.abs(dy) / Math.max(dt, 1);
    const velocityX = Math.abs(dx) / Math.max(dt, 1);

    // Restore transitions
    this.carouselInner.style.transition = '';
    this.carouselInner.style.transform = '';
    this.carouselInner.style.opacity = '';
    this.metadataPanel.style.transition = '';
    this.metadataPanel.style.opacity = '';

    if (this.touch.locked === 'vertical') {
      const shouldAdvance = Math.abs(dy) > SWIPE_THRESHOLD || velocityY > VELOCITY_THRESHOLD;
      if (shouldAdvance) {
        if (dy < 0 && this.currentIndex < this.playlist.matches.length - 1) {
          this.advanceTo(this.currentIndex + 1);
        } else if (dy > 0 && this.currentIndex > 0) {
          this.advanceTo(this.currentIndex - 1);
        } else {
          // At boundary — snap back
          this.snapBack();
        }
      } else {
        // Not enough — snap back
        this.snapBack();
      }
    } else if (this.touch.locked === 'horizontal') {
      if (this.metadataOpen) {
        const shouldClose = dx > SWIPE_THRESHOLD || velocityX > VELOCITY_THRESHOLD;
        if (shouldClose) {
          this.closeMetadata();
        } else {
          this.openMetadata(); // snap back to open
        }
      } else {
        const shouldOpen = dx > SWIPE_THRESHOLD || velocityX > VELOCITY_THRESHOLD;
        if (shouldOpen) {
          this.openMetadata();
        } else {
          this.closeMetadata(); // snap back to closed
        }
      }
    }

    this.touch.currentDeltaY = 0;
    this.touch.currentDeltaX = 0;
  }

  private onTouchCancel(): void {
    this.touch.tracking = false;
    this.carouselInner.style.transition = '';
    this.carouselInner.style.transform = '';
    this.carouselInner.style.opacity = '';
    this.metadataPanel.style.transition = '';
    this.metadataPanel.style.opacity = '';
    this.touch.currentDeltaY = 0;
    this.touch.currentDeltaX = 0;
  }

  private snapBack(): void {
    // CSS transition handles the snap-back animation
    this.carouselInner.style.transform = '';
    this.carouselInner.style.opacity = '';
  }

  // ── Card loading ─────────────────────────────────────────────────────────

  private async loadCard(index: number): Promise<void> {
    const match = this.playlist.matches[index];
    if (!match) return;

    // Update header
    this.headerBar.innerHTML = `
      <span class="carousel-playlist-name">${escapeHtml(this.playlist.title)}</span>
      <span class="carousel-counter">${index + 1} of ${this.playlist.matches.length}</span>
    `;

    // Update score bar with placeholder
    this.scoreBar.innerHTML = `
      <span class="carousel-score-loading">Loading...</span>
    `;

    // Reset metadata panel
    this.metadataOpen = false;
    this.metadataPanel.style.transform = `translateX(${METADATA_PANEL_WIDTH}px)`;
    this.metadataPanel.style.opacity = '0';
    this.carouselInner.classList.remove('carousel-shifted');
    this.updateMetadataContent(match, null);

    // Hide countdown ring
    this.hideCountdownRing();

    // Clear auto-advance timer
    this.clearAutoAdvance();

    // Fetch replay
    let replay = this.preloadedReplays.get(index);
    if (!replay) {
      try {
        replay = await this.fetchReplay(match.match_id);
      } catch {
        this.scoreBar.innerHTML = `<span class="carousel-score-loading">Failed to load</span>`;
        return;
      }
    }

    if (!this.viewer) return;

    // Load into viewer
    this.viewer.loadReplay(replay);

    // Set up director mode for auto-play
    const densities = computeAllDensities(replay);
    this.directorSchedule = computeSpeedSchedule(densities, 30 as DurationPreset);
    this.directorState = createDirectorState();
    this.directorState.enabled = true;
    this.viewer.setDirectorMode(true);

    // Start playing
    this.viewer.togglePlay();
    this.startDirectorTick();

    // Update score bar
    this.updateScoreBar(match, replay);

    // Update event hint
    this.updateEventHint(replay);

    // Update metadata panel with full info
    this.updateMetadataContent(match, replay);

    // Preload next replay
    this.preloadNext(index + 1);
  }

  private async fetchReplay(matchId: string): Promise<Replay> {
    const urls = [
      `${R2_BASE}/replays/${matchId}.json.gz`,
      `${B2_FALLBACK}/replays/${matchId}.json.gz`,
    ];
    for (const url of urls) {
      try {
        const resp = await fetch(url);
        if (resp.ok) return await resp.json();
      } catch { /* try next */ }
    }
    const resp = await fetch(`/replays/${matchId}.json.gz`);
    if (!resp.ok) throw new Error(`Failed to fetch replay ${matchId}`);
    return resp.json();
  }

  private preloadNext(index: number): void {
    if (index >= this.playlist.matches.length) return;
    if (this.preloadedReplays.has(index)) return;
    const matchId = this.playlist.matches[index].match_id;
    this.fetchReplay(matchId)
      .then(r => this.preloadedReplays.set(index, r))
      .catch(() => { /* preload failure is non-critical */ });
  }

  private updateScoreBar(_match: PlaylistMatch, replay: Replay): void {
    const players = replay.players.map((p, i) => {
      const score = replay.result.scores?.[i] ?? '-';
      const won = replay.result.winner === i;
      return `<span class="carousel-player${won ? ' carousel-winner' : ''}">${escapeHtml(p.name)} ${score}</span>`;
    }).join(' <span class="carousel-vs">vs</span> ');
    this.scoreBar.innerHTML = players;
  }

  private updateEventHint(replay: Replay): void {
    const events = replay.turns.reduce((count, t) => count + (t.events?.length ?? 0), 0);
    const icons = events > 20 ? '⚔️💎🏰' : events > 5 ? '⚔️💎' : '⚔️';
    const totalTurns = replay.turns.length;
    const estSeconds = Math.round(totalTurns / 16);
    this.eventHint.innerHTML = `${icons} ~${estSeconds}s`;
  }

  private updateMetadataContent(match: PlaylistMatch, replay: Replay | null): void {
    const parts: string[] = [];
    parts.push(`<div class="carousel-meta-title">${escapeHtml(match.title ?? `Match ${match.order + 1}`)}</div>`);
    if (match.curation_tag) parts.push(`<div class="carousel-meta-tag">${escapeHtml(match.curation_tag)}</div>`);
    if (replay) {
      parts.push(`<div class="carousel-meta-row"><span>Turns</span><span>${replay.turns.length}</span></div>`);
      parts.push(`<div class="carousel-meta-row"><span>Map</span><span>${replay.map.rows}x${replay.map.cols}</span></div>`);
      if (replay.result.reason) parts.push(`<div class="carousel-meta-row"><span>End</span><span>${escapeHtml(replay.result.reason)}</span></div>`);
    }
    if (match.completed_at) {
      const d = new Date(match.completed_at);
      parts.push(`<div class="carousel-meta-row"><span>Date</span><span>${d.toLocaleDateString()}</span></div>`);
    }
    parts.push(`<button class="carousel-meta-watch-full" data-match-id="${match.match_id}">Watch Full Replay →</button>`);
    this.metadataPanel.innerHTML = parts.join('');

    const btn = this.metadataPanel.querySelector('.carousel-meta-watch-full');
    if (btn) {
      btn.addEventListener('click', () => {
        const id = (btn as HTMLElement).dataset.matchId!;
        this.destroy();
        window.location.hash = `/watch/replay?url=/replays/${id}.json.gz`;
      });
    }
  }

  // ── Auto-advance with countdown ring ─────────────────────────────────────

  private onReplayEnd(): void {
    if (this.currentIndex >= this.playlist.matches.length - 1) {
      // Last match — no auto-advance
      return;
    }

    const startTime = performance.now();
    const duration = this.autoAdvanceDelay;

    // Show countdown ring
    this.showCountdownRing();

    // Animate the countdown ring
    const animate = (now: number) => {
      const elapsed = now - startTime;
      const progress = Math.min(elapsed / duration, 1);

      // Update ring stroke-dashoffset
      const circle = this.countdownRing.querySelector('.carousel-countdown-circle') as SVGElement | null;
      if (circle) {
        const circumference = 2 * Math.PI * 14; // r=14
        circle.style.strokeDashoffset = String(circumference * (1 - progress));
      }

      // Update label
      const label = this.countdownRing.querySelector('.carousel-countdown-label');
      if (label) {
        const remaining = Math.ceil((duration - elapsed) / 1000);
        label.textContent = String(Math.max(remaining, 0));
      }

      if (progress < 1) {
        this.countdownAnimFrame = requestAnimationFrame(animate);
      }
    };
    this.countdownAnimFrame = requestAnimationFrame(animate);

    // Set the actual timer
    this.autoAdvanceTimer = setTimeout(() => {
      this.hideCountdownRing();
      if (this.currentIndex < this.playlist.matches.length - 1) {
        this.advanceTo(this.currentIndex + 1);
      }
    }, duration);

    // Tap anywhere on the ring area to cancel
    const cancelHandler = () => {
      this.clearAutoAdvance();
      this.hideCountdownRing();
      this.countdownRing.removeEventListener('click', cancelHandler);
    };
    this.countdownRing.addEventListener('click', cancelHandler);
  }

  private showCountdownRing(): void {
    this.countdownRing.style.display = 'flex';
    this.countdownRing.style.opacity = '1';
    const circle = this.countdownRing.querySelector('.carousel-countdown-circle') as SVGElement | null;
    if (circle) {
      const circumference = 2 * Math.PI * 14;
      circle.style.strokeDasharray = String(circumference);
      circle.style.strokeDashoffset = String(circumference);
    }
  }

  private hideCountdownRing(): void {
    this.countdownRing.style.opacity = '0';
    if (this.countdownAnimFrame !== null) {
      cancelAnimationFrame(this.countdownAnimFrame);
      this.countdownAnimFrame = null;
    }
    // Keep display for fade-out transition
    setTimeout(() => {
      this.countdownRing.style.display = 'none';
    }, 200);
  }

  private clearAutoAdvance(): void {
    if (this.autoAdvanceTimer) {
      clearTimeout(this.autoAdvanceTimer);
      this.autoAdvanceTimer = null;
    }
    if (this.countdownAnimFrame !== null) {
      cancelAnimationFrame(this.countdownAnimFrame);
      this.countdownAnimFrame = null;
    }
  }

  // ── Metadata panel ───────────────────────────────────────────────────────

  private openMetadata(): void {
    if (this.metadataOpen) return;
    this.metadataOpen = true;
    this.metadataPanel.style.transform = 'translateX(0)';
    this.metadataPanel.style.opacity = '1';
    this.carouselInner.classList.add('carousel-shifted');
  }

  private closeMetadata(): void {
    if (!this.metadataOpen) return;
    this.metadataOpen = false;
    this.metadataPanel.style.transform = `translateX(${METADATA_PANEL_WIDTH}px)`;
    this.metadataPanel.style.opacity = '0';
    this.carouselInner.classList.remove('carousel-shifted');
  }

  // ── Card transitions ─────────────────────────────────────────────────────

  private advanceTo(index: number): void {
    if (index < 0 || index >= this.playlist.matches.length) return;
    this.transitioning = true;
    const direction = index > this.currentIndex ? 1 : -1;
    this.currentIndex = index;

    this.clearAutoAdvance();
    this.hideCountdownRing();

    if (REDUCED_MOTION) {
      // Instant swap
      this.stopDirectorTick();
      this.loadCard(this.currentIndex).then(() => {
        this.transitioning = false;
      });
      return;
    }

    // Animate out
    this.carouselInner.classList.add(direction > 0 ? 'carousel-exit-up' : 'carousel-exit-down');

    setTimeout(() => {
      this.stopDirectorTick();
      this.carouselInner.classList.remove('carousel-exit-up', 'carousel-exit-down');
      this.carouselInner.classList.add(direction > 0 ? 'carousel-enter-up' : 'carousel-enter-down');

      this.loadCard(this.currentIndex).then(() => {
        setTimeout(() => {
          this.carouselInner.classList.remove('carousel-enter-up', 'carousel-enter-down');
          this.transitioning = false;
        }, TRANSITION_MS);
      });
    }, TRANSITION_MS / 2);
  }

  // ── Director tick ────────────────────────────────────────────────────────

  private startDirectorTick(): void {
    this.stopDirectorTick();
    const tick = () => {
      if (!this.directorState.enabled || !this.viewer) return;
      const now = performance.now();
      const turn = this.viewer.getTurn();
      const ms = tickDirectorSpeed(this.directorState, this.directorSchedule, turn, now);
      this.viewer.setDirectorSpeed(ms);
      this.directorAnimFrame = requestAnimationFrame(tick);
    };
    this.directorAnimFrame = requestAnimationFrame(tick);
  }

  private stopDirectorTick(): void {
    if (this.directorAnimFrame !== null) {
      cancelAnimationFrame(this.directorAnimFrame);
      this.directorAnimFrame = null;
    }
  }

  // ── Cleanup ──────────────────────────────────────────────────────────────

  destroy(): void {
    this.stopDirectorTick();
    this.clearAutoAdvance();
    if (this.viewer) {
      this.viewer.pause();
      this.viewer.destroy();
    }
    this.styleEl.remove();
    this.overlay.remove();
    document.body.style.overflow = '';
    this.onClose();
  }
}

// ── HTML template ──────────────────────────────────────────────────────────

const CAROUSEL_HTML = `
<div class="carousel-container">
  <div class="carousel-header">
    <span class="carousel-playlist-name"></span>
    <span class="carousel-counter"></span>
  </div>

  <div class="carousel-card">
    <canvas class="carousel-canvas"></canvas>

    <div class="carousel-score-bar"></div>
    <div class="carousel-event-hint"></div>
  </div>

  <div class="carousel-countdown-ring" style="display:none">
    <svg viewBox="0 0 36 36" class="carousel-countdown-svg">
      <circle class="carousel-countdown-bg" cx="18" cy="18" r="14"
              fill="none" stroke="rgba(255,255,255,0.15)" stroke-width="2.5"/>
      <circle class="carousel-countdown-circle" cx="18" cy="18" r="14"
              fill="none" stroke="rgba(59,130,246,0.9)" stroke-width="2.5"
              stroke-linecap="round"
              transform="rotate(-90 18 18)"/>
    </svg>
    <span class="carousel-countdown-label">3</span>
  </div>

  <div class="carousel-swipe-hint">↑ swipe for next</div>

  <div class="carousel-metadata-panel"></div>

  <button class="carousel-close-btn" aria-label="Close carousel">✕</button>
</div>
`;

// ── CSS ────────────────────────────────────────────────────────────────────

const CAROUSEL_CSS = `
.carousel-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: #000;
  display: flex;
  flex-direction: column;
  animation: carousel-fade-in 200ms ease-out;
}

@keyframes carousel-fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

.carousel-container {
  position: relative;
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.carousel-header {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 10;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: linear-gradient(to bottom, rgba(0,0,0,0.7) 0%, transparent 100%);
  color: rgba(255,255,255,0.85);
  font-size: 0.8rem;
  pointer-events: none;
}

.carousel-playlist-name {
  font-weight: 600;
}

.carousel-counter {
  opacity: 0.7;
}

.carousel-card {
  position: relative;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  transition: transform ${TRANSITION_MS}ms ease-in-out, opacity ${TRANSITION_MS}ms ease-in-out;
  will-change: transform, opacity;
  -webkit-user-select: none;
  user-select: none;
}

.carousel-card.carousel-shifted {
  transform: translateX(-${METADATA_PANEL_WIDTH}px);
}

.carousel-canvas {
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
  touch-action: none;
}

.carousel-score-bar {
  position: absolute;
  bottom: 80px;
  left: 0;
  right: 0;
  z-index: 10;
  text-align: center;
  padding: 8px 16px;
  background: linear-gradient(to top, rgba(0,0,0,0.6) 0%, transparent 100%);
  color: rgba(255,255,255,0.9);
  font-size: 0.95rem;
  font-weight: 600;
  pointer-events: none;
}

.carousel-vs {
  color: rgba(255,255,255,0.4);
  margin: 0 6px;
  font-size: 0.8rem;
}

.carousel-winner {
  color: #22c55e;
}

.carousel-score-loading {
  opacity: 0.5;
  font-weight: 400;
}

.carousel-event-hint {
  position: absolute;
  bottom: 60px;
  left: 0;
  right: 0;
  z-index: 10;
  text-align: center;
  padding: 4px 16px;
  color: rgba(255,255,255,0.5);
  font-size: 0.75rem;
  pointer-events: none;
}

/* ── Countdown ring (auto-advance indicator) ── */

.carousel-countdown-ring {
  position: absolute;
  bottom: 120px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 15;
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: opacity 200ms ease-out;
  cursor: pointer;
}

.carousel-countdown-svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.carousel-countdown-bg {
  /* static background ring */
}

.carousel-countdown-circle {
  transition: stroke-dashoffset 0.1s linear;
}

.carousel-countdown-label {
  position: relative;
  z-index: 1;
  color: rgba(255,255,255,0.9);
  font-size: 0.85rem;
  font-weight: 700;
  font-family: monospace;
  pointer-events: none;
}

/* ── Swipe hint ── */

.carousel-swipe-hint {
  position: absolute;
  bottom: 24px;
  left: 0;
  right: 0;
  z-index: 10;
  text-align: center;
  color: rgba(255,255,255,0.35);
  font-size: 0.75rem;
  padding: 8px;
  transition: opacity 1s ease-out;
  pointer-events: none;
}

.carousel-hint-fade {
  opacity: 0;
}

/* ── Metadata panel (revealed on horizontal swipe) ── */

.carousel-metadata-panel {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: ${METADATA_PANEL_WIDTH}px;
  z-index: 20;
  background: rgba(15, 15, 25, 0.95);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  padding: 60px 16px 16px;
  transform: translateX(${METADATA_PANEL_WIDTH}px);
  opacity: 0;
  transition: transform ${TRANSITION_MS}ms ease-in-out, opacity ${TRANSITION_MS}ms ease-in-out;
  color: rgba(255,255,255,0.85);
  font-size: 0.85rem;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

.carousel-meta-title {
  font-size: 1rem;
  font-weight: 600;
  margin-bottom: 8px;
}

.carousel-meta-tag {
  font-size: 0.7rem;
  color: #94a3b8;
  font-style: italic;
  margin-bottom: 12px;
}

.carousel-meta-row {
  display: flex;
  justify-content: space-between;
  padding: 6px 0;
  border-bottom: 1px solid rgba(255,255,255,0.08);
}

.carousel-meta-row span:first-child {
  color: rgba(255,255,255,0.5);
}

.carousel-meta-watch-full {
  display: block;
  width: 100%;
  margin-top: 16px;
  padding: 10px;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  text-align: center;
}

.carousel-meta-watch-full:active {
  opacity: 0.8;
}

/* ── Close button ── */

.carousel-close-btn {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 30;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgba(0,0,0,0.5);
  border: none;
  color: rgba(255,255,255,0.8);
  font-size: 1.2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.carousel-close-btn:active {
  background: rgba(0,0,0,0.8);
}

/* ── Exit/enter animations ── */

.carousel-exit-up {
  animation: carousel-slide-out-up ${TRANSITION_MS}ms ease-in forwards;
}

.carousel-exit-down {
  animation: carousel-slide-out-down ${TRANSITION_MS}ms ease-in forwards;
}

.carousel-enter-up {
  animation: carousel-slide-in-up ${TRANSITION_MS}ms ease-out forwards;
}

.carousel-enter-down {
  animation: carousel-slide-in-down ${TRANSITION_MS}ms ease-out forwards;
}

@keyframes carousel-slide-out-up {
  from { transform: translateY(0); opacity: 1; }
  to { transform: translateY(-100%); opacity: 0; }
}

@keyframes carousel-slide-out-down {
  from { transform: translateY(0); opacity: 1; }
  to { transform: translateY(100%); opacity: 0; }
}

@keyframes carousel-slide-in-up {
  from { transform: translateY(100%); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

@keyframes carousel-slide-in-down {
  from { transform: translateY(-100%); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

/* ── Reduced motion ── */

@media (prefers-reduced-motion: reduce) {
  .carousel-overlay {
    animation: none;
  }
  .carousel-card {
    transition: none !important;
  }
  .carousel-metadata-panel {
    transition: none !important;
  }
  .carousel-exit-up,
  .carousel-exit-down,
  .carousel-enter-up,
  .carousel-enter-down {
    animation: none !important;
  }
}
`;
