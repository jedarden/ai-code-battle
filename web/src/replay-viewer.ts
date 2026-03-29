import type { Replay, ReplayTurn, Position, ReplayBot, GameEvent } from './types';

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
];

// Shape types for each player (0-5) - allows shape + color identification
type PlayerShape = 'circle' | 'square' | 'triangle' | 'diamond' | 'pentagon' | 'hexagon';
const PLAYER_SHAPES: PlayerShape[] = ['circle', 'square', 'triangle', 'diamond', 'pentagon', 'hexagon'];

const NEUTRAL_COLOR = '#6b7280'; // Gray
const WALL_COLOR = '#1f2937'; // Dark gray
const ENERGY_COLOR = '#fbbf24'; // Yellow
const BACKGROUND_COLOR = '#111827'; // Very dark gray
const GRID_COLOR = '#374151'; // Medium gray

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
    this.canvas.height = rows * this.cellSize;
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

    const { ctx, cellSize, canvas } = this;
    const { rows, cols } = this.replay.map;
    const colors = this.getPlayerColors();
    const bgColor = this.getBackgroundColor();
    const gridColor = this.getGridColor();
    const wallColor = this.getWallColor();
    const energyColor = this.getEnergyColor();
    const neutralColor = this.accessibility.highContrast ? HIGH_CONTRAST_NEUTRAL : NEUTRAL_COLOR;

    // Clear canvas
    ctx.fillStyle = bgColor;
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    // Draw grid lines
    if (this.showGrid) {
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

    // Get current turn data
    const turnData = this.replay.turns[this.currentTurn];
    if (!turnData) return;

    // Determine visibility for fog of war
    const visible = this.fogOfWarPlayer !== null
      ? this.computeVisibility(turnData, this.fogOfWarPlayer)
      : null;

    // Draw walls (always visible)
    for (const wall of this.replay.map.walls) {
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

    // Draw bots with accessible shapes
    for (const bot of turnData.bots) {
      if (!bot.alive) continue;
      if (visible && !visible.has(this.posKey(bot.position))) continue;
      const color = colors[bot.owner];
      this.drawBot(bot, color);
    }

    // Draw score overlay
    this.drawScoreOverlay(turnData, colors);

    // Announce turn to screen reader if reduced motion is preferred
    if (this.accessibility.reducedMotion) {
      const events = turnData.events ?? [];
      this.announceToScreenReader(this.generateTurnDescription(events));
    }
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
    const radius = (cellSize / 2) - 1;

    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.arc(x, y, radius, 0, Math.PI * 2);
    ctx.fill();

    // Draw inactive marker
    if (!active) {
      ctx.strokeStyle = this.getBackgroundColor();
      ctx.lineWidth = this.accessibility.highContrast ? 3 : 2;
      ctx.beginPath();
      ctx.moveTo(x - radius / 2, y - radius / 2);
      ctx.lineTo(x + radius / 2, y + radius / 2);
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
    const padding = 10;
    const lineHeight = 24; // Increased for shape indicators

    // Draw semi-transparent background
    ctx.fillStyle = this.accessibility.highContrast ? 'rgba(0, 0, 0, 0.9)' : 'rgba(0, 0, 0, 0.7)';
    const bgHeight = padding * 2 + lineHeight * this.replay.players.length;
    ctx.fillRect(0, 0, 170, bgHeight);

    // Draw scores for each player
    ctx.font = this.accessibility.highContrast ? 'bold 14px monospace' : '14px monospace';
    ctx.textAlign = 'left';
    ctx.textBaseline = 'top';

    this.replay.players.forEach((player, idx) => {
      const score = turnData.scores[idx] ?? 0;
      const energy = turnData.energy_held[idx] ?? 0;
      const color = colors[idx];
      const yOffset = padding + idx * lineHeight;

      // Draw shape indicator for accessibility
      const indicatorSize = 14;
      const indicatorX = padding + indicatorSize / 2;
      const indicatorY = yOffset + indicatorSize / 2 + 3;

      if (this.accessibility.showShapes) {
        this.drawPlayerShape(indicatorX, indicatorY, indicatorSize / 2 - 1, idx, color);
      } else {
        ctx.fillStyle = color;
        ctx.fillRect(padding, yOffset + 3, indicatorSize, indicatorSize);
      }

      // Draw text with better contrast in high contrast mode
      ctx.fillStyle = this.accessibility.highContrast ? '#ffffff' : '#e5e7eb';
      ctx.fillText(`${player.name}: ${score} (E:${energy})`, padding + 22, yOffset + 4);
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
}
