import type { Replay, ReplayTurn, Position, ReplayBot, GameEvent } from './types';

// Player colors - accessible and distinct
const PLAYER_COLORS = [
  '#3b82f6', // Blue (player 0)
  '#ef4444', // Red (player 1)
  '#22c55e', // Green (player 2)
  '#f59e0b', // Amber (player 3)
  '#8b5cf6', // Purple (player 4)
  '#06b6d4', // Cyan (player 5)
];

const NEUTRAL_COLOR = '#6b7280'; // Gray
const WALL_COLOR = '#1f2937'; // Dark gray
const ENERGY_COLOR = '#fbbf24'; // Yellow
const BACKGROUND_COLOR = '#111827'; // Very dark gray
const GRID_COLOR = '#374151'; // Medium gray

export interface ViewerOptions {
  cellSize?: number;
  showGrid?: boolean;
  fogOfWarPlayer?: number | null; // null = disabled, number = player perspective
  animationSpeed?: number; // ms per frame
}

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

    this.render = this.render.bind(this);
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

  setFogOfWar(player: number | null): void {
    this.fogOfWarPlayer = player;
    this.render();
  }

  getFogOfWar(): number | null {
    return this.fogOfWarPlayer;
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

    // Clear canvas
    ctx.fillStyle = BACKGROUND_COLOR;
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    // Draw grid lines
    if (this.showGrid) {
      ctx.strokeStyle = GRID_COLOR;
      ctx.lineWidth = 0.5;
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
      this.drawCell(wall.row, wall.col, WALL_COLOR);
    }

    // Draw cores
    for (const core of turnData.cores) {
      if (visible && !visible.has(this.posKey(core.position))) continue;
      const color = core.active ? PLAYER_COLORS[core.owner] : NEUTRAL_COLOR;
      this.drawCore(core.position.row, core.position.col, color, core.active);
    }

    // Draw energy
    for (const energy of turnData.energy) {
      if (visible && !visible.has(this.posKey(energy))) continue;
      this.drawEnergy(energy.row, energy.col);
    }

    // Draw bots
    for (const bot of turnData.bots) {
      if (!bot.alive) continue;
      if (visible && !visible.has(this.posKey(bot.position))) continue;
      const color = PLAYER_COLORS[bot.owner];
      this.drawBot(bot, color);
    }

    // Draw score overlay
    this.drawScoreOverlay(turnData);
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
      ctx.strokeStyle = BACKGROUND_COLOR;
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(x - radius / 2, y - radius / 2);
      ctx.lineTo(x + radius / 2, y + radius / 2);
      ctx.stroke();
    }
  }

  private drawEnergy(row: number, col: number): void {
    const { ctx, cellSize } = this;
    const x = col * cellSize + cellSize / 2;
    const y = row * cellSize + cellSize / 2;
    const radius = (cellSize / 3);

    ctx.fillStyle = ENERGY_COLOR;
    ctx.beginPath();
    ctx.arc(x, y, radius, 0, Math.PI * 2);
    ctx.fill();
  }

  private drawBot(bot: ReplayBot, color: string): void {
    const { ctx, cellSize } = this;
    const x = bot.position.col * cellSize + cellSize / 2;
    const y = bot.position.row * cellSize + cellSize / 2;
    const radius = (cellSize / 2) - 1;

    // Draw bot as filled circle
    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.arc(x, y, radius, 0, Math.PI * 2);
    ctx.fill();

    // Draw border
    ctx.strokeStyle = '#ffffff';
    ctx.lineWidth = 1;
    ctx.stroke();
  }

  private drawScoreOverlay(turnData: ReplayTurn): void {
    if (!this.replay) return;

    const { ctx } = this;
    const padding = 10;
    const lineHeight = 20;

    // Draw semi-transparent background
    ctx.fillStyle = 'rgba(0, 0, 0, 0.7)';
    ctx.fillRect(0, 0, 150, padding * 2 + lineHeight * this.replay.players.length);

    // Draw scores for each player
    ctx.font = '14px monospace';
    ctx.textAlign = 'left';
    ctx.textBaseline = 'top';

    this.replay.players.forEach((player, idx) => {
      const score = turnData.scores[idx] ?? 0;
      const energy = turnData.energy_held[idx] ?? 0;
      const color = PLAYER_COLORS[idx];

      // Draw color indicator
      ctx.fillStyle = color;
      ctx.fillRect(padding, padding + idx * lineHeight, 12, 12);

      // Draw text
      ctx.fillStyle = '#ffffff';
      ctx.fillText(`${player.name}: ${score} (E:${energy})`, padding + 18, padding + idx * lineHeight);
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
