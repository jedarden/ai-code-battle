// TypeScript game engine – mirrors the Go engine for in-browser use.
// Used by the sandbox page to run matches without a server.

export interface Position { row: number; col: number; }
export type Direction = 'N' | 'E' | 'S' | 'W' | '';
export interface Move { position: Position; direction: Direction; }

export interface Config {
  rows: number;
  cols: number;
  max_turns: number;
  vision_radius2: number;
  attack_radius2: number;
  spawn_cost: number;
  energy_interval: number;
}

export function defaultConfig(): Config {
  return {
    rows: 30, cols: 30, max_turns: 200,
    vision_radius2: 49, attack_radius2: 5,
    spawn_cost: 3, energy_interval: 10,
  };
}

export interface Bot { id: number; owner: number; position: Position; alive: boolean; }
export interface Core { position: Position; owner: number; active: boolean; }
export interface EnergyNode { position: Position; hasEnergy: boolean; tick: number; }
export interface Player { id: number; energy: number; score: number; botCount: number; }

export interface VisibleBot { position: Position; owner: number; }
export interface VisibleCore { position: Position; owner: number; active: boolean; }
export interface VisibleState {
  match_id: string;
  turn: number;
  config: Config;
  you: { id: number; energy: number; score: number; };
  bots: VisibleBot[];
  energy: Position[];
  cores: VisibleCore[];
  walls: Position[];
  dead: VisibleBot[];
}

export interface GameEvent {
  type: string;
  turn: number;
  details?: unknown;
}

export interface MatchResult {
  winner: number;
  reason: string;
  turns: number;
  scores: number[];
  energy: number[];
  bots_alive: number[];
}

export interface GameState {
  config: Config;
  bots: Bot[];
  cores: Core[];
  energy: EnergyNode[];
  players: Player[];
  turn: number;
  matchId: string;
  walls: Set<string>; // "row,col"
  events: GameEvent[];
  dominance: Map<number, number>;
}

// ────────────────────────────────────────────────────────────────────────────
// Utility helpers
// ────────────────────────────────────────────────────────────────────────────

export function posKey(p: Position): string { return `${p.row},${p.col}`; }

export function wrap(row: number, col: number, cfg: Config): Position {
  return { row: ((row % cfg.rows) + cfg.rows) % cfg.rows, col: ((col % cfg.cols) + cfg.cols) % cfg.cols };
}

export function applyDir(p: Position, dir: Direction, cfg: Config): Position {
  switch (dir) {
    case 'N': return wrap(p.row - 1, p.col, cfg);
    case 'S': return wrap(p.row + 1, p.col, cfg);
    case 'E': return wrap(p.row, p.col + 1, cfg);
    case 'W': return wrap(p.row, p.col - 1, cfg);
    default:  return p;
  }
}

export function dist2(a: Position, b: Position, cfg: Config): number {
  let dr = Math.abs(a.row - b.row);
  let dc = Math.abs(a.col - b.col);
  if (dr > cfg.rows / 2) dr = cfg.rows - dr;
  if (dc > cfg.cols / 2) dc = cfg.cols - dc;
  return dr * dr + dc * dc;
}

function randInt(max: number): number { return Math.floor(Math.random() * max); }
const DIRS: Direction[] = ['N', 'E', 'S', 'W'];

// ────────────────────────────────────────────────────────────────────────────
// Map generation (simplified cellular-automata)
// ────────────────────────────────────────────────────────────────────────────

export function generateMap(cfg: Config, seed?: number): { walls: Set<string>; cores: Core[]; energyNodes: EnergyNode[] } {
  // Simple deterministic map using linear congruential generator
  let s = seed ?? 42;
  const lcg = () => { s = (s * 1664525 + 1013904223) & 0xffffffff; return (s >>> 0) / 0x100000000; };

  const walls = new Set<string>();
  const numPlayers = 2;
  const rows = cfg.rows;
  const cols = cfg.cols;

  // Generate wall clusters avoiding cores & centres
  const wallProb = 0.15;
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      if (lcg() < wallProb) {
        // Rotation symmetry: place wall + 180° mirror
        walls.add(posKey({ row: r, col: c }));
        walls.add(posKey(wrap(rows - r - 1, cols - c - 1, cfg)));
      }
    }
  }

  // Player cores placed symmetrically
  const cores: Core[] = [];
  const corePositions: Position[] = [
    { row: Math.floor(rows * 0.25), col: Math.floor(cols * 0.25) },
    { row: Math.floor(rows * 0.75), col: Math.floor(cols * 0.75) },
  ];
  for (let i = 0; i < numPlayers; i++) {
    const p = corePositions[i] ?? wrap(i * Math.floor(rows / numPlayers), Math.floor(cols / 2), cfg);
    walls.delete(posKey(p)); // ensure core tile is clear
    cores.push({ position: p, owner: i, active: true });
  }

  // Energy nodes – 8% of tiles, avoiding walls and cores
  const energyNodes: EnergyNode[] = [];
  const coreSet = new Set(cores.map(c => posKey(c.position)));
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      const k = posKey({ row: r, col: c });
      if (!walls.has(k) && !coreSet.has(k) && lcg() < 0.08) {
        energyNodes.push({ position: { row: r, col: c }, hasEnergy: true, tick: 0 });
      }
    }
  }

  return { walls, cores, energyNodes };
}

// ────────────────────────────────────────────────────────────────────────────
// Game state initialization
// ────────────────────────────────────────────────────────────────────────────

export function newGame(cfg: Config, seed?: number): GameState {
  const { walls, cores, energyNodes } = generateMap(cfg, seed);

  const players: Player[] = [
    { id: 0, energy: 0, score: 0, botCount: 1 },
    { id: 1, energy: 0, score: 0, botCount: 1 },
  ];

  // Initial bots at each core
  const bots: Bot[] = cores.map((c, i) => ({
    id: i, owner: c.owner, position: { ...c.position }, alive: true,
  }));

  return {
    config: cfg,
    bots,
    cores,
    energy: energyNodes,
    players,
    turn: 0,
    matchId: `m_${Math.random().toString(36).slice(2, 10)}`,
    walls,
    events: [],
    dominance: new Map(),
  };
}

// ────────────────────────────────────────────────────────────────────────────
// Visibility / fog of war
// ────────────────────────────────────────────────────────────────────────────

export function getVisibleState(gs: GameState, playerID: number): VisibleState {
  const player = gs.players[playerID];
  if (!player) throw new Error(`no player ${playerID}`);

  const myBots = gs.bots.filter(b => b.alive && b.owner === playerID);

  // Compute visible positions (union of vision from all own bots)
  const visible = new Set<string>();
  for (const bot of myBots) {
    for (let dr = -10; dr <= 10; dr++) {
      for (let dc = -10; dc <= 10; dc++) {
        if (dr * dr + dc * dc <= gs.config.vision_radius2) {
          visible.add(posKey(wrap(bot.position.row + dr, bot.position.col + dc, gs.config)));
        }
      }
    }
  }

  const visibleBots: VisibleBot[] = [];
  for (const b of gs.bots) {
    if (b.alive && visible.has(posKey(b.position))) {
      visibleBots.push({ position: b.position, owner: b.owner });
    }
  }

  const visibleEnergy: Position[] = [];
  for (const en of gs.energy) {
    if (en.hasEnergy && visible.has(posKey(en.position))) {
      visibleEnergy.push(en.position);
    }
  }

  const visibleCores: VisibleCore[] = gs.cores
    .filter(c => visible.has(posKey(c.position)))
    .map(c => ({ position: c.position, owner: c.owner, active: c.active }));

  const visibleWalls: Position[] = [];
  for (const k of visible) {
    if (gs.walls.has(k)) {
      const [r, c] = k.split(',').map(Number);
      visibleWalls.push({ row: r, col: c });
    }
  }

  return {
    match_id: gs.matchId,
    turn: gs.turn,
    config: gs.config,
    you: { id: playerID, energy: player.energy, score: player.score },
    bots: visibleBots,
    energy: visibleEnergy,
    cores: visibleCores,
    walls: visibleWalls,
    dead: [],
  };
}

// ────────────────────────────────────────────────────────────────────────────
// Turn execution
// ────────────────────────────────────────────────────────────────────────────

export function executeTurn(gs: GameState, allMoves: Map<number, Move[]>): MatchResult | null {
  gs.turn++;
  gs.events = [];

  // Flatten moves: position key -> direction
  const moveMap = new Map<string, Direction>();
  for (const [, moves] of allMoves) {
    for (const m of moves) {
      moveMap.set(posKey(m.position), m.direction);
    }
  }

  // Phase 1: Movement
  const intended = new Map<number, Position>(); // bot id -> dest
  const destBots = new Map<string, Bot[]>();

  for (const b of gs.bots) {
    if (!b.alive) continue;
    const dir = moveMap.get(posKey(b.position)) ?? '';
    let dest = dir ? applyDir(b.position, dir as Direction, gs.config) : b.position;
    if (gs.walls.has(posKey(dest))) dest = b.position; // wall blocks
    intended.set(b.id, dest);
    const dk = posKey(dest);
    if (!destBots.has(dk)) destBots.set(dk, []);
    destBots.get(dk)!.push(b);
  }

  for (const b of gs.bots) {
    if (!b.alive) continue;
    const dest = intended.get(b.id)!;
    const dk = posKey(dest);
    const botsAtDest = destBots.get(dk)!;
    if (botsAtDest.length > 1) {
      // Check if same owner
      const sameOwner = botsAtDest.every(ob => ob.owner === b.owner);
      if (sameOwner) {
        for (const ob of botsAtDest) killBot(gs, ob, 'collision_death');
        continue;
      }
    }
    b.position = dest;
  }

  // Phase 2: Combat (bots within attack radius kill each other pairwise)
  const aliveBots = gs.bots.filter(b => b.alive);
  const killed = new Set<number>();
  for (let i = 0; i < aliveBots.length; i++) {
    for (let j = i + 1; j < aliveBots.length; j++) {
      const a = aliveBots[i], bBot = aliveBots[j];
      if (a.owner === bBot.owner) continue;
      if (dist2(a.position, bBot.position, gs.config) <= gs.config.attack_radius2) {
        killed.add(a.id);
        killed.add(bBot.id);
      }
    }
  }
  for (const id of killed) {
    const b = gs.bots.find(b => b.id === id);
    if (b) killBot(gs, b, 'combat_death');
  }

  // Phase 3: Energy collection
  const energyMap = new Map<string, EnergyNode>();
  for (const en of gs.energy) {
    if (en.hasEnergy) energyMap.set(posKey(en.position), en);
  }
  const botsOnEnergy = new Map<string, Bot[]>();
  for (const b of gs.bots) {
    if (!b.alive) continue;
    const ek = posKey(b.position);
    if (energyMap.has(ek)) {
      if (!botsOnEnergy.has(ek)) botsOnEnergy.set(ek, []);
      botsOnEnergy.get(ek)!.push(b);
    }
  }
  for (const [ek, bots] of botsOnEnergy) {
    // Contested energy: only one owner can collect
    const owners = new Set(bots.map(b => b.owner));
    if (owners.size === 1) {
      const owner = bots[0].owner;
      gs.players[owner].energy++;
      gs.players[owner].score++;
      energyMap.get(ek)!.hasEnergy = false;
      gs.events.push({ type: 'energy_collected', turn: gs.turn, details: { owner } });
    }
  }

  // Phase 4: Spawning (if enough energy)
  for (const p of gs.players) {
    if (p.energy >= gs.config.spawn_cost) {
      const myCore = gs.cores.find(c => c.owner === p.id && c.active);
      if (myCore) {
        p.energy -= gs.config.spawn_cost;
        const newBot: Bot = {
          id: gs.bots.length,
          owner: p.id,
          position: { ...myCore.position },
          alive: true,
        };
        gs.bots.push(newBot);
        p.botCount++;
        gs.events.push({ type: 'bot_spawned', turn: gs.turn, details: { owner: p.id } });
      }
    }
  }

  // Phase 5: Energy tick
  for (const en of gs.energy) {
    if (!en.hasEnergy) {
      en.tick++;
      if (en.tick >= gs.config.energy_interval) {
        en.hasEnergy = true;
        en.tick = 0;
      }
    }
  }

  // Phase 6: Core capture – enemy bots on undefended cores raze them
  for (const core of gs.cores) {
    if (!core.active) continue;
    const ck = posKey(core.position);
    const onCore = gs.bots.filter(b => b.alive && posKey(b.position) === ck);
    if (onCore.length > 0) {
      const owners = new Set(onCore.map(b => b.owner));
      if (!owners.has(core.owner) && owners.size === 1) {
        core.active = false;
        gs.events.push({ type: 'core_captured', turn: gs.turn, details: { coreOwner: core.owner, captureOwner: [...owners][0] } });
      }
    }
  }

  // Phase 7: Dominance check
  for (const p of gs.players) {
    const alive = gs.bots.filter(b => b.alive);
    const myCount = alive.filter(b => b.owner === p.id).length;
    const total = alive.length;
    if (total > 0 && myCount / total >= 0.8) {
      gs.dominance.set(p.id, (gs.dominance.get(p.id) ?? 0) + 1);
      if (gs.dominance.get(p.id)! >= 100) {
        return buildResult(gs, p.id, 'dominance');
      }
    } else {
      gs.dominance.set(p.id, 0);
    }
  }

  // Check for elimination
  for (const p of gs.players) {
    const alive = gs.bots.filter(b => b.alive && b.owner === p.id);
    const hasCore = gs.cores.some(c => c.owner === p.id && c.active);
    if (alive.length === 0 && !hasCore) {
      // This player is eliminated; find the remaining player
      const survivors = gs.players.filter(op => {
        const opAlive = gs.bots.filter(b => b.alive && b.owner === op.id);
        const opCore = gs.cores.some(c => c.owner === op.id && c.active);
        return opAlive.length > 0 || opCore;
      });
      if (survivors.length === 1) {
        return buildResult(gs, survivors[0].id, 'elimination');
      }
    }
  }

  // Turn limit
  if (gs.turn >= gs.config.max_turns) {
    // Winner by score
    const maxScore = Math.max(...gs.players.map(p => p.score));
    const winners = gs.players.filter(p => p.score === maxScore);
    const winner = winners.length === 1 ? winners[0].id : -1;
    return buildResult(gs, winner, winner >= 0 ? 'turns' : 'draw');
  }

  return null;
}

function killBot(gs: GameState, b: Bot, reason: string): void {
  b.alive = false;
  gs.players[b.owner].botCount = Math.max(0, gs.players[b.owner].botCount - 1);
  gs.events.push({ type: 'bot_died', turn: gs.turn, details: { owner: b.owner, reason } });
}

function buildResult(gs: GameState, winner: number, reason: string): MatchResult {
  return {
    winner,
    reason,
    turns: gs.turn,
    scores: gs.players.map(p => p.score),
    energy: gs.players.map(p => p.energy),
    bots_alive: gs.players.map(p => gs.bots.filter(b => b.alive && b.owner === p.id).length),
  };
}

// ────────────────────────────────────────────────────────────────────────────
// Built-in bot strategy implementations (TypeScript)
// ────────────────────────────────────────────────────────────────────────────

export type BotStrategy = (state: VisibleState) => Move[];

export function randomStrategy(state: VisibleState): Move[] {
  const myID = state.you.id;
  return state.bots
    .filter(b => b.owner === myID)
    .map(b => ({ position: b.position, direction: DIRS[randInt(4)] }));
}

export function gathererStrategy(state: VisibleState): Move[] {
  const myID = state.you.id;
  const energySet = new Set(state.energy.map(posKey));
  const enemySet = new Set(state.bots.filter(b => b.owner !== myID).map(b => posKey(b.position)));
  const cfg = state.config;

  return state.bots
    .filter(b => b.owner === myID)
    .map(b => {
      let dir = fleeFrom(b.position, enemySet, cfg);
      if (!dir) dir = toward(b.position, energySet, cfg);
      return { position: b.position, direction: dir ?? DIRS[randInt(4)] };
    });
}

export function rusherStrategy(state: VisibleState): Move[] {
  const myID = state.you.id;
  const cfg = state.config;
  const coreSet = new Set(state.cores.filter(c => c.owner !== myID && c.active).map(c => posKey(c.position)));
  const enemySet = new Set(state.bots.filter(b => b.owner !== myID).map(b => posKey(b.position)));

  return state.bots
    .filter(b => b.owner === myID)
    .map(b => {
      const targets = coreSet.size > 0 ? coreSet : enemySet;
      const dir = toward(b.position, targets, cfg) ?? DIRS[randInt(4)];
      return { position: b.position, direction: dir };
    });
}

export function guardianStrategy(state: VisibleState): Move[] {
  const myID = state.you.id;
  const cfg = state.config;
  const myCoreSet = new Set(state.cores.filter(c => c.owner === myID && c.active).map(c => posKey(c.position)));
  const enemySet = new Set(state.bots.filter(b => b.owner !== myID).map(b => posKey(b.position)));

  return state.bots
    .filter(b => b.owner === myID)
    .map(b => {
      let dir: Direction | null = null;
      if (isNearSet(b.position, enemySet, cfg, cfg.attack_radius2 + 4)) {
        dir = toward(b.position, enemySet, cfg);
      } else {
        dir = toward(b.position, myCoreSet, cfg);
      }
      return { position: b.position, direction: dir ?? DIRS[randInt(4)] };
    });
}

export function swarmStrategy(state: VisibleState): Move[] {
  const myID = state.you.id;
  const cfg = state.config;
  const myBots = state.bots.filter(b => b.owner === myID);

  return myBots.map(b => {
    let best: Direction = 'N';
    let bestScore = -Infinity;
    for (const d of DIRS) {
      const np = applyDir(b.position, d, cfg);
      const score = myBots.reduce((s, ob) => s + dist2(np, ob.position, cfg), 0);
      if (score > bestScore) { bestScore = score; best = d; }
    }
    return { position: b.position, direction: best };
  });
}

export function hunterStrategy(state: VisibleState): Move[] {
  const myID = state.you.id;
  const cfg = state.config;
  const enemySet = new Set(state.bots.filter(b => b.owner !== myID).map(b => posKey(b.position)));
  const energySet = new Set(state.energy.map(posKey));

  return state.bots
    .filter(b => b.owner === myID)
    .map(b => {
      const targets = enemySet.size > 0 ? enemySet : energySet;
      const dir = toward(b.position, targets, cfg) ?? DIRS[randInt(4)];
      return { position: b.position, direction: dir };
    });
}

export const BUILTIN_STRATEGIES: Record<string, BotStrategy> = {
  random: randomStrategy,
  gatherer: gathererStrategy,
  rusher: rusherStrategy,
  guardian: guardianStrategy,
  swarm: swarmStrategy,
  hunter: hunterStrategy,
};

// ────────────────────────────────────────────────────────────────────────────
// Strategy helpers
// ────────────────────────────────────────────────────────────────────────────

function toward(from: Position, targets: Set<string>, cfg: Config): Direction | null {
  if (targets.size === 0) return null;
  let best: Direction | null = null;
  let bestD = Infinity;
  for (const d of DIRS) {
    const np = applyDir(from, d, cfg);
    for (const k of targets) {
      const [r, c] = k.split(',').map(Number);
      const d2 = dist2(np, { row: r, col: c }, cfg);
      if (d2 < bestD) { bestD = d2; best = d; }
    }
  }
  return best;
}

function fleeFrom(from: Position, enemies: Set<string>, cfg: Config): Direction | null {
  const thr = cfg.attack_radius2 + 4;
  let close = false;
  for (const k of enemies) {
    const [r, c] = k.split(',').map(Number);
    if (dist2(from, { row: r, col: c }, cfg) <= thr) { close = true; break; }
  }
  if (!close) return null;
  let best: Direction | null = null;
  let bestD = -1;
  for (const d of DIRS) {
    const np = applyDir(from, d, cfg);
    let minD = Infinity;
    for (const k of enemies) {
      const [r, c] = k.split(',').map(Number);
      const d2 = dist2(np, { row: r, col: c }, cfg);
      if (d2 < minD) minD = d2;
    }
    if (minD > bestD) { bestD = minD; best = d; }
  }
  return best;
}

function isNearSet(from: Position, targets: Set<string>, cfg: Config, r2: number): boolean {
  for (const k of targets) {
    const [r, c] = k.split(',').map(Number);
    if (dist2(from, { row: r, col: c }, cfg) <= r2) return true;
  }
  return false;
}

// ────────────────────────────────────────────────────────────────────────────
// Match runner
// ────────────────────────────────────────────────────────────────────────────

export interface ReplayTurn {
  turn: number;
  bots: { id: number; owner: number; position: Position; alive: boolean }[];
  cores: { position: Position; owner: number; active: boolean }[];
  energy: Position[];
  scores: number[];
  energy_held: number[];
  events: GameEvent[];
}

export interface Replay {
  match_id: string;
  config: Config;
  result: MatchResult;
  players: { id: number; name: string }[];
  map: { rows: number; cols: number; walls: Position[]; cores: { position: Position; owner: number }[]; energy_nodes: Position[] };
  turns: ReplayTurn[];
}

export function runMatch(
  cfg: Config,
  strategy1: BotStrategy | string,
  strategy2: BotStrategy | string,
  seed?: number,
): { replay: Replay; result: MatchResult } {
  const s1 = typeof strategy1 === 'string' ? BUILTIN_STRATEGIES[strategy1] ?? randomStrategy : strategy1;
  const s2 = typeof strategy2 === 'string' ? BUILTIN_STRATEGIES[strategy2] ?? randomStrategy : strategy2;

  const gs = newGame(cfg, seed);

  const wallPositions: Position[] = [];
  for (const k of gs.walls) {
    const [r, c] = k.split(',').map(Number);
    wallPositions.push({ row: r, col: c });
  }

  const turns: ReplayTurn[] = [];

  function recordTurn(): ReplayTurn {
    return {
      turn: gs.turn,
      bots: gs.bots.map(b => ({ ...b })),
      cores: gs.cores.map(c => ({ ...c })),
      energy: gs.energy.filter(e => e.hasEnergy).map(e => e.position),
      scores: gs.players.map(p => p.score),
      energy_held: gs.players.map(p => p.energy),
      events: [...gs.events],
    };
  }

  turns.push(recordTurn());

  let result: MatchResult | null = null;
  while (!result) {
    const allMoves = new Map<number, Move[]>();
    for (const p of gs.players) {
      const visible = getVisibleState(gs, p.id);
      const strategy = p.id === 0 ? s1 : s2;
      try {
        allMoves.set(p.id, strategy(visible));
      } catch {
        allMoves.set(p.id, []);
      }
    }
    result = executeTurn(gs, allMoves);
    turns.push(recordTurn());
  }

  const replay: Replay = {
    match_id: gs.matchId,
    config: cfg,
    result,
    players: [{ id: 0, name: typeof strategy1 === 'string' ? strategy1 : 'custom' },
              { id: 1, name: typeof strategy2 === 'string' ? strategy2 : 'opponent' }],
    map: {
      rows: cfg.rows,
      cols: cfg.cols,
      walls: wallPositions,
      cores: gs.cores.map(c => ({ position: c.position, owner: c.owner })),
      energy_nodes: gs.energy.map(e => e.position),
    },
    turns,
  };

  return { replay, result };
}
