/**
 * Win Probability Sparkline Component
 *
 * Renders a sparkline graph showing win probability over time for each player.
 * Includes critical moment markers (turns where win probability shifted >15%).
 * Clicking anywhere on the sparkline scrubs to that turn.
 *
 * Usage:
 *   const sparkline = new WinProbSparkline(container, {
 *     width: 800,
 *     height: 70,
 *     onTurnClick: (turn) => viewer.setTurn(turn),
 *   });
 *   sparkline.setData(winProbData, playerColors, criticalMoments);
 *   sparkline.setCurrentTurn(currentTurn);
 */

export interface WinProbPoint {
  turn: number;
  probs: number[];  // one probability per player (0.0–1.0)
}

export interface CriticalMoment {
  turn: number;
  delta: number;
  description: string;
}

export interface WinProbSparklineOptions {
  width?: number;
  height?: number;
  onTurnClick?: (turn: number) => void;
  showLegend?: boolean;
}

// Default player colors (matches DEFAULT_PLAYER_COLORS from replay-viewer)
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

export class WinProbSparkline {
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  private data: WinProbPoint[] = [];
  private playerColors: string[] = SPARKLINE_COLORS;
  private criticalMoments: CriticalMoment[] = [];
  private currentTurn: number = 0;
  private onTurnClick?: (turn: number) => void;

  constructor(container: HTMLElement, options: WinProbSparklineOptions = {}) {
    const width = options.width ?? Math.max(container.clientWidth, 400);
    const height = options.height ?? 70;
    this.onTurnClick = options.onTurnClick;

    this.canvas = document.createElement('canvas');
    this.canvas.width = width;
    this.canvas.height = height;
    this.canvas.className = 'win-prob-sparkline-canvas';
    this.canvas.style.cssText = `
      width: 100%;
      height: ${height}px;
      border-radius: 6px;
      cursor: pointer;
      display: block;
    `;

    const ctx = this.canvas.getContext('2d');
    if (!ctx) {
      throw new Error('Failed to get 2D context for win probability sparkline');
    }
    this.ctx = ctx;

    container.appendChild(this.canvas);

    // Set up click handler for turn scrubbing
    if (this.onTurnClick) {
      this.canvas.addEventListener('click', (e) => this.handleClick(e));
    }
  }

  /**
   * Update the win probability data and redraw
   */
  setData(data: WinProbPoint[], playerColors?: string[], criticalMoments?: CriticalMoment[]): void {
    this.data = data;
    if (playerColors) {
      this.playerColors = playerColors;
    }
    if (criticalMoments) {
      this.criticalMoments = criticalMoments;
    }
    this.render();
  }

  /**
   * Update the current turn marker and redraw
   */
  setCurrentTurn(turn: number): void {
    this.currentTurn = turn;
    this.render();
  }

  /**
   * Update the canvas size (call on resize)
   */
  resize(width: number, height: number): void {
    this.canvas.width = width;
    this.canvas.height = height;
    this.render();
  }

  /**
   * Get the canvas element for integration with other components
   */
  getCanvas(): HTMLCanvasElement {
    return this.canvas;
  }

  /**
   * Clean up event listeners
   */
  destroy(): void {
    this.canvas.remove();
  }

  private handleClick(e: MouseEvent): void {
    if (this.data.length < 2 || !this.onTurnClick) return;

    const rect = this.canvas.getBoundingClientRect();
    const x = (e.clientX - rect.left) * (this.canvas.width / rect.width);

    const padding = { top: 8, bottom: 8, left: 4, right: 4 };
    const chartW = this.canvas.width - padding.left - padding.right;

    const maxTurn = this.data[this.data.length - 1].turn;
    const turn = Math.round(
      Math.max(0, Math.min(maxTurn, ((x - padding.left) / chartW) * maxTurn))
    );

    this.onTurnClick(turn);
  }

  private render(): void {
    const { width, height } = this.canvas;
    const padding = { top: 8, bottom: 8, left: 4, right: 4 };
    const chartW = width - padding.left - padding.right;
    const chartH = height - padding.top - padding.bottom;

    // Clear and draw background
    this.ctx.fillStyle = '#1e293b';
    this.ctx.fillRect(0, 0, width, height);

    if (this.data.length < 2) {
      this.ctx.fillStyle = '#475569';
      this.ctx.font = '12px sans-serif';
      this.ctx.textAlign = 'center';
      this.ctx.fillText('No data', width / 2, height / 2);
      return;
    }

    const maxTurn = this.data[this.data.length - 1].turn;

    const x = (turn: number) => padding.left + (turn / maxTurn) * chartW;
    const y = (prob: number) => padding.top + chartH * (1 - prob);

    // Draw 50% baseline
    const midY = y(0.5);
    this.ctx.strokeStyle = '#475569';
    this.ctx.lineWidth = 1;
    this.ctx.setLineDash([3, 3]);
    this.ctx.beginPath();
    this.ctx.moveTo(padding.left, midY);
    this.ctx.lineTo(width - padding.right, midY);
    this.ctx.stroke();
    this.ctx.setLineDash([]);

    // Draw percentage labels
    this.ctx.fillStyle = '#475569';
    this.ctx.font = '8px monospace';
    this.ctx.textAlign = 'right';
    this.ctx.fillText('100%', padding.left + 28, padding.top + 6);
    this.ctx.fillText('0%', padding.left + 22, height - padding.bottom - 1);
    this.ctx.fillText('50%', padding.left + 28, midY + 3);

    // Draw critical moment markers
    for (const moment of this.criticalMoments) {
      const mx = x(moment.turn);
      const markerColor = moment.delta > 0
        ? this.playerColors[0] ?? SPARKLINE_COLORS[0]
        : this.playerColors[1] ?? SPARKLINE_COLORS[1];

      // Dashed vertical line
      this.ctx.strokeStyle = markerColor + 'aa';
      this.ctx.lineWidth = 1.5;
      this.ctx.setLineDash([3, 3]);
      this.ctx.beginPath();
      this.ctx.moveTo(mx, padding.top);
      this.ctx.lineTo(mx, height - padding.bottom);
      this.ctx.stroke();
      this.ctx.setLineDash([]);

      // Diamond marker at midpoint
      const my = height / 2;
      const s = 3;
      this.ctx.fillStyle = markerColor;
      this.ctx.beginPath();
      this.ctx.moveTo(mx, my - s);
      this.ctx.lineTo(mx + s, my);
      this.ctx.lineTo(mx, my + s);
      this.ctx.lineTo(mx - s, my);
      this.ctx.closePath();
      this.ctx.fill();

      // Delta label
      const label = `${moment.delta > 0 ? '+' : ''}${(moment.delta * 100).toFixed(0)}%`;
      this.ctx.fillStyle = markerColor;
      this.ctx.font = '9px monospace';
      this.ctx.textAlign = 'center';
      this.ctx.fillText(
        label,
        Math.max(18, Math.min(width - 18, mx)),
        padding.top + 7
      );
    }

    const numPlayers = this.data[0].probs.length;

    // Draw area fill for first two players
    if (numPlayers >= 2) {
      this.ctx.beginPath();
      this.ctx.moveTo(padding.left, y(this.data[0].probs[0]));
      for (const pt of this.data) {
        this.ctx.lineTo(x(pt.turn), y(pt.probs[0]));
      }
      this.ctx.lineTo(width - padding.right, y(this.data[this.data.length - 1].probs[1]));
      for (let i = this.data.length - 1; i >= 0; i--) {
        this.ctx.lineTo(x(this.data[i].turn), y(this.data[i].probs[1]));
      }
      this.ctx.closePath();

      const grad = this.ctx.createLinearGradient(0, padding.top, 0, height - padding.bottom);
      grad.addColorStop(0, (this.playerColors[0] ?? SPARKLINE_COLORS[0]) + '33');
      grad.addColorStop(0.5, 'transparent');
      grad.addColorStop(1, (this.playerColors[1] ?? SPARKLINE_COLORS[1]) + '33');
      this.ctx.fillStyle = grad;
      this.ctx.fill();
    }

    // Draw lines for each player
    for (let p = numPlayers - 1; p >= 0; p--) {
      const color = this.playerColors[p] ?? SPARKLINE_COLORS[p % SPARKLINE_COLORS.length];
      this.ctx.beginPath();
      for (let i = 0; i < this.data.length; i++) {
        const pt = this.data[i];
        if (i === 0) {
          this.ctx.moveTo(x(pt.turn), y(pt.probs[p]));
        } else {
          this.ctx.lineTo(x(pt.turn), y(pt.probs[p]));
        }
      }
      this.ctx.strokeStyle = color;
      this.ctx.lineWidth = p === 0 ? 2 : 1.5;
      if (p > 1) this.ctx.setLineDash([4, 3]);
      this.ctx.stroke();
      this.ctx.setLineDash([]);
    }

    // Draw current turn marker
    const curX = x(this.currentTurn);
    this.ctx.strokeStyle = '#f8fafc';
    this.ctx.lineWidth = 2;
    this.ctx.beginPath();
    this.ctx.moveTo(curX, padding.top);
    this.ctx.lineTo(curX, height - padding.bottom);
    this.ctx.stroke();

    // Draw dots at current turn for all players
    const curPt = this.data.find(p => p.turn >= this.currentTurn) ?? this.data[this.data.length - 1];
    if (curPt) {
      for (let p = 0; p < curPt.probs.length; p++) {
        const color = this.playerColors[p] ?? SPARKLINE_COLORS[p % SPARKLINE_COLORS.length];
        this.ctx.beginPath();
        this.ctx.arc(curX, y(curPt.probs[p]), 4, 0, Math.PI * 2);
        this.ctx.fillStyle = color;
        this.ctx.fill();
        this.ctx.strokeStyle = '#ffffff';
        this.ctx.lineWidth = 1;
        this.ctx.stroke();
      }
    }
  }
}

/**
 * CSS styles for the win probability sparkline component
 */
export const WIN_PROB_STYLES = `
.win-prob-sparkline-canvas {
  display: block;
  width: 100%;
  border-radius: 6px;
  cursor: pointer;
  transition: transform 0.1s ease;
}

.win-prob-sparkline-canvas:hover {
  transform: scale(1.01);
}

.win-prob-section {
  background-color: var(--bg-secondary);
  border-radius: 8px;
  padding: 12px;
  margin-top: 10px;
}

.win-prob-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  flex-wrap: wrap;
  gap: 8px;
}

.win-prob-title {
  color: var(--text-muted);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 600;
}

.win-prob-container {
  width: 100%;
  overflow: hidden;
  border-radius: 4px;
}

.win-prob-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 6px;
  font-size: 0.75rem;
  font-family: monospace;
}

.critical-moment-nav {
  display: flex;
  align-items: center;
  gap: 8px;
}

.critical-moment-nav .btn {
  padding: 4px 10px;
  font-size: 0.75rem;
}

.critical-moment-nav .btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.critical-moment-info {
  color: var(--text-muted);
  font-size: 0.8rem;
  max-width: 280px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 900px) {
  .win-prob-section {
    padding: 8px;
  }

  .win-prob-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .win-prob-title {
    font-size: 0.7rem;
  }
}
`;
