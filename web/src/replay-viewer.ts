import type { Replay, ReplayTurn, Position, ReplayBot, GameEvent, DebugInfo, ViewMode } from './types';

// Win probability point for sparkline
export interface WinProbPoint {
  turn: number;
  p0WinProb: number;
  p1WinProb: number;
  drawProb?: number;
}

// Render win probability sparkline to canvas
export function renderWinProbSparkline(
  ctx: CanvasRenderingContext2D,
  points: WinProbPoint[],
  currentTurn: number,
  options: {
    width: number;
    height: number;
    color0?: string;
    color1?: string;
  },
): void {
  const { width, height, color0 = '#3b82f6', color1 = '#ef4444' } = options;
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

  // P0 area fill
  ctx.beginPath();
  ctx.moveTo(padding.left, y(0.5));
  for (const pt of points) {
    ctx.lineTo(x(pt.turn), y(pt.p0WinProb));
  }
  ctx.lineTo(width - padding.right, y(0.5));
  ctx.closePath();
  const grad = ctx.createLinearGradient(0, padding.top, 0, height - padding.bottom);
  grad.addColorStop(0, color0 + '44');
  grad.addColorStop(0.5, 'transparent');
  grad.addColorStop(1, color1 + '44');
  ctx.fillStyle = grad;
  ctx.fill();

  // P0 line
  ctx.beginPath();
  for (let i = 0; i < points.length; i++) {
    const pt = points[i];
    if (i === 0) ctx.moveTo(x(pt.turn), y(pt.p0WinProb));
    else ctx.lineTo(x(pt.turn), y(pt.p0WinProb));
  }
  ctx.strokeStyle = color0;
  ctx.lineWidth = 2;
  ctx.stroke();

  // P1 line (dashed)
  ctx.beginPath();
  for (let i = 0; i < points.length; i++) {
    const pt = points[i];
    if (i === 0) ctx.moveTo(x(pt.turn), y(pt.p1WinProb));
    else ctx.lineTo(x(pt.turn), y(pt.p1WinProb));
  }
  ctx.strokeStyle = color1;
  ctx.lineWidth = 1.5;
  ctx.setLineDash([4, 3]);
  ctx.stroke();
  ctx.setLineDash([]);

  // Current turn marker
  const curX = x(currentTurn);
  ctx.strokeStyle = '#f8fafc';
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.moveTo(curX, padding.top);
  ctx.lineTo(curX, height - padding.bottom);
  ctx.stroke();

  // Current probability dot
  const curPt = points.find(p => p.turn >= currentTurn) ?? points[points.length - 1];
  if (curPt) {
    ctx.beginPath();
    ctx.arc(curX, y(curPt.p0WinProb), 4, 0, Math.PI * 2);
    ctx.fillStyle = curPt.p0WinProb > 0.5 ? color0 : color1;
    ctx.fill();
    ctx.strokeStyle = '#ffffff';
    ctx.lineWidth = 1;
    ctx.stroke();
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
  private lastFrameTime: number = 0;
  private cellSize: number;
  private showGrid: boolean;
  private fogOfWarPlayer: number | null;
  private animationSpeed: number;
  private accessibility: AccessibilitySettings;
  private viewMode: ViewMode;
  private showDebug: boolean;
  private screenReaderRegion: HTMLElement | null = null;

  // Event callbacks
  public onTurnChange?: (turn: number) => void;
  public onPlayStateChange?: (playing: boolean) => void;
  public onReplayLoad?: (replay: Replay) => void;

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

    // Resize canvas to fit the grid
    this.resizeCanvas();

    // Render initial state
    this.render();

    if (this.onReplayLoad) this.onReplayLoad(replay);
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
    this.currentTurn = Math.max(0, Math.min(turn, this.replay.turns.length - 1));
    this.render();
    if (this.onTurnChange) this.onTurnChange(this.currentTurn);
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
    this.lastFrameTime = performance.now();
    this.animationFrame = requestAnimationFrame(this.animate.bind(this));
    if (this.onPlayStateChange) this.onPlayStateChange(true);
  }

  pause(): void {
    this.isPlaying = false;
    if (this.animationFrame !== null) {
      cancelAnimationFrame(this.animationFrame);
      this.animationFrame = null;
    }
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
    this.viewMode = mode;
    this.render();
  }

  getViewMode(): ViewMode {
    return this.viewMode;
  }

  setShowDebug(show: boolean): void {
    this.showDebug = show;
    this.render();
  }

  getShowDebug(): boolean {
    return this.showDebug;
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

  private animate(timestamp: number): void {
    if (!this.isPlaying || !this.replay) return;

    const elapsed = timestamp - this.lastFrameTime;
    if (elapsed >= this.animationSpeed) {
      this.lastFrameTime = timestamp;

      // Advance to next turn
      if (this.currentTurn < this.replay.turns.length - 1) {
        this.currentTurn++;
        this.render();
        if (this.onTurnChange) this.onTurnChange(this.currentTurn);
      } else {
        // End of replay
        this.pause();
        return;
      }
    }

    this.animationFrame = requestAnimationFrame(this.animate.bind(this));
  }

  private render(): void {
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

    // Draw debug telemetry overlay if enabled
    if (this.showDebug && turnData.debug) {
      this.renderDebugOverlay(turnData.debug, colors);
    }

    // Draw score overlay
    this.drawScoreOverlay(turnData, colors);

    // Announce turn to screen reader if reduced motion is preferred
    if (this.accessibility.reducedMotion) {
      const events = turnData.events ?? [];
      this.announceToScreenReader(this.generateTurnDescription(events));
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

    for (const [playerId, info] of Object.entries(debug)) {
      const playerIdx = parseInt(playerId, 10);
      const color = colors[playerIdx] || '#ffffff';

      // Draw debug targets
      if (info.targets) {
        for (const target of info.targets) {
          const x = target.position.col * cellSize + cellSize / 2;
          const y = target.position.row * cellSize + cellSize / 2;

          // Draw target marker
          ctx.strokeStyle = target.color || color;
          ctx.lineWidth = 2;
          ctx.beginPath();
          ctx.arc(x, y, cellSize / 2, 0, Math.PI * 2);
          ctx.stroke();

          // Draw label if provided
          if (target.label) {
            ctx.fillStyle = color;
            ctx.font = '10px monospace';
            ctx.textAlign = 'center';
            ctx.fillText(target.label, x, y - cellSize / 2 - 4);
          }
        }
      }

      // Draw reasoning text
      if (info.reasoning) {
        const padding = 10;
        const maxWidth = 200;
        const lineHeight = 14;

        ctx.fillStyle = 'rgba(0, 0, 0, 0.8)';
        ctx.fillRect(padding, this.canvas.height - 60 - padding, maxWidth + padding * 2, 50);

        ctx.fillStyle = color;
        ctx.font = '11px monospace';
        ctx.textAlign = 'left';

        const lines = this.wrapText(info.reasoning, maxWidth);
        lines.forEach((line, i) => {
          ctx.fillText(line, padding * 2, this.canvas.height - 60 + i * lineHeight);
        });
      }
    }
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
    const x = bot.position.col * cellSize + cellSize / 2;
    const y = bot.position.row * cellSize + cellSize / 2;
    const radius = (cellSize / 2) - 1;

    // Draw bot with player-specific shape for accessibility
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

      ctx.fillStyle = color;
      ctx.fillRect(padding, yOffset + 2, 12, 12);

      ctx.fillStyle = '#e5e7eb';
      ctx.fillText(`${player.name}  score:${score}  bots:${bots}  energy:${energy}`, padding + 18, yOffset + 2);
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

  // Set win probability data for sparkline rendering
  setWinProbabilityData(points: WinProbPoint[]): void {
    this.winProbData = points;
    if (this.winProbCanvas) {
      this.renderWinProbSparkline();
    }
  }

  // Get the win probability data
  getWinProbabilityData(): WinProbPoint[] | null {
    return this.winProbData;
  }

  // Create and attach a win probability sparkline canvas
  createWinProbSparkline(container: HTMLElement, width?: number, height = 60): HTMLCanvasElement {
    this.winProbCanvas = document.createElement('canvas');
    this.winProbCanvas.width = width ?? container.clientWidth;
    this.winProbCanvas.height = height;
    this.winProbCanvas.className = 'win-prob-sparkline-canvas';
    this.winProbCanvas.style.cssText = 'width:100%;height:' + height + 'px;border-radius:6px;';
    container.appendChild(this.winProbCanvas);

    if (this.winProbData) {
      this.renderWinProbSparkline();
    }

    return this.winProbCanvas;
  }

  // Render the sparkline
  private renderWinProbSparkline(): void {
    if (!this.winProbCanvas || !this.winProbData || this.winProbData.length < 2) return;

    const ctx = this.winProbCanvas.getContext('2d');
    if (!ctx) return;

    renderWinProbSparkline(ctx, this.winProbData, this.currentTurn, {
      width: this.winProbCanvas.width,
      height: this.winProbCanvas.height,
      color0: this.accessibility.highContrast ? '#0000ff' : '#3b82f6',
      color1: this.accessibility.highContrast ? '#ff0000' : '#ef4444',
    });
  }
}
