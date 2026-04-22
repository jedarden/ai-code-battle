import type { Replay, ReplayTurn, Position, ReplayBot, GameEvent, DebugInfo, ViewMode, EnrichedCommentary, TranscriptEntry, ReplayPlayer } from './types';

// Export TranscriptEntry type for use in other modules
export type { TranscriptEntry };

// ── Particle System (pooled, 100 objects, zero GC) ──────────────────────────────
interface Particle {
  x: number;
  y: number;
  vx: number;
  vy: number;
  alpha: number;
  color: string;
  lifetime: number;   // ms
  elapsed: number;    // ms
  active: boolean;
}

const PARTICLE_POOL_SIZE = 100;
const particlePool: Particle[] = Array.from({ length: PARTICLE_POOL_SIZE }, () => ({
  x: 0, y: 0, vx: 0, vy: 0, alpha: 1, color: '#fff', lifetime: 0, elapsed: 0, active: false,
}));

function borrowParticle(x: number, y: number, vx: number, vy: number, color: string, lifetime: number): Particle | null {
  for (const p of particlePool) {
    if (!p.active) {
      p.x = x; p.y = y; p.vx = vx; p.vy = vy;
      p.color = color; p.lifetime = lifetime; p.elapsed = 0;
      p.alpha = 1; p.active = true;
      return p;
    }
  }
  return null;
}

function tickParticles(dt: number): void {
  for (const p of particlePool) {
    if (!p.active) continue;
    p.elapsed += dt;
    if (p.elapsed >= p.lifetime) { p.active = false; continue; }
    p.x += p.vx * dt;
    p.y += p.vy * dt;
    p.alpha = 1 - p.elapsed / p.lifetime;
  }
}

function drawParticles(ctx: CanvasRenderingContext2D): void {
  for (const p of particlePool) {
    if (!p.active) continue;
    ctx.globalAlpha = p.alpha;
    ctx.fillStyle = p.color;
    ctx.beginPath();
    ctx.arc(p.x, p.y, 2, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.globalAlpha = 1;
}

// ── One-shot effect slots (reusable, max 20 concurrent) ─────────────────────────
interface FloatText  { x: number; y: number; text: string; color: string; elapsed: number; lifetime: number; active: boolean; }
interface Shockwave  { x: number; y: number; radius: number; maxRadius: number; color: string; elapsed: number; lifetime: number; active: boolean; }
interface SpawnGlow  { x: number; y: number; color: string; elapsed: number; lifetime: number; active: boolean; }
interface Trail      { x: number; y: number; prevX: number; prevY: number; color: string; alpha: number; active: boolean; }

const MAX_EFFECTS = 20;
const floatTexts: FloatText[]  = Array.from({ length: MAX_EFFECTS }, () => ({ x: 0, y: 0, text: '', color: '', elapsed: 0, lifetime: 0, active: false }));
const shockwaves: Shockwave[]  = Array.from({ length: MAX_EFFECTS }, () => ({ x: 0, y: 0, radius: 0, maxRadius: 0, color: '', elapsed: 0, lifetime: 0, active: false }));
const spawnGlows: SpawnGlow[]  = Array.from({ length: MAX_EFFECTS }, () => ({ x: 0, y: 0, color: '', elapsed: 0, lifetime: 0, active: false }));
const trails: Trail[]          = Array.from({ length: MAX_EFFECTS }, () => ({ x: 0, y: 0, prevX: 0, prevY: 0, color: '', alpha: 0, active: false }));

function borrowSlot<T extends { active: boolean }>(arr: T[]): T | null {
  for (const item of arr) { if (!item.active) return item; }
  return null;
}

function tickEffects(dt: number): void {
  for (const e of floatTexts)  { if (!e.active) continue; e.elapsed += dt; e.y -= 20 * dt / 1000; if (e.elapsed >= e.lifetime) e.active = false; }
  for (const e of shockwaves)  { if (!e.active) continue; e.elapsed += dt; if (e.elapsed >= e.lifetime) e.active = false; }
  for (const e of spawnGlows)  { if (!e.active) continue; e.elapsed += dt; if (e.elapsed >= e.lifetime) e.active = false; }
  for (const e of trails)      { if (!e.active) continue; e.alpha -= dt / 150; if (e.alpha <= 0) e.active = false; }
}

function drawEffects(ctx: CanvasRenderingContext2D): void {
  // Float texts
  for (const e of floatTexts) {
    if (!e.active) continue;
    const t = e.elapsed / e.lifetime;
    ctx.globalAlpha = 1 - t;
    ctx.fillStyle = e.color;
    ctx.font = 'bold 11px monospace';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(e.text, e.x, e.y);
  }

  // Shockwaves
  for (const e of shockwaves) {
    if (!e.active) continue;
    const t = e.elapsed / e.lifetime;
    const r = e.maxRadius * t;
    ctx.globalAlpha = 0.6 * (1 - t);
    ctx.strokeStyle = e.color;
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.arc(e.x, e.y, r, 0, Math.PI * 2);
    ctx.stroke();
  }

  // Spawn glows
  for (const e of spawnGlows) {
    if (!e.active) continue;
    const t = e.elapsed / e.lifetime;
    const r = 12;
    const grad = ctx.createRadialGradient(e.x, e.y, 0, e.x, e.y, r * (1 + t));
    grad.addColorStop(0, e.color + 'aa');
    grad.addColorStop(1, e.color + '00');
    ctx.globalAlpha = 1 - t;
    ctx.fillStyle = grad;
    ctx.beginPath();
    ctx.arc(e.x, e.y, r * (1 + t), 0, Math.PI * 2);
    ctx.fill();
  }

  // Motion trails
  for (const e of trails) {
    if (!e.active) continue;
    ctx.globalAlpha = e.alpha * 0.4;
    ctx.strokeStyle = e.color;
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(e.prevX, e.prevY);
    ctx.lineTo(e.x, e.y);
    ctx.stroke();
  }

  ctx.globalAlpha = 1;
}

// Win probability point for sparkline
export interface WinProbPoint {
  turn: number;
  probs: number[];  // one probability per player (0.0–1.0)
}

export interface CriticalMomentMarker {
  turn: number;
  delta: number;
  description: string;
}

// Default player colors for sparkline (matches DEFAULT_PLAYER_COLORS)
const SPARKLINE_COLORS = [
  '#3b82f6', // Blue
  '#ef4444', // Red
  '#22c55e', // Green
  '#f59e0b', // Amber
  '#8b5cf6', // Purple
  '#06b6d4', // Cyan
  '#ec4899', // Pink
  '#f97316', // Orange
];

// Render win probability sparkline to canvas
export function renderWinProbSparkline(
  ctx: CanvasRenderingContext2D,
  points: WinProbPoint[],
  currentTurn: number,
  options: {
    width: number;
    height: number;
    playerColors?: string[];
    criticalMoments?: CriticalMomentMarker[];
  },
): void {
  const {
    width, height,
    playerColors = SPARKLINE_COLORS,
    criticalMoments = [],
  } = options;
  const padding = { top: 8, bottom: 8, left: 4, right: 4 };
  const chartW = width - padding.left - padding.right;
  const chartH = height - padding.top - padding.bottom;

  if (points.length < 2) {
    ctx.fillStyle = '#475569';
    ctx.fillRect(0, 0, width, height);
    return;
  }

  // Clear
  ctx.fillStyle = '#1e293b';
  ctx.fillRect(0, 0, width, height);

  const maxTurn = points[points.length - 1].turn;
  const numPlayers = points[0].probs.length;

  const x = (turn: number) => padding.left + (turn / maxTurn) * chartW;
  const y = (prob: number) => padding.top + chartH * (1 - prob);

  // 50% baseline
  const midY = y(0.5);
  ctx.strokeStyle = '#475569';
  ctx.lineWidth = 1;
  ctx.setLineDash([3, 3]);
  ctx.beginPath();
  ctx.moveTo(padding.left, midY);
  ctx.lineTo(width - padding.right, midY);
  ctx.stroke();
  ctx.setLineDash([]);

  // 0% and 100% labels
  ctx.fillStyle = '#475569';
  ctx.font = '8px monospace';
  ctx.textAlign = 'right';
  ctx.fillText('100%', padding.left + 28, padding.top + 6);
  ctx.fillText('0%', padding.left + 22, height - padding.bottom - 1);

  // Critical moment markers — dashed vertical lines with delta labels
  for (const moment of criticalMoments) {
    const mx = x(moment.turn);
    const markerColor = moment.delta > 0
      ? playerColors[0] ?? SPARKLINE_COLORS[0]
      : playerColors[1] ?? SPARKLINE_COLORS[1];

    ctx.strokeStyle = markerColor + 'aa';
    ctx.lineWidth = 1.5;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(mx, padding.top);
    ctx.lineTo(mx, height - padding.bottom);
    ctx.stroke();
    ctx.setLineDash([]);

    // Small diamond at midpoint
    const my = height / 2;
    const s = 3;
    ctx.fillStyle = markerColor;
    ctx.beginPath();
    ctx.moveTo(mx, my - s);
    ctx.lineTo(mx + s, my);
    ctx.lineTo(mx, my + s);
    ctx.lineTo(mx - s, my);
    ctx.closePath();
    ctx.fill();

    // Delta label near top
    const label = `${moment.delta > 0 ? '+' : ''}${(moment.delta * 100).toFixed(0)}%`;
    ctx.fillStyle = markerColor;
    ctx.font = '9px monospace';
    ctx.textAlign = 'center';
    ctx.fillText(label, Math.max(18, Math.min(width - 18, mx)), padding.top + 7);
  }

  // Area fill for first two players (creates the visual gradient)
  if (numPlayers >= 2) {
    ctx.beginPath();
    ctx.moveTo(padding.left, y(points[0].probs[0]));
    for (const pt of points) {
      ctx.lineTo(x(pt.turn), y(pt.probs[0]));
    }
    ctx.lineTo(width - padding.right, y(points[points.length - 1].probs[1]));
    for (let i = points.length - 1; i >= 0; i--) {
      ctx.lineTo(x(points[i].turn), y(points[i].probs[1]));
    }
    ctx.closePath();
    const grad = ctx.createLinearGradient(0, padding.top, 0, height - padding.bottom);
    grad.addColorStop(0, (playerColors[0] ?? SPARKLINE_COLORS[0]) + '33');
    grad.addColorStop(0.5, 'transparent');
    grad.addColorStop(1, (playerColors[1] ?? SPARKLINE_COLORS[1]) + '33');
    ctx.fillStyle = grad;
    ctx.fill();
  }

  // Draw a line per player
  for (let p = numPlayers - 1; p >= 0; p--) {
    const color = playerColors[p] ?? SPARKLINE_COLORS[p % SPARKLINE_COLORS.length];
    ctx.beginPath();
    for (let i = 0; i < points.length; i++) {
      const pt = points[i];
      if (i === 0) ctx.moveTo(x(pt.turn), y(pt.probs[p]));
      else ctx.lineTo(x(pt.turn), y(pt.probs[p]));
    }
    ctx.strokeStyle = color;
    ctx.lineWidth = p === 0 ? 2 : 1.5;
    if (p > 1) ctx.setLineDash([4, 3]);
    ctx.stroke();
    ctx.setLineDash([]);
  }

  // Current turn marker
  const curX = x(currentTurn);
  ctx.strokeStyle = '#f8fafc';
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.moveTo(curX, padding.top);
  ctx.lineTo(curX, height - padding.bottom);
  ctx.stroke();

  // Current probability dots for all players
  const curPt = points.find(p => p.turn >= currentTurn) ?? points[points.length - 1];
  if (curPt) {
    for (let p = 0; p < curPt.probs.length; p++) {
      const color = playerColors[p] ?? SPARKLINE_COLORS[p % SPARKLINE_COLORS.length];
      ctx.beginPath();
      ctx.arc(curX, y(curPt.probs[p]), 4, 0, Math.PI * 2);
      ctx.fillStyle = color;
      ctx.fill();
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 1;
      ctx.stroke();
    }
  }
}

// ── Accessibility: Paul Tol's color-blind safe palette ──────────────────────────
// These colors are designed to be distinguishable for all color vision deficiencies
// See: https://personal.sron.nl/~pault/
const TOL_PALETTE = [
  '#332288', // Indigo (player 0)
  '#88ccee', // Cyan (player 1)
  '#44aa99', // Teal (player 2)
  '#117733', // Green (player 3)
  '#999933', // Olive (player 4)
  '#ddcc77', // Sand (player 5)
];

// High contrast version for accessibility mode
const HIGH_CONTRAST_PALETTE = [
  '#0000ff', // Blue (player 0)
  '#ff0000', // Red (player 1)
  '#00ff00', // Green (player 2)
  '#ff00ff', // Magenta (player 3)
  '#00ffff', // Cyan (player 4)
  '#ffff00', // Yellow (player 5)
];

// Default palette (original - not color-blind safe, used for backwards compat)
const DEFAULT_PLAYER_COLORS = [
  '#3b82f6', // Blue (player 0)
  '#ef4444', // Red (player 1)
  '#22c55e', // Green (player 2)
  '#f59e0b', // Amber (player 3)
  '#8b5cf6', // Purple (player 4)
  '#06b6d4', // Cyan (player 5)
  '#ec4899', // Pink (player 6)
  '#f97316', // Orange (player 7)
];

// Shape types for each player (0-7) - allows shape + color identification
type PlayerShape = 'circle' | 'square' | 'triangle' | 'diamond' | 'pentagon' | 'hexagon' | 'star' | 'cross';
const PLAYER_SHAPES: PlayerShape[] = ['circle', 'square', 'triangle', 'diamond', 'pentagon', 'hexagon', 'star', 'cross'];

const NEUTRAL_COLOR = '#6b7280'; // Gray
const WALL_COLOR = '#4b5563'; // Medium gray - clearly distinct from background
const ENERGY_COLOR = '#fbbf24'; // Yellow
const BACKGROUND_COLOR = '#0f172a'; // Dark navy - open tiles
const GRID_COLOR = '#1e293b'; // Subtle grid lines

// High contrast versions
const HIGH_CONTRAST_NEUTRAL = '#888888';
const HIGH_CONTRAST_WALL = '#444444';
const HIGH_CONTRAST_ENERGY = '#ffff00';
const HIGH_CONTRAST_BACKGROUND = '#000000';
const HIGH_CONTRAST_GRID = '#666666';

export interface ViewerOptions {
  cellSize?: number;
  showGrid?: boolean;
  fogOfWarPlayer?: number | null; // null = disabled, number = player perspective
  animationSpeed?: number; // ms per frame
  // Accessibility options
  colorBlindSafe?: boolean;  // Use Tol palette (default: true)
  highContrast?: boolean;    // High contrast mode
  showShapes?: boolean;      // Draw different shapes per player (default: true)
  reducedMotion?: boolean;   // Skip animations (auto-detected from prefers-reduced-motion)
  // View modes
  viewMode?: ViewMode;
  showDebug?: boolean;       // Show debug telemetry overlay
}

// Accessibility mode configuration
export interface AccessibilitySettings {
  colorBlindSafe: boolean;
  highContrast: boolean;
  showShapes: boolean;
  reducedMotion: boolean;
}

// Default accessibility settings
export const DEFAULT_ACCESSIBILITY: AccessibilitySettings = {
  colorBlindSafe: true,
  highContrast: false,
  showShapes: true,
  reducedMotion: typeof window !== 'undefined' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches,
};

export class ReplayViewer {
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  private replay: Replay | null = null;
  private currentTurn: number = 0;
  private isPlaying: boolean = false;
  private animationFrame: number | null = null;
  private cellSize: number;
  private showGrid: boolean;
  private fogOfWarPlayer: number | null;
  private animationSpeed: number;
  private accessibility: AccessibilitySettings;
  private viewMode: ViewMode;
  private showDebug: boolean;
  private debugPlayerEnabled: Map<number, boolean> = new Map();
  private screenReaderRegion: HTMLElement | null = null;

  // Animation state
  private turnStartTime: number = 0;
  private lastRenderTime: number = 0;
  private renderLoopRunning: boolean = false;
  // Per-bot interpolated positions: map botId -> {renderX, renderY}
  private botRenderPos: Map<number, { x: number; y: number }> = new Map();
  // Per-bot previous turn positions (for lerp source)
  private botPrevPos: Map<number, { x: number; y: number }> = new Map();
  // Bots that spawned this turn (for spawn animation)
  private spawnedBotIds: Set<number> = new Set();
  // Global idle pulse phase (radians)
  private idlePhase: number = 0;

  // View mode cross-fade transition state (§16.11)
  private viewTransition: {
    active: boolean;
    fromMode: ViewMode;
    toMode: ViewMode;
    startTime: number;
    duration: number; // ms
    offscreenFrom: HTMLCanvasElement | null;
    offscreenTo: HTMLCanvasElement | null;
  } = {
    active: false,
    fromMode: 'standard',
    toMode: 'standard',
    startTime: 0,
    duration: 400,
    offscreenFrom: null,
    offscreenTo: null,
  };

  // Follow camera state (§16.12)
  private followPlayer: number | null = null;
  private cameraCenterX: number = 0;
  private cameraCenterY: number = 0;
  private cameraTargetCenterX: number = 0;
  private cameraTargetCenterY: number = 0;
  private cameraZoom: number = 1;
  private cameraTargetZoom: number = 1;
  private followZoom: number = 3;

  // Event callbacks
  public onTurnChange?: (turn: number) => void;
  public onPlayStateChange?: (playing: boolean) => void;
  public onReplayLoad?: (replay: Replay) => void;
  public onCommentaryChange?: (entry: { turn: number; text: string; type: string } | null) => void;
  public onDebugChange?: (debug: Record<number, DebugInfo> | null) => void;
  public onFollowChange?: (player: number | null) => void;

  // Director mode: external speed override from director controller
  private directorEnabled: boolean = false;
  private directorMsPerTurn: number = 500;

  // Enriched commentary state (§13.3)
  private commentary: EnrichedCommentary | null = null;
  private commentaryEnabled: boolean = true;

  // Annotation overlay state (§16.8)
  private annotations: Array<{ turn: number; type: string; position?: Position }> = [];

  constructor(canvas: HTMLCanvasElement, options: ViewerOptions = {}) {
    this.canvas = canvas;
    const ctx = canvas.getContext('2d');
    if (!ctx) throw new Error('Could not get 2D context');
    this.ctx = ctx;

    this.cellSize = options.cellSize ?? 10;
    this.showGrid = options.showGrid ?? true;
    this.fogOfWarPlayer = options.fogOfWarPlayer ?? null;
    this.animationSpeed = options.animationSpeed ?? 100;

    // Initialize accessibility settings
    this.accessibility = {
      colorBlindSafe: options.colorBlindSafe ?? DEFAULT_ACCESSIBILITY.colorBlindSafe,
      highContrast: options.highContrast ?? DEFAULT_ACCESSIBILITY.highContrast,
      showShapes: options.showShapes ?? DEFAULT_ACCESSIBILITY.showShapes,
      reducedMotion: options.reducedMotion ??
        (options.reducedMotion ?? DEFAULT_ACCESSIBILITY.reducedMotion),
    };

    // Initialize view mode
    this.viewMode = options.viewMode ?? 'standard';
    this.showDebug = options.showDebug ?? false;

    // Create screen reader region for announcements
    this.createScreenReaderRegion();

    this.render = this.render.bind(this);
  }

  // Create or get the aria-live region for screen reader announcements
  private createScreenReaderRegion(): void {
    const existingRegion = document.getElementById('acb-screen-reader-region');
    if (existingRegion) {
      this.screenReaderRegion = existingRegion;
      return;
    }

    const region = document.createElement('div');
    region.id = 'acb-screen-reader-region';
    region.setAttribute('role', 'status');
    region.setAttribute('aria-live', 'polite');
    region.setAttribute('aria-atomic', 'true');
    region.style.cssText = 'position:absolute;left:-10000px;width:1px;height:1px;overflow:hidden;';
    document.body.appendChild(region);
    this.screenReaderRegion = region;
  }

  loadReplay(replay: Replay): void {
    this.replay = replay;
    this.currentTurn = 0;
    this.turnStartTime = performance.now();
    this.botPrevPos.clear();
    this.botRenderPos.clear();
    this.spawnedBotIds.clear();
    this.idlePhase = 0;

    // Resize canvas to fit the grid
    this.resizeCanvas();

    // Initialize follow camera to full grid view
    const mapW = replay.map.cols * this.cellSize;
    const mapH = replay.map.rows * this.cellSize;
    this.cameraCenterX = mapW / 2;
    this.cameraCenterY = mapH / 2;
    this.cameraTargetCenterX = mapW / 2;
    this.cameraTargetCenterY = mapH / 2;
    this.cameraZoom = 1;
    this.cameraTargetZoom = 1;
    this.followPlayer = null;

    // Render initial state
    this.render();

    // Start the continuous render loop
    this.startRenderLoop();

    if (this.onReplayLoad) this.onReplayLoad(replay);
    this.fireDebugForTurn(0);
  }

  private resizeCanvas(): void {
    if (!this.replay) return;
    const { rows, cols } = this.replay.map;
    this.canvas.width = cols * this.cellSize;
    // Extra space below map for score overlay (not overlapping the playfield)
    const overlayHeight = 8 * 2 + 20 * (this.replay?.players?.length ?? 2) + 8;
    this.canvas.height = rows * this.cellSize + overlayHeight;
  }

  private posKey(pos: Position): string {
    return `${pos.row},${pos.col}`;
  }

  setTurn(turn: number): void {
    if (!this.replay) return;
    const newTurn = Math.max(0, Math.min(turn, this.replay.turns.length - 1));
    if (newTurn !== this.currentTurn) {
      this.advanceTurn(newTurn);
      // Ensure render loop is running
      this.startRenderLoop();
    }
  }

  getTurn(): number {
    return this.currentTurn;
  }

  getTotalTurns(): number {
    return this.replay?.turns.length ?? 0;
  }

  play(): void {
    if (this.isPlaying || !this.replay) return;
    this.isPlaying = true;
    this.turnStartTime = performance.now();
    this.startRenderLoop();
    if (this.onPlayStateChange) this.onPlayStateChange(true);
  }

  pause(): void {
    this.isPlaying = false;
    // Keep render loop running for idle animations and particles
    if (this.onPlayStateChange) this.onPlayStateChange(false);
  }

  togglePlay(): void {
    if (this.isPlaying) {
      this.pause();
    } else {
      this.play();
    }
  }

  setSpeed(msPerFrame: number): void {
    this.animationSpeed = Math.max(10, Math.min(2000, msPerFrame));
  }

  getSpeed(): number {
    return this.animationSpeed;
  }

  // Director mode: when enabled, tickDirectorSpeed overrides animationSpeed
  setDirectorMode(enabled: boolean): void {
    this.directorEnabled = enabled;
  }

  isDirectorMode(): boolean {
    return this.directorEnabled;
  }

  // Called externally by the director controller each tick to set eased speed
  setDirectorSpeed(msPerTurn: number): void {
    this.directorMsPerTurn = Math.max(10, Math.min(2000, msPerTurn));
  }

  getIsPlaying(): boolean {
    return this.isPlaying;
  }

  setFogOfWar(player: number | null): void {
    this.fogOfWarPlayer = player;
    this.render();
  }

  getFogOfWar(): number | null {
    return this.fogOfWarPlayer;
  }

  // ── Accessibility Controls ─────────────────────────────────────────────────────

  setAccessibility(settings: Partial<AccessibilitySettings>): void {
    this.accessibility = { ...this.accessibility, ...settings };
    this.render();
  }

  getAccessibility(): AccessibilitySettings {
    return { ...this.accessibility };
  }

  // ── View Mode Controls ─────────────────────────────────────────────────────

  setViewMode(mode: ViewMode): void {
    if (mode === this.viewMode) return;

    // Snap instantly when reduced motion is preferred
    if (this.accessibility.reducedMotion) {
      this.viewMode = mode;
      this.render();
      return;
    }

    // Capture the current canvas state as the "from" buffer
    const w = this.canvas.width;
    const h = this.canvas.height;
    if (w === 0 || h === 0) {
      this.viewMode = mode;
      this.render();
      return;
    }

    const fromBuf = document.createElement('canvas');
    fromBuf.width = w;
    fromBuf.height = h;
    fromBuf.getContext('2d')!.drawImage(this.canvas, 0, 0);

    // Switch mode and render the "to" state into an offscreen buffer
    const prevMode = this.viewMode;
    this.viewMode = mode;

    const toBuf = document.createElement('canvas');
    toBuf.width = w;
    toBuf.height = h;
    const toCtx = toBuf.getContext('2d')!;

    // Render the new mode into the offscreen buffer
    const origCtx = this.ctx;
    (this as any).ctx = toCtx;
    this.renderViewLayer();
    (this as any).ctx = origCtx;

    // Start transition
    this.viewTransition = {
      active: true,
      fromMode: prevMode,
      toMode: mode,
      startTime: performance.now(),
      duration: 400,
      offscreenFrom: fromBuf,
      offscreenTo: toBuf,
    };

    this.startRenderLoop();
  }

  getViewMode(): ViewMode {
    return this.viewMode;
  }

  setCellSize(size: number): void {
    this.cellSize = Math.max(4, Math.min(20, Math.round(size)));
    if (this.replay) {
      this.resizeCanvas();
      this.render();
    }
  }

  getCellSize(): number {
    return this.cellSize;
  }

  setShowDebug(show: boolean): void {
    this.showDebug = show;
    this.render();
  }

  getShowDebug(): boolean {
    return this.showDebug;
  }

  setDebugPlayerEnabled(player: number, enabled: boolean): void {
    this.debugPlayerEnabled.set(player, enabled);
    this.render();
  }

  getDebugPlayerEnabled(player: number): boolean {
    return this.debugPlayerEnabled.get(player) ?? true;
  }

  getDebugForCurrentTurn(): Record<number, DebugInfo> | null {
    return this.replay?.turns[this.currentTurn]?.debug ?? null;
  }

  // ── Annotation Overlay (§16.8) ─────────────────────────────────────────────────

  setAnnotations(anns: Array<{ turn: number; type: string; position?: Position }>): void {
    this.annotations = anns;
    this.render();
  }

  // ── Enriched Commentary Controls (§13.3) ──────────────────────────────────────

  setCommentary(commentary: EnrichedCommentary | null): void {
    this.commentary = commentary;
    this.fireCommentaryForTurn(this.currentTurn);
  }

  getCommentary(): EnrichedCommentary | null {
    return this.commentary;
  }

  setCommentaryEnabled(enabled: boolean): void {
    this.commentaryEnabled = enabled;
    this.fireCommentaryForTurn(this.currentTurn);
  }

  getCommentaryEnabled(): boolean {
    return this.commentaryEnabled;
  }

  // ── Follow Camera Controls (§16.12) ────────────────────────────────────────────

  setFollowPlayer(player: number | null): void {
    if (player === this.followPlayer) return;
    this.followPlayer = player;

    if (player === null && this.replay) {
      // Target: full grid view
      const mapW = this.replay.map.cols * this.cellSize;
      const mapH = this.replay.map.rows * this.cellSize;
      this.cameraTargetCenterX = mapW / 2;
      this.cameraTargetCenterY = mapH / 2;
      this.cameraTargetZoom = 1;
    }

    if (this.onFollowChange) this.onFollowChange(player);
    this.startRenderLoop();
  }

  getFollowPlayer(): number | null {
    return this.followPlayer;
  }

  setFollowZoom(zoom: number): void {
    this.followZoom = Math.max(1, Math.min(10, zoom));
  }

  getFollowZoom(): number {
    return this.followZoom;
  }

  private updateCamera(): void {
    if (!this.replay) return;
    const { rows, cols } = this.replay.map;
    const mapW = cols * this.cellSize;
    const mapH = rows * this.cellSize;

    if (this.followPlayer !== null) {
      const turnData = this.replay.turns[this.currentTurn];
      if (turnData) {
        const playerBots = turnData.bots.filter(b => b.owner === this.followPlayer && b.alive);

        if (playerBots.length === 0) {
          // No living bots — gradually return to full view
          this.cameraTargetCenterX = mapW / 2;
          this.cameraTargetCenterY = mapH / 2;
          this.cameraTargetZoom = 1;
        } else {
          // Toroidal centroid via circular mean (handles wrap-around groups)
          let sinR = 0, cosR = 0, sinC = 0, cosC = 0;
          for (const bot of playerBots) {
            const aR = (2 * Math.PI * bot.position.row) / rows;
            const aC = (2 * Math.PI * bot.position.col) / cols;
            sinR += Math.sin(aR);
            cosR += Math.cos(aR);
            sinC += Math.sin(aC);
            cosC += Math.cos(aC);
          }
          sinR /= playerBots.length;
          cosR /= playerBots.length;
          sinC /= playerBots.length;
          cosC /= playerBots.length;

          const centroidRow = ((rows * Math.atan2(sinR, cosR) / (2 * Math.PI)) % rows + rows) % rows;
          const centroidCol = ((cols * Math.atan2(sinC, cosC) / (2 * Math.PI)) % cols + cols) % cols;

          this.cameraTargetCenterX = centroidCol * this.cellSize + this.cellSize / 2;
          this.cameraTargetCenterY = centroidRow * this.cellSize + this.cellSize / 2;

          // Max distance from centroid (in pixels)
          let maxDist = 0;
          for (const bot of playerBots) {
            const d = this.toroidalDistance(centroidRow, centroidCol, bot.position.row, bot.position.col);
            maxDist = Math.max(maxDist, d);
          }
          maxDist *= this.cellSize;

          // Zoom to fit all bots + 8-tile margin
          const margin = 8 * this.cellSize;
          const viewRadius = maxDist + margin;
          const canvasW = this.canvas.width;
          const fitZoom = Math.min(canvasW, mapH) / (2 * viewRadius);

          // Clamp: followZoom default (3x), fitZoom when spread, max 15x15 tiles visible, min full grid
          const maxZoom = Math.min(canvasW / (15 * this.cellSize), mapH / (15 * this.cellSize));
          this.cameraTargetZoom = Math.max(1, Math.min(maxZoom, Math.min(this.followZoom, fitZoom)));
        }
      }
    } else {
      this.cameraTargetCenterX = mapW / 2;
      this.cameraTargetCenterY = mapH / 2;
      this.cameraTargetZoom = 1;
    }

    // Smooth lerp toward targets (toroidal-aware for center coordinates)
    const panFactor = this.accessibility.reducedMotion ? 1 : 0.15;
    const zoomFactor = this.accessibility.reducedMotion ? 1 : 0.10;

    this.cameraCenterX = this.lerpToroidal(this.cameraCenterX, this.cameraTargetCenterX, panFactor, mapW);
    this.cameraCenterY = this.lerpToroidal(this.cameraCenterY, this.cameraTargetCenterY, panFactor, mapH);
    this.cameraZoom += (this.cameraTargetZoom - this.cameraZoom) * zoomFactor;
  }

  private lerpToroidal(current: number, target: number, factor: number, size: number): number {
    let delta = target - current;
    if (delta > size / 2) delta -= size;
    if (delta < -size / 2) delta += size;
    const result = current + delta * factor;
    return ((result % size) + size) % size;
  }

  private applyCameraTransform(): void {
    const { ctx } = this;
    if (!this.replay) return;

    const mapH = this.replay.map.rows * this.cellSize;
    const canvasW = this.canvas.width;

    ctx.translate(canvasW / 2, mapH / 2);
    ctx.scale(this.cameraZoom, this.cameraZoom);
    ctx.translate(-this.cameraCenterX, -this.cameraCenterY);
  }

  // Get the active commentary entry for a given turn
  getCommentaryForTurn(turn: number): { turn: number; text: string; type: string } | null {
    if (!this.commentary || !this.commentaryEnabled) return null;
    // Find the most recent entry at or before this turn
    let best: { turn: number; text: string; type: string } | null = null;
    for (const entry of this.commentary.entries) {
      if (entry.turn <= turn) {
        best = entry;
      } else {
        break;
      }
    }
    return best;
  }

  private fireCommentaryForTurn(turn: number): void {
    if (this.onCommentaryChange) {
      this.onCommentaryChange(this.getCommentaryForTurn(turn));
    }
  }

  private fireDebugForTurn(turn: number): void {
    if (this.onDebugChange) {
      const turnData = this.replay?.turns[turn];
      this.onDebugChange(turnData?.debug ?? null);
    }
  }

  destroy(): void {
    this.stopRenderLoop();
    this.isPlaying = false;
  }

  // Get the active color palette based on accessibility settings
  private getPlayerColors(): string[] {
    if (this.accessibility.highContrast) {
      return HIGH_CONTRAST_PALETTE;
    }
    return this.accessibility.colorBlindSafe ? TOL_PALETTE : DEFAULT_PLAYER_COLORS;
  }

  // Get background color based on accessibility mode
  private getBackgroundColor(): string {
    return this.accessibility.highContrast ? HIGH_CONTRAST_BACKGROUND : BACKGROUND_COLOR;
  }

  // Get wall color based on accessibility mode
  private getWallColor(): string {
    return this.accessibility.highContrast ? HIGH_CONTRAST_WALL : WALL_COLOR;
  }

  // Get energy color based on accessibility mode
  private getEnergyColor(): string {
    return this.accessibility.highContrast ? HIGH_CONTRAST_ENERGY : ENERGY_COLOR;
  }

  // Get grid color based on accessibility mode
  private getGridColor(): string {
    return this.accessibility.highContrast ? HIGH_CONTRAST_GRID : GRID_COLOR;
  }

  // Announce events to screen readers
  private announceToScreenReader(message: string): void {
    if (this.screenReaderRegion) {
      this.screenReaderRegion.textContent = message;
    }
  }

  // Generate text description of turn events for screen readers
  private generateTurnDescription(events: GameEvent[]): string {
    if (events.length === 0) {
      return `Turn ${this.currentTurn}: No events.`;
    }

    const descriptions = events.map(e => {
      const details = e.details as Record<string, unknown>;
      switch (e.type) {
        case 'bot_died':
          return `Bot ${(details as { bot_id: number }).bot_id} was destroyed`;
        case 'bot_spawned':
          return `New bot ${(details as { bot_id: number }).bot_id} spawned`;
        case 'energy_collected':
          return 'Energy collected';
        case 'core_captured':
          return `Core captured by player ${(details as { new_owner: number }).new_owner}`;
        case 'core_destroyed':
          return 'Core destroyed';
        default:
          return e.type.replace(/_/g, ' ');
      }
    });

    return `Turn ${this.currentTurn}: ${descriptions.join(', ')}.`;
  }

  // ── Transcript Generation (§15.3 Screen Reader Transcript) ──────────────────────

  /**
   * Generate a detailed turn-by-turn transcript for screen readers.
   * Returns an array of transcript entries, one per turn.
   */
  generateTranscript(): TranscriptEntry[] {
    if (!this.replay) return [];

    const transcript: TranscriptEntry[] = [];
    const { players, win_prob } = this.replay;

    for (let turnIdx = 0; turnIdx < this.replay.turns.length; turnIdx++) {
      const turn = this.replay.turns[turnIdx];
      const entry = this.generateTurnTranscript(turnIdx, turn, players, win_prob);
      transcript.push(entry);
    }

    return transcript;
  }

  /**
   * Generate a detailed transcript entry for a single turn.
   */
  private generateTurnTranscript(
    turnIdx: number,
    turn: ReplayTurn,
    players: ReplayPlayer[],
    winProb?: number[][]
  ): TranscriptEntry {
    const parts: string[] = [];

    // Turn header
    parts.push(`Turn ${turnIdx}:`);

    // Player moves summary
    const moveSummaries = this.summarizePlayerMoves(turn, turnIdx, players);
    if (moveSummaries.length > 0) {
      parts.push(moveSummaries.join('. '));
    }

    // Combat events
    const combatSummary = this.summarizeCombatEvents(turn);
    if (combatSummary) {
      parts.push(combatSummary);
    }

    // Core captures
    const captureSummary = this.summarizeCoreCaptures(turn);
    if (captureSummary) {
      parts.push(captureSummary);
    }

    // Energy collection
    const energySummary = this.summarizeEnergyCollection(turn, players);
    if (energySummary) {
      parts.push(energySummary);
    }

    // Bot spawns
    const spawnSummary = this.summarizeBotSpawns(turn, players);
    if (spawnSummary) {
      parts.push(spawnSummary);
    }

    // Win probability
    if (winProb && winProb[turnIdx]) {
      const probs = winProb[turnIdx];
      const probSummary = probs.map((p, i) => `${players[i].name} ${Math.round(p * 100)}%`).join(', ');
      parts.push(`Win probability: ${probSummary}.`);
    }

    return {
      turn: turnIdx,
      text: parts.join(' '),
    };
  }

  /**
   * Summarize player movements for a turn.
   * Returns array of strings like "Player 1 (SwarmBot) moved 5 bots east."
   */
  private summarizePlayerMoves(turn: ReplayTurn, turnIdx: number, players: ReplayPlayer[]): string[] {
    const movesByPlayer: Map<number, { byDirection: Map<string, number>, total: number }> = new Map();

    // Initialize for all players
    players.forEach((_, idx) => {
      movesByPlayer.set(idx, { byDirection: new Map(), total: 0 });
    });

    // Count moves by direction per player
    // We need to compare with previous turn to detect movements
    if (turnIdx > 0) {
      const prevTurn = this.replay!.turns[turnIdx - 1];
      const prevBots = new Map(prevTurn.bots.map(b => [b.id, b]));

      for (const bot of turn.bots) {
        if (!bot.alive) continue;
        const prevBot = prevBots.get(bot.id);
        if (!prevBot || !prevBot.alive) continue;

        const dr = bot.position.row - prevBot.position.row;
        const dc = bot.position.col - prevBot.position.col;

        // Handle toroidal wrapping
        const rows = this.replay!.map.rows;
        const cols = this.replay!.map.cols;
        if (Math.abs(dr) > rows / 2) {
          // Wrapped vertically
        }
        if (Math.abs(dc) > cols / 2) {
          // Wrapped horizontally
        }

        let direction: string | null = null;
        if (dr === -1 && dc === 0) direction = 'north';
        else if (dr === 1 && dc === 0) direction = 'south';
        else if (dr === 0 && dc === 1) direction = 'east';
        else if (dr === 0 && dc === -1) direction = 'west';

        if (direction) {
          const playerMoves = movesByPlayer.get(bot.owner)!;
          const count = playerMoves.byDirection.get(direction) ?? 0;
          playerMoves.byDirection.set(direction, count + 1);
          playerMoves.total++;
        }
      }
    }

    const summaries: string[] = [];
    for (const [playerIdx, moves] of movesByPlayer) {
      if (moves.total === 0) continue;

      const directionParts: string[] = [];
      const dirNames: Record<string, string> = {
        north: 'north',
        south: 'south',
        east: 'east',
        west: 'west',
      };

      for (const [dir, count] of moves.byDirection) {
        directionParts.push(`${count} ${dirNames[dir]}`);
      }

      const playerName = players[playerIdx].name;
      summaries.push(`${playerName} moved ${directionParts.join(', ')}.`);
    }

    return summaries;
  }

  /**
   * Summarize combat events (bot deaths) for a turn.
   */
  private summarizeCombatEvents(turn: ReplayTurn): string | null {
    const events = turn.events ?? [];
    const deathEvents = events.filter(e => e.type === 'bot_died');

    if (deathEvents.length === 0) return null;

    // Group deaths by position (combat at same location)
    const deathsByPosition = new Map<string, Array<{ owner: number; count: number }>>();

    for (const event of deathEvents) {
      const details = event.details as Record<string, unknown>;
      const pos = details.position as Position | undefined;
      const owner = details.owner as number ?? 0;

      if (!pos) continue;

      const key = `${pos.row},${pos.col}`;
      if (!deathsByPosition.has(key)) {
        deathsByPosition.set(key, []);
      }

      // Check if this owner already has deaths at this position
      const existing = deathsByPosition.get(key)!.find(d => d.owner === owner);
      if (existing) {
        existing.count++;
      } else {
        deathsByPosition.get(key)!.push({ owner, count: 1 });
      }
    }

    const combatParts: string[] = [];
    for (const [posKey, deaths] of deathsByPosition) {
      const [row, col] = posKey.split(',').map(Number);
      const deathDescriptions = deaths.map(d => {
        const playerName = this.replay!.players[d.owner].name;
        return `${d.count} ${playerName} unit${d.count > 1 ? 's' : ''}`;
      }).join(', ');

      combatParts.push(`Combat at (${row},${col}): ${deathDescriptions} killed.`);
    }

    return combatParts.join(' ');
  }

  /**
   * Summarize core captures for a turn.
   */
  private summarizeCoreCaptures(turn: ReplayTurn): string | null {
    const events = turn.events ?? [];
    const captureEvents = events.filter(e => e.type === 'core_captured');

    if (captureEvents.length === 0) return null;

    const captures = captureEvents.map(e => {
      const details = e.details as Record<string, unknown>;
      const oldOwner = details.old_owner as number ?? 0;
      const newOwner = details.new_owner as number ?? 0;
      const pos = details.position as Position | undefined;

      const oldPlayerName = this.replay!.players[oldOwner].name;
      const newPlayerName = this.replay!.players[newOwner].name;
      const posStr = pos ? ` at (${pos.row},${pos.col})` : '';

      return `${newPlayerName} captured ${oldPlayerName}'s core${posStr}.`;
    });

    return captures.join(' ');
  }

  /**
   * Summarize energy collection for a turn.
   */
  private summarizeEnergyCollection(turn: ReplayTurn, players: ReplayPlayer[]): string | null {
    const events = turn.events ?? [];
    const energyEvents = events.filter(e => e.type === 'energy_collected');

    if (energyEvents.length === 0) return null;

    // Group by player
    const energyByPlayer = new Map<number, number>();
    for (const event of energyEvents) {
      const details = event.details as Record<string, unknown>;
      const owner = details.owner as number ?? 0;
      const count = energyByPlayer.get(owner) ?? 0;
      energyByPlayer.set(owner, count + 1);
    }

    const parts: string[] = [];
    for (const [playerIdx, count] of energyByPlayer) {
      const playerName = players[playerIdx].name;
      const positions = energyEvents
        .filter(e => (e.details as Record<string, unknown>).owner === playerIdx)
        .map(e => {
          const pos = (e.details as Record<string, unknown>).position as Position | undefined;
          return pos ? `(${pos.row},${pos.col})` : '';
        })
        .filter(Boolean)
        .slice(0, 3); // Limit to first 3 positions

      const posStr = positions.length > 0
        ? ` at ${positions.join(', ')}${positions.length < energyEvents.filter(e => (e.details as Record<string, unknown>).owner === playerIdx).length ? '...' : ''}`
        : '';

      parts.push(`${playerName} collected ${count} energy${posStr}.`);
    }

    return parts.join(' ');
  }

  /**
   * Summarize bot spawns for a turn.
   */
  private summarizeBotSpawns(turn: ReplayTurn, players: ReplayPlayer[]): string | null {
    const events = turn.events ?? [];
    const spawnEvents = events.filter(e => e.type === 'bot_spawned');

    if (spawnEvents.length === 0) return null;

    // Group by player
    const spawnsByPlayer = new Map<number, number>();
    for (const event of spawnEvents) {
      const details = event.details as Record<string, unknown>;
      const owner = details.owner as number ?? 0;
      const count = spawnsByPlayer.get(owner) ?? 0;
      spawnsByPlayer.set(owner, count + 1);
    }

    const parts: string[] = [];
    for (const [playerIdx, count] of spawnsByPlayer) {
      const playerName = players[playerIdx].name;
      parts.push(`${playerName} spawned ${count} bot${count > 1 ? 's' : ''}.`);
    }

    return parts.join(' ');
  }

  // Get transcript for a specific turn (for ARIA announcements)
  getTranscriptForTurn(turn: number): string {
    if (!this.replay) return '';
    const turnData = this.replay.turns[turn];
    if (!turnData) return '';

    const entry = this.generateTurnTranscript(turn, turnData, this.replay.players, this.replay.win_prob);
    return entry.text;
  }

  // Draw a player shape (circle, square, triangle, etc.)
  private drawPlayerShape(x: number, y: number, radius: number, playerIdx: number, color: string): void {
    const { ctx } = this;
    const shape = PLAYER_SHAPES[playerIdx % PLAYER_SHAPES.length];

    ctx.fillStyle = color;
    ctx.strokeStyle = this.accessibility.highContrast ? '#ffffff' : '#ffffff';
    ctx.lineWidth = this.accessibility.highContrast ? 2 : 1;

    if (!this.accessibility.showShapes) {
      // Default: draw circle
      ctx.beginPath();
      ctx.arc(x, y, radius, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
      return;
    }

    switch (shape) {
      case 'circle':
        ctx.beginPath();
        ctx.arc(x, y, radius, 0, Math.PI * 2);
        ctx.fill();
        ctx.stroke();
        break;

      case 'square':
        ctx.beginPath();
        ctx.rect(x - radius, y - radius, radius * 2, radius * 2);
        ctx.fill();
        ctx.stroke();
        break;

      case 'triangle':
        ctx.beginPath();
        ctx.moveTo(x, y - radius);
        ctx.lineTo(x + radius * 0.866, y + radius * 0.5);
        ctx.lineTo(x - radius * 0.866, y + radius * 0.5);
        ctx.closePath();
        ctx.fill();
        ctx.stroke();
        break;

      case 'diamond':
        ctx.beginPath();
        ctx.moveTo(x, y - radius);
        ctx.lineTo(x + radius * 0.707, y);
        ctx.lineTo(x, y + radius);
        ctx.lineTo(x - radius * 0.707, y);
        ctx.closePath();
        ctx.fill();
        ctx.stroke();
        break;

      case 'pentagon':
        this.drawPolygon(x, y, radius, 5);
        ctx.fill();
        ctx.stroke();
        break;

      case 'hexagon':
        this.drawPolygon(x, y, radius, 6);
        ctx.fill();
        ctx.stroke();
        break;
    }
  }

  // Helper to draw regular polygons
  private drawPolygon(cx: number, cy: number, radius: number, sides: number): void {
    const { ctx } = this;
    ctx.beginPath();
    for (let i = 0; i < sides; i++) {
      const angle = (i * 2 * Math.PI / sides) - Math.PI / 2;
      const x = cx + radius * Math.cos(angle);
      const y = cy + radius * Math.sin(angle);
      if (i === 0) {
        ctx.moveTo(x, y);
      } else {
        ctx.lineTo(x, y);
      }
    }
    ctx.closePath();
  }

  // ── Continuous 60fps render loop (decoupled from tick rate) ─────────────────
  private startRenderLoop(): void {
    if (this.renderLoopRunning) return;
    this.renderLoopRunning = true;
    this.lastRenderTime = performance.now();
    this.renderLoopTick(this.lastRenderTime);
  }

  private stopRenderLoop(): void {
    this.renderLoopRunning = false;
    if (this.animationFrame !== null) {
      cancelAnimationFrame(this.animationFrame);
      this.animationFrame = null;
    }
  }

  private renderLoopTick(timestamp: number): void {
    if (!this.renderLoopRunning) return;

    const dt = timestamp - this.lastRenderTime;
    this.lastRenderTime = timestamp;

    // Advance idle pulse phase (2s cycle = π per second)
    this.idlePhase += (Math.PI * dt) / 1000;

    // Update follow camera (§16.12)
    this.updateCamera();

    // Tick particles and effects
    if (!this.accessibility.reducedMotion) {
      tickParticles(dt);
      tickEffects(dt);
    }

    // If playing, check if we should advance to next turn
    if (this.isPlaying && this.replay) {
      const effectiveSpeed = this.directorEnabled ? this.directorMsPerTurn : this.animationSpeed;
      const turnElapsed = timestamp - this.turnStartTime;
      if (turnElapsed >= effectiveSpeed) {
        if (this.currentTurn < this.replay.turns.length - 1) {
          this.advanceTurn(this.currentTurn + 1);
        } else {
          this.pause();
          // Render one last frame
          this.render();
          return;
        }
      }
    }

    // Always render at display refresh rate
    this.render();

    this.animationFrame = requestAnimationFrame(this.renderLoopTick.bind(this));
  }

  private advanceTurn(newTurn: number): void {
    if (!this.replay) return;

    // Store previous bot positions before advancing
    const prevTurnData = this.replay.turns[this.currentTurn];
    this.botPrevPos.clear();
    if (prevTurnData) {
      for (const bot of prevTurnData.bots) {
        if (!bot.alive) continue;
        this.botPrevPos.set(bot.id, {
          x: bot.position.col * this.cellSize + this.cellSize / 2,
          y: bot.position.row * this.cellSize + this.cellSize / 2,
        });
      }
    }

    this.currentTurn = newTurn;
    this.turnStartTime = performance.now();

    // Fire events for the new turn to spawn animations
    const turnData = this.replay.turns[this.currentTurn];
    if (turnData && !this.accessibility.reducedMotion) {
      this.fireTurnAnimations(turnData);
    }

    if (this.onTurnChange) this.onTurnChange(this.currentTurn);
    this.fireCommentaryForTurn(this.currentTurn);
    this.fireDebugForTurn(this.currentTurn);

    // Announce turn transcript to screen readers during auto-playback (§15.3)
    if (this.isPlaying) {
      const transcriptText = this.getTranscriptForTurn(this.currentTurn);
      if (transcriptText) {
        this.announceToScreenReader(transcriptText);
      }
    }
  }

  private fireTurnAnimations(turnData: ReplayTurn): void {
    const colors = this.getPlayerColors();
    const events = turnData.events ?? [];

    // Track spawned bot IDs for spawn animation
    this.spawnedBotIds.clear();

    for (const event of events) {
      const d = event.details as Record<string, unknown>;
      if (!d) continue;

      switch (event.type) {
        case 'bot_died': {
          const pos = d.position as Position | undefined;
          if (!pos) break;
          const owner = d.owner as number ?? 0;
          const cx = pos.col * this.cellSize + this.cellSize / 2;
          const cy = pos.row * this.cellSize + this.cellSize / 2;
          // Spawn 6-8 particles in random directions
          const count = 6 + Math.floor(Math.random() * 3);
          for (let i = 0; i < count; i++) {
            const angle = (Math.PI * 2 * i) / count + (Math.random() - 0.5) * 0.5;
            const speed = 40 + Math.random() * 60; // px/s
            borrowParticle(
              cx, cy,
              Math.cos(angle) * speed / 1000,
              Math.sin(angle) * speed / 1000,
              colors[owner] ?? '#ef4444',
              400
            );
          }
          break;
        }
        case 'energy_collected': {
          const pos = d.position as Position | undefined;
          if (!pos) break;
          const cx = pos.col * this.cellSize + this.cellSize / 2;
          const cy = pos.row * this.cellSize + this.cellSize / 2;
          // 4-line starburst
          for (let i = 0; i < 4; i++) {
            const angle = (Math.PI / 2) * i;
            borrowParticle(cx, cy, Math.cos(angle) * 0.05, Math.sin(angle) * 0.05, ENERGY_COLOR, 200);
          }
          // Floating '+1'
          const ft = borrowSlot(floatTexts);
          if (ft) {
            ft.x = cx; ft.y = cy - 8;
            ft.text = '+1'; ft.color = ENERGY_COLOR;
            ft.elapsed = 0; ft.lifetime = 200; ft.active = true;
          }
          break;
        }
        case 'core_captured': {
          const pos = d.position as Position | undefined;
          if (!pos) break;
          const newOwner = d.new_owner as number ?? 0;
          const cx = pos.col * this.cellSize + this.cellSize / 2;
          const cy = pos.row * this.cellSize + this.cellSize / 2;
          const sw = borrowSlot(shockwaves);
          if (sw) {
            sw.x = cx; sw.y = cy; sw.radius = 0;
            sw.maxRadius = this.cellSize * 2;
            sw.color = colors[newOwner] ?? '#fff';
            sw.elapsed = 0; sw.lifetime = 500; sw.active = true;
          }
          break;
        }
        case 'bot_spawned': {
          const botId = d.bot_id as number | undefined;
          if (botId !== undefined) this.spawnedBotIds.add(botId);
          const owner = d.owner as number ?? 0;
          const pos = d.position as Position | undefined;
          if (!pos) break;
          const cx = pos.col * this.cellSize + this.cellSize / 2;
          const cy = pos.row * this.cellSize + this.cellSize / 2;
          const sg = borrowSlot(spawnGlows);
          if (sg) {
            sg.x = cx; sg.y = cy;
            sg.color = colors[owner] ?? '#fff';
            sg.elapsed = 0; sg.lifetime = 200; sg.active = true;
          }
          break;
        }
      }
    }
  }

  // Lerp factor: 0 at turn start → 1 at turn end
  private getLerpT(): number {
    const elapsed = performance.now() - this.turnStartTime;
    return Math.min(1, elapsed / this.animationSpeed);
  }

  private render(): void {
    if (!this.replay) return;

    // If a view mode cross-fade is active, blend the two offscreen buffers
    if (this.viewTransition.active) {
      const { ctx } = this;
      const now = performance.now();
      const elapsed = now - this.viewTransition.startTime;
      let t = Math.min(1, elapsed / this.viewTransition.duration);

      // Ease-in-out cubic
      t = t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;

      // Blend the two complete frames (both already contain overlays)
      ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
      ctx.globalAlpha = 1 - t;
      if (this.viewTransition.offscreenFrom) {
        ctx.drawImage(this.viewTransition.offscreenFrom, 0, 0);
      }
      ctx.globalAlpha = t;
      if (this.viewTransition.offscreenTo) {
        ctx.drawImage(this.viewTransition.offscreenTo, 0, 0);
      }
      ctx.globalAlpha = 1;

      // End transition when complete
      if (elapsed >= this.viewTransition.duration) {
        this.viewTransition.active = false;
        this.viewTransition.offscreenFrom = null;
        this.viewTransition.offscreenTo = null;
      }
      return;
    }

    this.renderViewLayer();
  }

  // Renders the full frame for the current view mode (no transition blending)
  private renderViewLayer(): void {
    if (!this.replay) return;

    const { ctx } = this;
    const colors = this.getPlayerColors();
    const bgColor = this.getBackgroundColor();
    const gridColor = this.getGridColor();
    const wallColor = this.getWallColor();
    const energyColor = this.getEnergyColor();
    const neutralColor = this.accessibility.highContrast ? HIGH_CONTRAST_NEUTRAL : NEUTRAL_COLOR;

    // Clear canvas
    ctx.fillStyle = bgColor;
    ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);

    // Get current turn data
    const turnData = this.replay.turns[this.currentTurn];
    if (!turnData) return;

    // Determine visibility for fog of war
    const visible = this.fogOfWarPlayer !== null
      ? this.computeVisibility(turnData, this.fogOfWarPlayer)
      : null;

    // ── Camera transform: clip to map area, apply pan/zoom (§16.12) ──
    const mapH = this.replay.map.rows * this.cellSize;
    ctx.save();
    ctx.beginPath();
    ctx.rect(0, 0, this.canvas.width, mapH);
    ctx.clip();
    this.applyCameraTransform();

    // Render based on view mode
    switch (this.viewMode) {
      case 'dots':
        this.renderDotsView(turnData, visible, colors, neutralColor, energyColor);
        break;
      case 'influence':
        this.renderInfluenceView(turnData, visible, colors, neutralColor, energyColor, wallColor);
        break;
      case 'voronoi':
        this.renderVoronoiView(turnData, visible, colors, neutralColor, energyColor, wallColor);
        break;
      case 'standard':
      default:
        this.renderStandardView(turnData, visible, colors, neutralColor, energyColor, wallColor, gridColor);
        break;
    }

    // Draw animated particles and effects (if not reduced motion)
    if (!this.accessibility.reducedMotion) {
      drawEffects(ctx);
      drawParticles(ctx);
    }

    // Draw annotation markers on canvas (§16.8) — world-space
    this.renderAnnotationMarkers(colors);

    ctx.restore();
    // ── End camera transform ──

    // Draw debug telemetry overlay (screen-space)
    if (this.showDebug && turnData.debug) {
      this.renderDebugOverlay(turnData.debug, colors);
    }

    // Draw score overlay (screen-space, below map)
    this.drawScoreOverlay(turnData, colors);

    // Announce turn to screen reader if reduced motion is preferred
    if (this.accessibility.reducedMotion) {
      const events = turnData.events ?? [];
      this.announceToScreenReader(this.generateTurnDescription(events));
    }

    // Keep sparkline current-turn marker in sync
    if (this.winProbCanvas && this.winProbData) {
      this.renderWinProbSparkline();
    }
  }

  // Standard view with grid
  private renderStandardView(
    turnData: ReplayTurn,
    visible: Set<string> | null,
    colors: string[],
    neutralColor: string,
    energyColor: string,
    wallColor: string,
    gridColor: string
  ): void {
    const { ctx, cellSize, showGrid, replay } = this;
    const { rows, cols } = replay!.map;

    // Draw grid lines
    if (showGrid) {
      ctx.strokeStyle = gridColor;
      ctx.lineWidth = this.accessibility.highContrast ? 1 : 0.5;
      for (let r = 0; r <= rows; r++) {
        ctx.beginPath();
        ctx.moveTo(0, r * cellSize);
        ctx.lineTo(cols * cellSize, r * cellSize);
        ctx.stroke();
      }
      for (let c = 0; c <= cols; c++) {
        ctx.beginPath();
        ctx.moveTo(c * cellSize, 0);
        ctx.lineTo(c * cellSize, rows * cellSize);
        ctx.stroke();
      }
    }

    // Draw walls
    for (const wall of this.replay!.map.walls) {
      this.drawCell(wall.row, wall.col, wallColor);
    }

    // Draw cores
    for (const core of turnData.cores) {
      if (visible && !visible.has(this.posKey(core.position))) continue;
      const color = core.active ? colors[core.owner] : neutralColor;
      this.drawCore(core.position.row, core.position.col, color, core.active);
    }

    // Draw energy
    for (const energy of turnData.energy) {
      if (visible && !visible.has(this.posKey(energy))) continue;
      this.drawEnergy(energy.row, energy.col, energyColor);
    }

    // Draw bots
    for (const bot of turnData.bots) {
      if (!bot.alive) continue;
      if (visible && !visible.has(this.posKey(bot.position))) continue;
      const color = colors[bot.owner];
      this.drawBot(bot, color);
    }

    // Draw combat effects from events this turn
    this.drawCombatEffects(turnData, colors, visible);

    // Draw threat lines between bots in attack range
    if (!this.accessibility.reducedMotion) {
      this.drawThreatLines(turnData, visible);
    }
  }

  private drawCombatEffects(
    turnData: ReplayTurn,
    colors: string[],
    visible: Set<string> | null
  ): void {
    const { ctx, cellSize } = this;
    const events = turnData.events ?? [];

    // Collect death positions
    const deaths: Array<{pos: Position; owner: number}> = [];
    for (const event of events) {
      if (event.type === 'bot_died') {
        const d = event.details as any;
        const rawPos = d.position ?? d.pos ?? d;
        const pos: Position = {row: rawPos.Row ?? rawPos.row ?? 0, col: rawPos.Col ?? rawPos.col ?? 0};
        if (pos.row === 0 && pos.col === 0 && !d.position && !d.pos) continue; // skip if no real position
        deaths.push({pos, owner: d.owner ?? 0});
      }
    }

    if (deaths.length === 0) return;

    // Find living bots this turn to draw attack lines from nearby enemies
    const livingBots = turnData.bots.filter(b => b.alive);
    const attackRadius = Math.sqrt(this.replay?.config?.attack_radius2 ?? 5) * cellSize;

    for (const death of deaths) {
      if (visible && !visible.has(this.posKey(death.pos))) continue;

      const dx = death.pos.col * cellSize + cellSize / 2;
      const dy = death.pos.row * cellSize + cellSize / 2;

      // Draw attack lines from nearby enemy bots to the death position
      for (const attacker of livingBots) {
        if (attacker.owner === death.owner) continue;
        const ax = attacker.position.col * cellSize + cellSize / 2;
        const ay = attacker.position.row * cellSize + cellSize / 2;
        const dist = Math.hypot(ax - dx, ay - dy);

        if (dist < attackRadius + cellSize * 3) {
          ctx.strokeStyle = colors[attacker.owner];
          ctx.lineWidth = 1.5;
          ctx.globalAlpha = 0.4;
          ctx.setLineDash([4, 4]);
          ctx.beginPath();
          ctx.moveTo(ax, ay);
          ctx.lineTo(dx, dy);
          ctx.stroke();
          ctx.setLineDash([]);
          ctx.globalAlpha = 1;
        }
      }

      // Draw red explosion flash behind the X
      const flashRadius = cellSize * 0.8;
      const gradient = ctx.createRadialGradient(dx, dy, 0, dx, dy, flashRadius);
      gradient.addColorStop(0, 'rgba(239, 68, 68, 0.6)');
      gradient.addColorStop(1, 'rgba(239, 68, 68, 0)');
      ctx.fillStyle = gradient;
      ctx.beginPath();
      ctx.arc(dx, dy, flashRadius, 0, Math.PI * 2);
      ctx.fill();

      // Draw death X marker
      const xSize = cellSize * 0.35;
      ctx.strokeStyle = '#fca5a5';
      ctx.lineWidth = 2.5;
      ctx.lineCap = 'round';
      ctx.beginPath();
      ctx.moveTo(dx - xSize, dy - xSize);
      ctx.lineTo(dx + xSize, dy + xSize);
      ctx.moveTo(dx + xSize, dy - xSize);
      ctx.lineTo(dx - xSize, dy + xSize);
      ctx.stroke();
      ctx.lineCap = 'butt';
    }
  }

  // Draw threat lines between bots of different owners within attack range
  private drawThreatLines(
    turnData: ReplayTurn,
    visible: Set<string> | null
  ): void {
    const { ctx, cellSize } = this;
    const aliveBots = turnData.bots.filter(b => b.alive);
    const attackRadius2 = this.replay?.config?.attack_radius2 ?? 5;
    const attackRadius = Math.sqrt(attackRadius2) * cellSize;

    for (let i = 0; i < aliveBots.length; i++) {
      for (let j = i + 1; j < aliveBots.length; j++) {
        const a = aliveBots[i];
        const b = aliveBots[j];
        if (a.owner === b.owner) continue;

        const ax = a.position.col * cellSize + cellSize / 2;
        const ay = a.position.row * cellSize + cellSize / 2;
        const bx = b.position.col * cellSize + cellSize / 2;
        const by = b.position.row * cellSize + cellSize / 2;

        // Use toroidal distance
        const dist = Math.hypot(
          Math.min(Math.abs(ax - bx), this.replay!.map.cols * cellSize - Math.abs(ax - bx)),
          Math.min(Math.abs(ay - by), this.replay!.map.rows * cellSize - Math.abs(ay - by))
        );

        if (dist <= attackRadius) {
          if (visible && (!visible.has(this.posKey(a.position)) || !visible.has(this.posKey(b.position)))) continue;
          ctx.strokeStyle = 'rgba(239, 68, 68, 0.25)';
          ctx.lineWidth = 1;
          ctx.setLineDash([3, 3]);
          ctx.beginPath();
          ctx.moveTo(ax, ay);
          ctx.lineTo(bx, by);
          ctx.stroke();
          ctx.setLineDash([]);
        }
      }
    }
  }

  // Dots view - minimal, just bot positions as dots
  private renderDotsView(
    turnData: ReplayTurn,
    visible: Set<string> | null,
    colors: string[],
    _neutralColor: string,
    _energyColor: string
  ): void {
    const { ctx, cellSize } = this;

    // Draw only bots as simple dots
    for (const bot of turnData.bots) {
      if (!bot.alive) continue;
      if (visible && !visible.has(this.posKey(bot.position))) continue;

      const x = bot.position.col * cellSize + cellSize / 2;
      const y = bot.position.row * cellSize + cellSize / 2;
      const radius = cellSize / 4;

      ctx.fillStyle = colors[bot.owner];
      ctx.beginPath();
      ctx.arc(x, y, radius, 0, Math.PI * 2);
      ctx.fill();
    }
  }

  // Influence view - shows territory influence gradient
  private renderInfluenceView(
    turnData: ReplayTurn,
    visible: Set<string> | null,
    colors: string[],
    _neutralColor: string,
    _energyColor: string,
    _wallColor: string
  ): void {
    const { ctx, cellSize, replay } = this;
    const { rows, cols } = replay!.map;

    // Compute influence map
    const influence = this.computeInfluenceMap(turnData);

    // Draw influence gradient
    for (let r = 0; r < rows; r++) {
      for (let c = 0; c < cols; c++) {
        const posKey = `${r},${c}`;
        if (visible && !visible.has(posKey)) continue;

        const inf = influence[r][c];
        if (inf.owner >= 0) {
          // Blend color based on influence strength
          const baseColor = colors[inf.owner];
          const alpha = Math.min(0.8, 0.2 + inf.strength * 0.6);
          ctx.fillStyle = this.hexToRgba(baseColor, alpha);
          ctx.fillRect(c * cellSize, r * cellSize, cellSize, cellSize);
        }
      }
    }

    // Draw bots on top
    for (const bot of turnData.bots) {
      if (!bot.alive) continue;
      if (visible && !visible.has(this.posKey(bot.position))) continue;
      const color = colors[bot.owner];
      this.drawBot(bot, color);
    }
  }

  // Voronoi view - shows Voronoi territories
  private renderVoronoiView(
    turnData: ReplayTurn,
    visible: Set<string> | null,
    colors: string[],
    _neutralColor: string,
    _energyColor: string,
    _wallColor: string
  ): void {
    const { ctx, cellSize, replay } = this;
    const { rows, cols } = replay!.map;

    // Compute Voronoi territories
    const territories = this.computeVoronoiTerritories(turnData);

    // Draw territories
    for (let r = 0; r < rows; r++) {
      for (let c = 0; c < cols; c++) {
        const posKey = `${r},${c}`;
        if (visible && !visible.has(posKey)) continue;

        const owner = territories[r][c];
        if (owner >= 0) {
          ctx.fillStyle = this.hexToRgba(colors[owner], 0.3);
          ctx.fillRect(c * cellSize, r * cellSize, cellSize, cellSize);
        }
      }
    }

    // Draw bots
    for (const bot of turnData.bots) {
      if (!bot.alive) continue;
      if (visible && !visible.has(this.posKey(bot.position))) continue;
      const color = colors[bot.owner];
      this.drawBot(bot, color);
    }
  }

  // Compute influence map (distance-weighted bot influence)
  private computeInfluenceMap(turnData: ReplayTurn): { owner: number; strength: number }[][] {
    const { rows, cols } = this.replay!.map;
    const influence: { owner: number; strength: number }[][] = [];

    // Initialize grid
    for (let r = 0; r < rows; r++) {
      influence[r] = [];
      for (let c = 0; c < cols; c++) {
        influence[r][c] = { owner: -1, strength: 0 };
      }
    }

    // For each cell, find the strongest influence
    for (let r = 0; r < rows; r++) {
      for (let c = 0; c < cols; c++) {
        let maxInfluence = 0;
        let bestOwner = -1;

        for (const bot of turnData.bots) {
          if (!bot.alive) continue;

          const dist = this.toroidalDistance(r, c, bot.position.row, bot.position.col);
          const inf = 1 / (1 + dist * 0.1);

          if (inf > maxInfluence) {
            maxInfluence = inf;
            bestOwner = bot.owner;
          }
        }

        influence[r][c] = { owner: bestOwner, strength: maxInfluence };
      }
    }

    return influence;
  }

  // Compute Voronoi territories (nearest bot ownership)
  private computeVoronoiTerritories(turnData: ReplayTurn): number[][] {
    const { rows, cols } = this.replay!.map;
    const territories: number[][] = [];

    for (let r = 0; r < rows; r++) {
      territories[r] = [];
      for (let c = 0; c < cols; c++) {
        let minDist = Infinity;
        let owner = -1;

        for (const bot of turnData.bots) {
          if (!bot.alive) continue;

          const dist = this.toroidalDistance(r, c, bot.position.row, bot.position.col);

          if (dist < minDist) {
            minDist = dist;
            owner = bot.owner;
          }
        }

        territories[r][c] = owner;
      }
    }

    return territories;
  }

  // Toroidal distance calculation
  private toroidalDistance(r1: number, c1: number, r2: number, c2: number): number {
    const { rows, cols } = this.replay!.map;
    const dr = Math.min(Math.abs(r1 - r2), rows - Math.abs(r1 - r2));
    const dc = Math.min(Math.abs(c1 - c2), cols - Math.abs(c1 - c2));
    return Math.sqrt(dr * dr + dc * dc);
  }

  // Convert hex color to rgba
  private hexToRgba(hex: string, alpha: number): string {
    const r = parseInt(hex.slice(1, 3), 16);
    const g = parseInt(hex.slice(3, 5), 16);
    const b = parseInt(hex.slice(5, 7), 16);
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }

  // Render debug telemetry overlay
  private renderDebugOverlay(debug: Record<number, DebugInfo>, colors: string[]): void {
    const { ctx, cellSize } = this;
    let reasoningRow = 0;

    for (const [playerId, info] of Object.entries(debug)) {
      const playerIdx = parseInt(playerId, 10);

      // Skip if this player's overlay is explicitly disabled
      if (this.debugPlayerEnabled.get(playerIdx) === false) continue;

      const color = colors[playerIdx] || '#ffffff';

      // Draw debug targets with priority-based opacity
      if (info.targets) {
        for (const target of info.targets) {
          const x = target.position.col * cellSize + cellSize / 2;
          const y = target.position.row * cellSize + cellSize / 2;
          const alpha = target.priority !== undefined ? Math.max(0.1, target.priority) : 1.0;
          const markerColor = target.color || color;

          ctx.globalAlpha = alpha;
          ctx.strokeStyle = markerColor;
          ctx.lineWidth = 2;
          ctx.beginPath();
          ctx.arc(x, y, cellSize / 2, 0, Math.PI * 2);
          ctx.stroke();

          if (target.label) {
            ctx.fillStyle = markerColor;
            ctx.font = '10px monospace';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'bottom';
            ctx.fillText(target.label, x, y - cellSize / 2 - 2);
          }
          ctx.globalAlpha = 1.0;
        }
      }

      // Draw reasoning text — stack boxes from the canvas bottom
      if (info.reasoning) {
        const padding = 10;
        const maxWidth = 200;
        const lineHeight = 14;
        const boxH = 54;
        const yTop = this.canvas.height - boxH - padding - reasoningRow * (boxH + 4);

        ctx.globalAlpha = 1.0;
        ctx.fillStyle = 'rgba(0, 0, 0, 0.82)';
        ctx.fillRect(padding, yTop, maxWidth + padding * 2, boxH);

        ctx.fillStyle = color;
        ctx.font = '11px monospace';
        ctx.textAlign = 'left';
        ctx.textBaseline = 'top';

        const lines = this.wrapText(info.reasoning, maxWidth);
        lines.forEach((line, i) => {
          ctx.fillText(line, padding * 2, yTop + padding / 2 + i * lineHeight);
        });

        reasoningRow++;
      }
    }

    // Reset canvas state
    ctx.globalAlpha = 1.0;
    ctx.textBaseline = 'alphabetic';
  }

  private renderAnnotationMarkers(_colors: string[]): void {
    const currentAnns = this.annotations.filter(a => a.turn === this.currentTurn);
    if (currentAnns.length === 0) return;

    const { ctx, cellSize } = this;
    const TYPE_COLORS: Record<string, string> = {
      insight: '#3b82f6',
      mistake: '#ef4444',
      idea: '#22c55e',
      highlight: '#fbbf24',
    };

    ctx.save();
    for (const ann of currentAnns) {
      const color = TYPE_COLORS[ann.type] ?? '#94a3b8';

      if (ann.position) {
        const x = ann.position.col * cellSize + cellSize / 2;
        const y = ann.position.row * cellSize + cellSize / 2;
        const r = cellSize / 2 + 2;

        ctx.globalAlpha = 0.6;
        ctx.strokeStyle = color;
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.arc(x, y, r, 0, Math.PI * 2);
        ctx.stroke();

        ctx.globalAlpha = 0.15;
        ctx.fillStyle = color;
        ctx.beginPath();
        ctx.arc(x, y, r, 0, Math.PI * 2);
        ctx.fill();
      }
    }
    ctx.restore();
  }

  // Wrap text to fit within max width
  private wrapText(text: string, maxWidth: number): string[] {
    const words = text.split(' ');
    const lines: string[] = [];
    let currentLine = '';

    for (const word of words) {
      const testLine = currentLine ? `${currentLine} ${word}` : word;
      // Approximate width (monospace, 11px)
      const width = testLine.length * 6.6;

      if (width > maxWidth && currentLine) {
        lines.push(currentLine);
        currentLine = word;
      } else {
        currentLine = testLine;
      }
    }

    if (currentLine) {
      lines.push(currentLine);
    }

    return lines.slice(0, 3); // Max 3 lines
  }

  // Get events for all turns (for timeline)
  getAllEvents(): { turn: number; events: GameEvent[] }[] {
    if (!this.replay) return [];

    return this.replay.turns.map((turn, idx) => ({
      turn: idx,
      events: turn.events ?? []
    }));
  }

  private computeVisibility(turnData: ReplayTurn, player: number): Set<string> {
    const visible = new Set<string>();
    const config = this.replay!.config;
    const visionRadius2 = config.vision_radius2;

    // Add all positions visible from this player's bots
    for (const bot of turnData.bots) {
      if (bot.owner !== player || !bot.alive) continue;

      // Add all cells within vision radius
      const vr = Math.ceil(Math.sqrt(visionRadius2));
      for (let dr = -vr; dr <= vr; dr++) {
        for (let dc = -vr; dc <= vr; dc++) {
          const dist2 = dr * dr + dc * dc;
          if (dist2 <= visionRadius2) {
            const r = (bot.position.row + dr + this.replay!.map.rows) % this.replay!.map.rows;
            const c = (bot.position.col + dc + this.replay!.map.cols) % this.replay!.map.cols;
            visible.add(`${r},${c}`);
          }
        }
      }
    }

    // Also add this player's cores (always visible)
    for (const core of this.replay!.map.cores) {
      if (core.owner === player) {
        visible.add(this.posKey(core.position));
      }
    }

    return visible;
  }

  private drawCell(row: number, col: number, color: string): void {
    const { ctx, cellSize } = this;
    ctx.fillStyle = color;
    ctx.fillRect(col * cellSize, row * cellSize, cellSize, cellSize);
  }

  private drawCore(row: number, col: number, color: string, active: boolean): void {
    const { ctx, cellSize } = this;
    const x = col * cellSize + cellSize / 2;
    const y = row * cellSize + cellSize / 2;
    const size = cellSize - 2;

    // Outer glow ring
    if (active) {
      ctx.strokeStyle = color;
      ctx.lineWidth = 2;
      ctx.globalAlpha = 0.3;
      ctx.beginPath();
      ctx.rect(x - size * 0.7, y - size * 0.7, size * 1.4, size * 1.4);
      ctx.stroke();
      ctx.globalAlpha = 1;
    }

    // Core body: filled square (distinct from circular bots)
    ctx.fillStyle = active ? color : '#4b5563';
    ctx.fillRect(x - size / 2, y - size / 2, size, size);

    // Inner diamond cutout for visual distinction
    ctx.fillStyle = active ? this.getBackgroundColor() : '#374151';
    const inner = size * 0.3;
    ctx.beginPath();
    ctx.moveTo(x, y - inner);
    ctx.lineTo(x + inner, y);
    ctx.lineTo(x, y + inner);
    ctx.lineTo(x - inner, y);
    ctx.closePath();
    ctx.fill();

    // Inactive: X overlay
    if (!active) {
      ctx.strokeStyle = '#ef4444';
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(x - size / 3, y - size / 3);
      ctx.lineTo(x + size / 3, y + size / 3);
      ctx.moveTo(x + size / 3, y - size / 3);
      ctx.lineTo(x - size / 3, y + size / 3);
      ctx.stroke();
    }
  }

  private drawEnergy(row: number, col: number, color: string): void {
    const { ctx, cellSize } = this;
    const x = col * cellSize + cellSize / 2;
    const y = row * cellSize + cellSize / 2;
    const radius = (cellSize / 3);

    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.arc(x, y, radius, 0, Math.PI * 2);
    ctx.fill();

    // Add star shape for accessibility
    if (this.accessibility.showShapes) {
      ctx.strokeStyle = this.accessibility.highContrast ? '#000000' : '#1f2937';
      ctx.lineWidth = 1;
      ctx.stroke();
    }
  }

  private drawBot(bot: ReplayBot, color: string): void {
    const { cellSize } = this;
    const targetX = bot.position.col * cellSize + cellSize / 2;
    const targetY = bot.position.row * cellSize + cellSize / 2;

    let x = targetX;
    let y = targetY;
    let scale = 1;

    if (!this.accessibility.reducedMotion) {
      // Lerp from previous position
      const prev = this.botPrevPos.get(bot.id);
      const t = this.getLerpT();
      if (prev && t < 1) {
        x = prev.x + (targetX - prev.x) * t;
        y = prev.y + (targetY - prev.y) * t;

        // Motion trail (only if moved meaningfully)
        const dx = targetX - prev.x;
        const dy = targetY - prev.y;
        if (Math.abs(dx) > 1 || Math.abs(dy) > 1) {
          const tr = borrowSlot(trails);
          if (tr) {
            tr.x = targetX; tr.y = targetY;
            tr.prevX = prev.x; tr.prevY = prev.y;
            tr.color = color; tr.alpha = 1; tr.active = true;
          }
        }
      }

      // Store interpolated position for this frame
      this.botRenderPos.set(bot.id, { x, y });

      // Idle pulse: 2% scale, 2s cycle
      const pulse = 1 + 0.02 * Math.sin(this.idlePhase);
      scale *= pulse;

      // Spawn animation: scale from 0→1 over 200ms
      if (this.spawnedBotIds.has(bot.id)) {
        const spawnT = Math.min(1, (performance.now() - this.turnStartTime) / 200);
        scale *= spawnT;
      }
    }

    const radius = ((cellSize / 2) - 1) * scale;
    this.drawPlayerShape(x, y, radius, bot.owner, color);
  }

  private drawScoreOverlay(turnData: ReplayTurn, colors: string[]): void {
    if (!this.replay) return;

    const { ctx } = this;
    const padding = 8;
    const lineHeight = 20;
    const mapHeight = this.replay.map.rows * this.cellSize;

    // Draw below the map, not over it
    const overlayY = mapHeight + 4;
    const bgHeight = padding * 2 + lineHeight * this.replay.players.length;
    const bgWidth = this.replay.map.cols * this.cellSize;

    ctx.fillStyle = '#1e293b';
    ctx.fillRect(0, overlayY, bgWidth, bgHeight);

    ctx.font = '13px monospace';
    ctx.textAlign = 'left';
    ctx.textBaseline = 'top';

    this.replay.players.forEach((player, idx) => {
      const score = turnData.scores[idx] ?? 0;
      const energy = turnData.energy_held[idx] ?? 0;
      const bots = turnData.bots.filter((b: any) => b.owner === idx).length;
      const color = colors[idx];
      const yOffset = overlayY + padding + idx * lineHeight;
      const isFollowed = this.followPlayer === idx;

      // Highlight row if followed
      if (isFollowed) {
        ctx.fillStyle = color + '22';
        ctx.fillRect(0, yOffset, bgWidth, lineHeight);
      }

      ctx.fillStyle = color;
      ctx.fillRect(padding, yOffset + 2, 12, 12);

      // Follow indicator (eye icon)
      const followIcon = isFollowed ? ' ◉' : ''; // ◉ when followed
      ctx.fillStyle = isFollowed ? color : '#e5e7eb';
      ctx.fillText(`${player.name}  score:${score}  bots:${bots}  energy:${energy}${followIcon}`, padding + 18, yOffset + 2);
    });
  }

  // Utility to get current turn events
  getTurnEvents(): GameEvent[] {
    if (!this.replay) return [];
    const turnData = this.replay.turns[this.currentTurn];
    return turnData?.events ?? [];
  }

  // Get replay info
  getReplay(): Replay | null {
    return this.replay;
  }

  // Check if at end of replay
  isAtEnd(): boolean {
    if (!this.replay) return true;
    return this.currentTurn >= this.replay.turns.length - 1;
  }

  // ── Win Probability Sparkline ─────────────────────────────────────────────────────

  private winProbData: WinProbPoint[] | null = null;
  private winProbCanvas: HTMLCanvasElement | null = null;
  private winProbCriticalMoments: CriticalMomentMarker[] = [];
  private winProbPlayerColors: string[] = [];

  setWinProbabilityData(points: WinProbPoint[]): void {
    this.winProbData = points;
    if (this.winProbCanvas) this.renderWinProbSparkline();
  }

  getWinProbabilityData(): WinProbPoint[] | null {
    return this.winProbData;
  }

  setCriticalMoments(moments: CriticalMomentMarker[]): void {
    this.winProbCriticalMoments = moments;
    if (this.winProbCanvas) this.renderWinProbSparkline();
  }

  getCriticalMomentMarkers(): CriticalMomentMarker[] {
    return this.winProbCriticalMoments;
  }

  // Set player colors used in the sparkline (must call before createWinProbSparkline)
  setWinProbPlayerColors(colors: string[]): void {
    this.winProbPlayerColors = colors;
  }

  // Re-render the sparkline at the current turn (call from onTurnChange)
  refreshWinProbSparkline(): void {
    if (this.winProbCanvas && this.winProbData) {
      this.renderWinProbSparkline();
    }
  }

  // Create and attach a win probability sparkline canvas below the main viewer.
  // Pass onTurnClick to enable click-to-scrub: clicking anywhere on the sparkline
  // calls onTurnClick with the nearest turn number.
  createWinProbSparkline(
    container: HTMLElement,
    width?: number,
    height = 70,
    onTurnClick?: (turn: number) => void,
  ): HTMLCanvasElement {
    // Replace any existing canvas
    if (this.winProbCanvas && this.winProbCanvas.parentElement === container) {
      container.removeChild(this.winProbCanvas);
    }

    this.winProbCanvas = document.createElement('canvas');
    this.winProbCanvas.width = width ?? Math.max(container.clientWidth, 400);
    this.winProbCanvas.height = height;
    this.winProbCanvas.className = 'win-prob-sparkline-canvas';
    this.winProbCanvas.style.cssText = `width:100%;height:${height}px;border-radius:6px;cursor:pointer;`;
    container.appendChild(this.winProbCanvas);

    if (onTurnClick) {
      this.winProbCanvas.addEventListener('click', (e) => {
        if (!this.winProbData || this.winProbData.length < 2 || !this.winProbCanvas) return;
        const rect = this.winProbCanvas.getBoundingClientRect();
        const x = (e.clientX - rect.left) * (this.winProbCanvas.width / rect.width);
        const padding = 4;
        const chartW = this.winProbCanvas.width - padding * 2;
        const maxTurn = this.winProbData[this.winProbData.length - 1].turn;
        const turn = Math.round(Math.max(0, Math.min(maxTurn, (x - padding) / chartW * maxTurn)));
        onTurnClick(turn);
      });
    }

    if (this.winProbData) this.renderWinProbSparkline();
    return this.winProbCanvas;
  }

  private renderWinProbSparkline(): void {
    if (!this.winProbCanvas || !this.winProbData || this.winProbData.length < 2) return;
    const ctx = this.winProbCanvas.getContext('2d');
    if (!ctx) return;

    renderWinProbSparkline(ctx, this.winProbData, this.currentTurn, {
      width: this.winProbCanvas.width,
      height: this.winProbCanvas.height,
      playerColors: this.winProbPlayerColors,
      criticalMoments: this.winProbCriticalMoments,
    });
  }
}
