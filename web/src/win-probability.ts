// Win probability via Monte Carlo rollout from a replay snapshot.
//
// Usage:
//   const wp = new WinProbabilityEngine(replay);
//   const probs = await wp.computeAll(50);   // run 50 simulations per turn
//   const sparkline = wp.getSparkline();     // [{turn, p0, p1}]
//   const critical = wp.getCriticalMoments();

import {
  type Config, type Replay, type ReplayTurn,
  type Move, type GameState, type Bot, type Core, type EnergyNode,
  type Player, type MatchResult,
  randomStrategy,
  getVisibleState, executeTurn, posKey,
} from './engine';

export interface WinProbPoint {
  turn: number;
  p0WinProb: number;
  p1WinProb: number;
  drawProb: number;
}

export interface CriticalMoment {
  turn: number;
  description: string;
  deltaP0: number; // change in p0 win probability (positive = improved for p0)
  type: 'swing' | 'kill' | 'capture' | 'energy' | 'milestone';
}

export class WinProbabilityEngine {
  private replay: Replay;
  private points: WinProbPoint[] = [];
  constructor(replay: Replay) {
    this.replay = replay;
  }

  /**
   * Compute win probability at every Nth turn using Monte Carlo rollouts.
   * @param simulations  number of random playouts per sampled turn
   * @param step         sample every N turns (default 5)
   */
  async computeAll(simulations = 50, step = 5): Promise<WinProbPoint[]> {
    const turns = this.replay.turns;
    this.points = [];

    const cfg = this.replay.config;
    const maxTurn = turns.length - 1;

    for (let t = 0; t <= maxTurn; t += step) {
      // Allow the browser to stay responsive
      if (t % (step * 10) === 0) {
        await yieldToUI();
      }

      const prob = this.rollout(turns[t], cfg, simulations);
      this.points.push({ turn: t, ...prob });
    }

    // Always include the last turn
    if (this.points[this.points.length - 1]?.turn !== maxTurn) {
      const prob = this.rollout(turns[maxTurn], cfg, simulations);
      this.points.push({ turn: maxTurn, ...prob });
    }

    return this.points;
  }

  getSparkline(): WinProbPoint[] {
    return this.points;
  }

  /**
   * Returns turns where the win probability swung by >= 15 percentage points.
   */
  getCriticalMoments(): CriticalMoment[] {
    if (this.points.length < 2) return [];
    const moments: CriticalMoment[] = [];

    for (let i = 1; i < this.points.length; i++) {
      const prev = this.points[i - 1];
      const curr = this.points[i];
      const delta = curr.p0WinProb - prev.p0WinProb;

      if (Math.abs(delta) >= 0.15) {
        const turnData = this.replay.turns[curr.turn];
        const description = this.describeMoment(turnData, delta);
        moments.push({
          turn: curr.turn,
          description,
          deltaP0: delta,
          type: classifyMoment(turnData, delta),
        });
      }
    }

    return moments;
  }

  // ── Private helpers ──────────────────────────────────────────────────────

  private rollout(
    snapshot: ReplayTurn,
    cfg: Config,
    simulations: number,
  ): { p0WinProb: number; p1WinProb: number; drawProb: number } {
    let p0Wins = 0, p1Wins = 0, draws = 0;

    for (let i = 0; i < simulations; i++) {
      const winner = simulateFromSnapshot(snapshot, cfg, this.replay);
      if (winner === 0) p0Wins++;
      else if (winner === 1) p1Wins++;
      else draws++;
    }

    return {
      p0WinProb: p0Wins / simulations,
      p1WinProb: p1Wins / simulations,
      drawProb: draws / simulations,
    };
  }

  private describeMoment(turn: ReplayTurn | undefined, delta: number): string {
    if (!turn) return 'Probability shift';
    const events = turn.events ?? [];
    const kills = events.filter(e => e.type === 'bot_died').length;
    const captures = events.filter(e => e.type === 'core_captured').length;

    if (captures > 0) return `Core captured (${delta > 0 ? '+' : ''}${(delta * 100).toFixed(0)}% shift)`;
    if (kills > 2) return `${kills} bots eliminated (${delta > 0 ? '+' : ''}${(delta * 100).toFixed(0)}% shift)`;
    return `Probability swung ${delta > 0 ? '+' : ''}${(delta * 100).toFixed(0)}%`;
  }
}

// ────────────────────────────────────────────────────────────────────────────
// Single-game rollout from a snapshot
// ────────────────────────────────────────────────────────────────────────────

function simulateFromSnapshot(snapshot: ReplayTurn, cfg: Config, replay: Replay): number {
  // Reconstruct a lightweight game state from the snapshot
  const gs = replaySnapshotToGameState(snapshot, cfg, replay);

  // Run the rest of the match with random strategies
  const s0 = randomStrategy;
  const s1 = randomStrategy;

  let result: MatchResult | null = null;
  let safety = cfg.max_turns - snapshot.turn + 1;

  while (!result && safety-- > 0) {
    const allMoves = new Map<number, Move[]>();
    for (const p of gs.players) {
      const visible = getVisibleState(gs, p.id);
      try {
        allMoves.set(p.id, p.id === 0 ? s0(visible) : s1(visible));
      } catch {
        allMoves.set(p.id, []);
      }
    }
    result = executeTurn(gs, allMoves);
  }

  return result?.winner ?? -1;
}

function replaySnapshotToGameState(snap: ReplayTurn, cfg: Config, replay: Replay): GameState {
  const walls = new Set<string>(
    (replay.map?.walls ?? []).map((p: any) => posKey(p)),
  );

  const bots: Bot[] = (snap.bots ?? []).map(b => ({ ...b }));
  const cores: Core[] = (snap.cores ?? []).map(c => ({ ...c }));

  // Reconstruct energy nodes from map + current state
  const energyOnTile = new Set<string>((snap.energy ?? []).map(posKey));
  const energy: EnergyNode[] = (replay.map?.energy_nodes ?? []).map((p: any) => ({
    position: p,
    hasEnergy: energyOnTile.has(posKey(p)),
    tick: 0,
  }));

  const players: Player[] = (snap.scores ?? [0, 0]).map((score: number, id: number) => ({
    id,
    energy: snap.energy_held?.[id] ?? 0,
    score,
    botCount: bots.filter(b => b.alive && b.owner === id).length,
  }));

  return {
    config: cfg,
    bots,
    cores,
    energy,
    players,
    turn: snap.turn,
    matchId: replay.match_id,
    walls,
    events: [],
    dominance: new Map(),
  };
}

function classifyMoment(turn: ReplayTurn | undefined, _delta: number): CriticalMoment['type'] {
  if (!turn) return 'swing';
  const events = turn.events ?? [];
  if (events.some((e: any) => e.type === 'core_captured')) return 'capture';
  if (events.filter((e: any) => e.type === 'bot_died').length > 2) return 'kill';
  if (events.some((e: any) => e.type === 'energy_collected')) return 'energy';
  return 'swing';
}

function yieldToUI(): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, 0));
}

// ────────────────────────────────────────────────────────────────────────────
// SVG sparkline renderer
// ────────────────────────────────────────────────────────────────────────────

export function renderWinProbSparkline(
  container: HTMLElement,
  points: WinProbPoint[],
  options: { width?: number; height?: number; showLegend?: boolean } = {},
): void {
  const W = (options.width ?? container.clientWidth) || 400;
  const H = options.height ?? 80;
  const showLegend = options.showLegend ?? true;

  if (points.length < 2) {
    container.innerHTML = '<div style="color:var(--text-muted);font-size:0.75rem;text-align:center;padding:10px">Not enough data</div>';
    return;
  }

  const maxTurn = points[points.length - 1].turn;

  function x(turn: number): number { return (turn / maxTurn) * W; }
  function y(prob: number): number { return H - prob * H; }

  // Build SVG paths
  const p0Path = points.map((pt, i) => `${i === 0 ? 'M' : 'L'} ${x(pt.turn).toFixed(1)} ${y(pt.p0WinProb).toFixed(1)}`).join(' ');
  const p1Path = points.map((pt, i) => `${i === 0 ? 'M' : 'L'} ${x(pt.turn).toFixed(1)} ${y(pt.p1WinProb).toFixed(1)}`).join(' ');

  // 50% line
  const midY = y(0.5).toFixed(1);

  const svg = `
    <svg width="${W}" height="${H + (showLegend ? 20 : 0)}" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <linearGradient id="p0grad" x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stop-color="#3b82f6" stop-opacity="0.3"/>
          <stop offset="100%" stop-color="#3b82f6" stop-opacity="0"/>
        </linearGradient>
      </defs>
      <!-- Background -->
      <rect width="${W}" height="${H}" fill="transparent"/>
      <!-- 50% line -->
      <line x1="0" y1="${midY}" x2="${W}" y2="${midY}" stroke="#475569" stroke-width="1" stroke-dasharray="4,4"/>
      <!-- P0 fill -->
      <path d="${p0Path} L ${W} ${H} L 0 ${H} Z" fill="url(#p0grad)"/>
      <!-- P0 line -->
      <path d="${p0Path}" fill="none" stroke="#3b82f6" stroke-width="2"/>
      <!-- P1 line -->
      <path d="${p1Path}" fill="none" stroke="#ef4444" stroke-width="2"/>
      ${showLegend ? `
        <circle cx="12" cy="${H + 12}" r="5" fill="#3b82f6"/>
        <text x="22" y="${H + 16}" fill="#94a3b8" font-size="11">Player 0</text>
        <circle cx="90" cy="${H + 12}" r="5" fill="#ef4444"/>
        <text x="100" y="${H + 16}" fill="#94a3b8" font-size="11">Player 1</text>
      ` : ''}
    </svg>
  `;

  container.innerHTML = svg;
}

// ────────────────────────────────────────────────────────────────────────────
// Exported re-type for Replay (mirrors types.ts shape)
// ────────────────────────────────────────────────────────────────────────────

// Re-export the Replay type from engine for consumers that only import
// from win-probability
export type { Replay } from './engine';
