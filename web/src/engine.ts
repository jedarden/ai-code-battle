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
  zone?: { center: Position; radius: number; active: boolean }; // Go engine only; TS engine has no zone
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
  combat_deaths?: number[];
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

export function generateMap(cfg: Config, seed?: number, numPlayers = 2): { walls: Set<string>; cores: Core[]; energyNodes: EnergyNode[] } {
  // Simple deterministic map using linear congruential generator
  let s = seed ?? 42;
  const lcg = () => { s = (s * 1664525 + 1013904223) & 0xffffffff; return (s >>> 0) / 0x100000000; };

  const walls = new Set<string>();
  const rows = cfg.rows;
  const cols = cfg.cols;

  // Generate wall clusters with rotational symmetry for all players
  const wallProb = 0.12;
  const symmetryDiv = numPlayers;
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      if (lcg() < wallProb) {
        for (let i = 0; i < symmetryDiv; i++) {
          const angle = (2 * Math.PI * i) / symmetryDiv;
          const cr = rows / 2, cc = cols / 2;
          const dr = r - cr, dc = c - cc;
          const nr = Math.round(cr + dr * Math.cos(angle) - dc * Math.sin(angle));
          const nc = Math.round(cc + dr * Math.sin(angle) + dc * Math.cos(angle));
          const wp = wrap(nr, nc, cfg);
          walls.add(posKey(wp));
        }
      }
    }
  }

  // Player cores placed symmetrically
  const cores: Core[] = [];
  const corePositions: Position[] = [];
  const cx = rows / 2, cy = cols / 2;
  const coreRadius = Math.min(rows, cols) * 0.35;
  for (let i = 0; i < numPlayers; i++) {
    const angle = (2 * Math.PI * i) / numPlayers - Math.PI / 2;
    corePositions.push({
      row: Math.round(cx + coreRadius * Math.cos(angle)),
      col: Math.round(cy + coreRadius * Math.sin(angle)),
    });
  }
  for (let i = 0; i < numPlayers; i++) {
    const p = wrap(corePositions[i].row, corePositions[i].col, cfg);
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

export function newGame(cfg: Config, seed?: number, numPlayers = 2): GameState {
  const { walls, cores, energyNodes } = generateMap(cfg, seed, numPlayers);

  const players: Player[] = Array.from({ length: numPlayers }, (_, i) => ({
    id: i, energy: 0, score: 0, botCount: 1,
  }));

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

// ────────────────────────────────────────────────────────────────────────────
// Extended roster (plan §13.1) — TS mirrors of the Go ports in
// cmd/acb-wasm/strategies/strategies.go, which port the ladder bots under
// bots/. The sandbox runs built-in opponents through these functions on both
// engine paths (TS engine directly, Go WASM engine via JS callbacks).
// ────────────────────────────────────────────────────────────────────────────

/** Per-match state for stateful strategies, keyed by match_id:player_id so two
 *  same-strategy opponents in one match keep independent state. */
function stateKey(state: VisibleState): string {
  return `${state.match_id}:${state.you.id}`;
}

// Farmer — maximizes energy collection, avoids combat (bots/farmer).
export function farmerStrategy(state: VisibleState): Move[] {
  const myID = state.you.id;
  const cfg = state.config;
  const wallSet = new Set(state.walls.map(posKey));
  const enemySet = new Set(state.bots.filter(b => b.owner !== myID).map(b => posKey(b.position)));
  const enemyList = state.bots.filter(b => b.owner !== myID).map(b => b.position);
  const energySet = new Set(state.energy.map(posKey));
  const myCores = state.cores.filter(c => c.owner === myID && c.active).map(c => c.position);
  const myBots = state.bots.filter(b => b.owner === myID);

  // Energy tiles adjacent (≤√2) to an enemy are contested — collecting there
  // destroys the energy node instead.
  const contestedEnergy = new Set<string>();
  for (const e of state.energy) {
    for (const ep of enemyList) {
      if (dist2(e, ep, cfg) <= 2) { contestedEnergy.add(posKey(e)); break; }
    }
  }

  const assignedEnergy = new Set<string>();
  const claimedDests = new Set<string>();
  const passable = (p: Position): boolean => !wallSet.has(posKey(p)) && !enemySet.has(posKey(p));

  // Process bots closest to uncontested energy first.
  const order = myBots
    .map((bot, i) => {
      let bestDist = Infinity;
      for (const e of state.energy) {
        if (contestedEnergy.has(posKey(e))) continue;
        const d = dist2(bot.position, e, cfg);
        if (d < bestDist) bestDist = d;
      }
      return { i, bot, score: bestDist };
    })
    .sort((a, b) => a.score - b.score);

  const moves: Move[] = [];
  for (const { bot } of order) {
    const dir = farmerBotMove(
      bot.position, state.bots, myID, energySet, enemyList,
      wallSet, enemySet, myCores, state.energy,
      contestedEnergy, assignedEnergy, claimedDests, passable, cfg,
    );

    let dest = bot.position;
    if (dir !== null) dest = applyDir(bot.position, dir, cfg);
    // Hold when the intended destination is already claimed this turn.
    const held = dir !== null && claimedDests.has(posKey(dest));
    claimedDests.add(posKey(held ? bot.position : dest));
    if (dir !== null && !held) {
      moves.push({ position: bot.position, direction: dir });
    }
  }
  return moves;
}

function farmerBotMove(
  pos: Position, bots: VisibleBot[], myID: number, energySet: Set<string>, enemyList: Position[],
  wallSet: Set<string>, enemySet: Set<string>, myCores: Position[], energy: Position[],
  contestedEnergy: Set<string>, assignedEnergy: Set<string>, claimedDests: Set<string>,
  passable: (p: Position) => boolean, cfg: Config,
): Direction | null {
  // Priority 1: flee when locally outnumbered within attack range.
  if (farmerShouldFlee(pos, bots, myID, cfg)) {
    const dir = maximizeMinDist(pos, enemyList, wallSet, enemySet, cfg);
    if (dir !== null) return dir;
  }

  // Priority 2: seek nearest uncontested, unassigned energy.
  let bestDist = Infinity;
  let bestEnergy: Position | null = null;
  for (const e of energy) {
    if (contestedEnergy.has(posKey(e)) || assignedEnergy.has(posKey(e))) continue;
    const d = dist2(pos, e, cfg);
    if (d < bestDist) { bestDist = d; bestEnergy = e; }
  }
  if (bestEnergy) {
    assignedEnergy.add(posKey(bestEnergy));
    const dir = bfsDirection(pos, bestEnergy, passable, cfg);
    if (dir !== null) return dir;
  }

  // Priority 3: already standing on energy — hold to collect.
  if (energySet.has(posKey(pos))) return null;

  // Priority 4: move toward nearest energy, even contested.
  if (energy.length > 0) {
    let bd = Infinity;
    let target = energy[0];
    for (const e of energy) {
      const d = dist2(pos, e, cfg);
      if (d < bd) { bd = d; target = e; }
    }
    const dir = bfsDirection(pos, target, passable, cfg);
    if (dir !== null) return dir;
  }

  // Priority 5: stay near an active core for spawning.
  if (myCores.length > 0) {
    let ncd = Infinity;
    let nearestCore = myCores[0];
    for (const c of myCores) {
      const d = dist2(pos, c, cfg);
      if (d < ncd) { ncd = d; nearestCore = c; }
    }
    if (ncd > 4) {
      const dir = bfsDirection(pos, nearestCore, passable, cfg);
      if (dir !== null) return dir;
    }
  }

  // Priority 6: spread out from friendly bots.
  return spreadFromBots(pos, bots, myID, claimedDests, cfg);
}

function farmerShouldFlee(pos: Position, bots: VisibleBot[], myID: number, cfg: Config): boolean {
  let nearbyEnemies = 0, nearbyAllies = 0;
  for (const b of bots) {
    if (posKey(b.position) === posKey(pos)) continue;
    if (dist2(pos, b.position, cfg) <= cfg.attack_radius2) {
      if (b.owner === myID) nearbyAllies++;
      else nearbyEnemies++;
    }
  }
  return nearbyEnemies > 0 && nearbyAllies < nearbyEnemies;
}

function maximizeMinDist(
  pos: Position, enemies: Position[], wallSet: Set<string>, enemySet: Set<string>, cfg: Config,
): Direction | null {
  let bestDir: Direction | null = null;
  let bestMinDist = -1;
  for (const st of cardinalSteps(pos, cfg)) {
    if (wallSet.has(posKey(st.pos)) || enemySet.has(posKey(st.pos))) continue;
    let minDist = Infinity;
    for (const ep of enemies) {
      const d = dist2(st.pos, ep, cfg);
      if (d < minDist) minDist = d;
    }
    if (minDist > bestMinDist) { bestMinDist = minDist; bestDir = st.dir; }
  }
  return bestDir;
}

function spreadFromBots(
  pos: Position, bots: VisibleBot[], myID: number, claimedDests: Set<string>, cfg: Config,
): Direction | null {
  let bestDir: Direction | null = null;
  let bestScore = -1;
  for (const st of cardinalSteps(pos, cfg)) {
    if (claimedDests.has(posKey(st.pos))) continue;
    let minDist = Infinity;
    for (const b of bots) {
      if (b.owner !== myID) continue;
      const d = dist2(st.pos, b.position, cfg);
      if (d < minDist) minDist = d;
    }
    if (minDist > bestScore) { bestScore = minDist; bestDir = st.dir; }
  }
  return bestDir;
}

// Opportunist — targets the weakest visible enemy, fights only with local
// numerical advantage, retreats and farms otherwise (bots/opportunist).
const OPP_ENGAGE_RADIUS2 = 25;
const OPP_RETREAT_RADIUS2 = 9;
const OPP_PATROL_RADIUS = 8;
const OPP_ENERGY_SEEK_RANGE2 = 100;

export function opportunistStrategy(state: VisibleState): Move[] {
  const myID = state.you.id;
  const cfg = state.config;
  const wallSet = new Set(state.walls.map(posKey));
  const enemyBots = state.bots.filter(b => b.owner !== myID);
  const enemySet = new Set(enemyBots.map(b => posKey(b.position)));
  const myBots = state.bots.filter(b => b.owner === myID).map(b => b.position);
  const myCores = state.cores.filter(c => c.owner === myID && c.active).map(c => c.position);
  const passable = (p: Position): boolean => !wallSet.has(posKey(p)) && !enemySet.has(posKey(p));

  const targets = oppScoreTargets(enemyBots, myBots, cfg);
  const assignments = oppAssignAttackers(targets, myBots, cfg);
  const claimedDests = new Set<string>();

  const moves: Move[] = [];
  for (const bot of myBots) {
    let dir: Direction | null;
    const target = assignments.get(posKey(bot));
    if (target) {
      dir = bfsDirection(bot, target, p => posKey(p) === posKey(target) || passable(p), cfg);
    } else if (oppShouldFlee(bot, enemyBots, myBots, cfg)) {
      dir = oppRetreatMove(bot, myBots, enemySet, wallSet, cfg);
      if (dir === null) dir = oppEnergyMove(bot, state.energy, passable, claimedDests, cfg);
    } else {
      dir = oppEconomyOrPatrol(bot, state.energy, myCores, passable, claimedDests, cfg);
    }

    let dest = bot;
    if (dir !== null) dest = applyDir(bot, dir, cfg);
    const held = dir !== null && claimedDests.has(posKey(dest));
    claimedDests.add(posKey(held ? bot : dest));
    if (dir !== null && !held) {
      moves.push({ position: bot, direction: dir });
    }
  }
  return moves;
}

interface OppTarget {
  pos: Position;
  score: number;
  localAlly: number;
  localEnemy: number;
}

function oppScoreTargets(enemies: VisibleBot[], myBots: Position[], cfg: Config): OppTarget[] {
  const targets: OppTarget[] = [];
  for (const e of enemies) {
    let isolation = 10.0;
    let minFriendly = Infinity;
    for (const other of enemies) {
      if (posKey(other.position) === posKey(e.position) || other.owner !== e.owner) continue;
      const d = dist2(e.position, other.position, cfg);
      if (d < minFriendly) minFriendly = d;
    }
    if (minFriendly !== Infinity) isolation = Math.sqrt(minFriendly);

    let localAlly = 0, localEnemy = 0;
    for (const mb of myBots) {
      if (dist2(mb, e.position, cfg) <= OPP_ENGAGE_RADIUS2) localAlly++;
    }
    for (const oe of enemies) {
      if (dist2(oe.position, e.position, cfg) <= OPP_ENGAGE_RADIUS2) localEnemy++;
    }
    const vulnerability = localEnemy > 0 ? 1.0 / localEnemy : 1.0;

    targets.push({ pos: e.position, score: isolation * vulnerability, localAlly, localEnemy });
  }
  return targets.sort((a, b) => b.score - a.score);
}

function oppAssignAttackers(targets: OppTarget[], myBots: Position[], cfg: Config): Map<string, Position> {
  const assignments = new Map<string, Position>();
  const assigned = new Set<string>();

  for (const tgt of targets) {
    if (tgt.localAlly < tgt.localEnemy) continue;
    const candidates = myBots
      .filter(mb => !assigned.has(posKey(mb)))
      .map(mb => ({ pos: mb, dist: dist2(mb, tgt.pos, cfg) }))
      .filter(c => c.dist <= OPP_ENGAGE_RADIUS2 * 2)
      .sort((a, b) => a.dist - b.dist);

    const wantCount = Math.max(2, tgt.localEnemy + 1);
    for (let i = 0; i < candidates.length && i < wantCount; i++) {
      assignments.set(posKey(candidates[i].pos), tgt.pos);
      assigned.add(posKey(candidates[i].pos));
    }
  }
  return assignments;
}

function oppShouldFlee(bot: Position, enemies: VisibleBot[], myBots: Position[], cfg: Config): boolean {
  let nearbyEnemies = 0;
  for (const e of enemies) {
    if (dist2(bot, e.position, cfg) <= OPP_RETREAT_RADIUS2) nearbyEnemies++;
  }
  if (nearbyEnemies === 0) return false;
  let nearbyAllies = 0;
  for (const mb of myBots) {
    if (posKey(mb) === posKey(bot)) continue;
    if (dist2(bot, mb, cfg) <= OPP_RETREAT_RADIUS2) nearbyAllies++;
  }
  return nearbyAllies < nearbyEnemies;
}

function oppRetreatMove(
  bot: Position, myBots: Position[], enemySet: Set<string>, wallSet: Set<string>, cfg: Config,
): Direction | null {
  let bestDir: Direction | null = null;
  let bestScore = -1;
  for (const st of cardinalSteps(bot, cfg)) {
    if (wallSet.has(posKey(st.pos)) || enemySet.has(posKey(st.pos))) continue;
    let score = 0;
    for (const mb of myBots) {
      if (posKey(mb) === posKey(bot)) continue;
      const d = torManhattan(st.pos, mb, cfg);
      if (d > 0) score += Math.floor(100 / d);
    }
    for (const ek of enemySet) {
      const [r, c] = ek.split(',').map(Number);
      score += dist2(st.pos, { row: r, col: c }, cfg);
    }
    if (score > bestScore) { bestScore = score; bestDir = st.dir; }
  }
  return bestDir;
}

function oppEconomyOrPatrol(
  bot: Position, energy: Position[], cores: Position[],
  passable: (p: Position) => boolean, claimedDests: Set<string>, cfg: Config,
): Direction | null {
  const energyDir = oppEnergyMove(bot, energy, passable, claimedDests, cfg);
  if (energyDir !== null) return energyDir;

  if (cores.length > 0) {
    let ncd = Infinity;
    let nearestCore = cores[0];
    for (const c of cores) {
      const d = dist2(bot, c, cfg);
      if (d < ncd) { ncd = d; nearestCore = c; }
    }
    if (ncd > OPP_PATROL_RADIUS * OPP_PATROL_RADIUS) {
      const dir = bfsDirection(bot, nearestCore, passable, cfg);
      if (dir !== null) return dir;
    }
  }

  // Spread out to avoid clustering.
  let bestDir: Direction | null = null;
  let bestScore = -1;
  for (const st of cardinalSteps(bot, cfg)) {
    if (claimedDests.has(posKey(st.pos))) continue;
    let score = 0;
    for (const dk of claimedDests) {
      const [r, c] = dk.split(',').map(Number);
      const d = dist2(st.pos, { row: r, col: c }, cfg);
      if (d > 0) score += d;
    }
    if (score > bestScore) { bestScore = score; bestDir = st.dir; }
  }
  return bestDir;
}

function oppEnergyMove(
  bot: Position, energy: Position[], passable: (p: Position) => boolean,
  claimedDests: Set<string>, cfg: Config,
): Direction | null {
  let bestDist = Infinity;
  let target: Position | null = null;
  for (const e of energy) {
    if (claimedDests.has(posKey(e))) continue;
    const d = dist2(bot, e, cfg);
    if (d < bestDist && d <= OPP_ENERGY_SEEK_RANGE2) { bestDist = d; target = e; }
  }
  if (!target) return null;
  return bfsDirection(bot, target, passable, cfg);
}

// Siege — spawn-lockout: occupies tiles around enemy cores so they cannot
// respawn; unassigned bots farm energy (bots/siege).
export function siegeStrategy(state: VisibleState): Move[] {
  const myID = state.you.id;
  const cfg = state.config;
  const myBots = state.bots.filter(b => b.owner === myID);
  const enemyBots = state.bots.filter(b => b.owner !== myID);
  const enemySet = new Set(enemyBots.map(b => posKey(b.position)));
  const wallSet = new Set(state.walls.map(posKey));
  const energySet = new Set(state.energy.map(posKey));
  const enemyCores = state.cores.filter(c => c.owner !== myID && c.active);

  const occupied = new Set(myBots.map(b => posKey(b.position)));
  const moves: Move[] = [];
  const assignedBots = new Set<string>();

  // PHASE 1: assign bots to lockout rings around enemy cores (greedy by distance).
  const lockout = siegeAssignLockout(myBots, enemyCores, enemySet, wallSet, occupied, cfg);
  for (const [botPos, target] of lockout) {
    const dir = siegeStepToward(botPos, target, enemySet, wallSet, occupied, cfg);
    if (dir !== null) {
      moves.push({ position: botPos, direction: dir });
      assignedBots.add(posKey(botPos));
      occupied.add(posKey(applyDir(botPos, dir, cfg)));
    }
  }

  // PHASE 2: unassigned bots survive the zone, flee, then collect energy.
  const usedEnergy = new Set<string>();
  for (const bot of myBots) {
    if (assignedBots.has(posKey(bot.position))) continue;

    // Zone awareness: survival first.
    if (state.zone?.active) {
      const d2 = dist2(bot.position, state.zone.center, cfg);
      const safetyMargin2 = 9; // (3 tiles)^2
      const r = state.zone.radius;
      if (d2 >= r * r - safetyMargin2) {
        const dir = siegeStepToward(bot.position, state.zone.center, enemySet, wallSet, occupied, cfg);
        if (dir !== null) {
          moves.push({ position: bot.position, direction: dir });
          occupied.add(posKey(applyDir(bot.position, dir, cfg)));
          continue;
        }
      }
    }

    // Flee when locally outnumbered.
    if (siegeShouldFlee(bot.position, myBots, enemyBots, cfg)) {
      const dir = siegeFleeDir(bot.position, enemyBots, wallSet, cfg);
      if (dir !== null) {
        moves.push({ position: bot.position, direction: dir });
        occupied.add(posKey(applyDir(bot.position, dir, cfg)));
        continue;
      }
    }

    // Collect adjacent energy (immediate gain).
    let collected = false;
    for (const dir of DIRS) {
      const adj = applyDir(bot.position, dir, cfg);
      const ak = posKey(adj);
      if (energySet.has(ak) && !usedEnergy.has(ak) &&
          !wallSet.has(ak) && !enemySet.has(ak) && !occupied.has(ak)) {
        moves.push({ position: bot.position, direction: dir });
        usedEnergy.add(ak);
        occupied.add(ak);
        collected = true;
        break;
      }
    }
    if (collected) continue;

    // BFS toward nearest untargeted energy.
    const path = siegeNearestEnergyDir(bot.position, energySet, usedEnergy, wallSet, enemySet, occupied, cfg);
    if (path !== null) {
      moves.push({ position: bot.position, direction: path });
      occupied.add(posKey(applyDir(bot.position, path, cfg)));
      continue;
    }

    // No energy — advance toward the nearest enemy core.
    if (enemyCores.length > 0) {
      let nearest = enemyCores[0];
      let minDist = dist2(bot.position, nearest.position, cfg);
      for (const core of enemyCores.slice(1)) {
        const d = dist2(bot.position, core.position, cfg);
        if (d < minDist) { minDist = d; nearest = core; }
      }
      const dir = siegeStepToward(bot.position, nearest.position, enemySet, wallSet, occupied, cfg);
      if (dir !== null) {
        moves.push({ position: bot.position, direction: dir });
        occupied.add(posKey(applyDir(bot.position, dir, cfg)));
      }
    }
  }
  return moves;
}

function siegeAssignLockout(
  myBots: VisibleBot[], enemyCores: VisibleCore[],
  enemySet: Set<string>, wallSet: Set<string>, occupied: Set<string>, cfg: Config,
): Map<Position, Position> {
  const slots: { pos: Position; blocked: boolean }[] = [];
  for (const core of enemyCores) {
    for (const [dr, dc] of [[-1, -1], [-1, 0], [-1, 1], [0, -1], [0, 1], [1, -1], [1, 0], [1, 1]]) {
      const p = wrap(core.position.row + dr, core.position.col + dc, cfg);
      const pk = posKey(p);
      if (wallSet.has(pk) || enemySet.has(pk)) continue;
      // A slot already held by a friendly bot is covered — block it from
      // assignment, like the ladder original.
      slots.push({ pos: p, blocked: occupied.has(pk) });
    }
  }

  const assignments = new Map<Position, Position>();
  for (;;) {
    let bestSlot = -1, bestBot: Position | null = null, bestDist = Infinity;
    const targeted = new Set([...assignments.values()].map(posKey));
    for (const bot of myBots) {
      if (assignments.has(bot.position)) continue;
      for (let si = 0; si < slots.length; si++) {
        const s = slots[si];
        if (s.blocked || targeted.has(posKey(s.pos))) continue;
        const d = dist2(bot.position, s.pos, cfg);
        if (d < bestDist) { bestDist = d; bestSlot = si; bestBot = bot.position; }
      }
    }
    if (bestBot === null || bestSlot === -1) break;
    assignments.set(bestBot, slots[bestSlot].pos);
    slots[bestSlot].blocked = true;
  }
  return assignments;
}

function siegeShouldFlee(pos: Position, myBots: VisibleBot[], enemyBots: VisibleBot[], cfg: Config): boolean {
  let nearbyEnemies = 0;
  for (const enemy of enemyBots) {
    if (dist2(pos, enemy.position, cfg) <= cfg.attack_radius2) nearbyEnemies++;
  }
  if (nearbyEnemies === 0) return false;
  let nearbyAllies = 0;
  for (const ally of myBots) {
    if (posKey(ally.position) === posKey(pos)) continue;
    if (dist2(pos, ally.position, cfg) <= cfg.attack_radius2) nearbyAllies++;
  }
  return nearbyAllies < nearbyEnemies;
}

function siegeFleeDir(pos: Position, enemies: VisibleBot[], wallSet: Set<string>, cfg: Config): Direction | null {
  let center: Position = { row: 0, col: 0 };
  for (const enemy of enemies) {
    center.row += enemy.position.row;
    center.col += enemy.position.col;
  }
  if (enemies.length > 0) {
    center.row = Math.trunc(center.row / enemies.length);
    center.col = Math.trunc(center.col / enemies.length);
  }
  let bestDir: Direction | null = null;
  let bestDist = -1;
  for (const dir of DIRS) {
    const np = applyDir(pos, dir, cfg);
    if (wallSet.has(posKey(np))) continue;
    const d = dist2(np, center, cfg);
    if (d > bestDist) { bestDist = d; bestDir = dir; }
  }
  return bestDir;
}

function siegeNearestEnergyDir(
  start: Position, energySet: Set<string>, usedEnergy: Set<string>,
  wallSet: Set<string>, enemySet: Set<string>, occupied: Set<string>, cfg: Config,
): Direction | null {
  const visited = new Set<string>();
  let queue: { pos: Position; path: Direction[] }[] = [{ pos: start, path: [] }];

  while (queue.length > 0) {
    const item = queue.shift()!;
    const k = posKey(item.pos);
    if (visited.has(k)) continue;
    visited.add(k);

    if (energySet.has(k) && !usedEnergy.has(k)) {
      if (item.path.length === 0) return null;
      return item.path[0];
    }
    if (item.path.length > 20) continue;
    for (const dir of DIRS) {
      const next = applyDir(item.pos, dir, cfg);
      const nk = posKey(next);
      if (wallSet.has(nk) || enemySet.has(nk) || occupied.has(nk)) continue;
      if (!visited.has(nk)) {
        queue.push({ pos: next, path: [...item.path, dir] });
      }
    }
  }
  return null;
}

function siegeStepToward(
  pos: Position, target: Position, enemySet: Set<string>,
  wallSet: Set<string>, occupied: Set<string>, cfg: Config,
): Direction | null {
  let bestDir: Direction | null = null;
  let bestDist = Infinity;
  for (const dir of DIRS) {
    const np = applyDir(pos, dir, cfg);
    const nk = posKey(np);
    if (wallSet.has(nk) || enemySet.has(nk) || occupied.has(nk)) continue;
    const d = dist2(np, target, cfg);
    if (d < bestDist) { bestDist = d; bestDir = dir; }
  }
  return bestDir;
}

// Economist — wins by energy starvation: contests nodes enemies are
// approaching so the energy is destroyed, harvests uncontested nodes
// (bots/economist, Python).
const ECON_ADJACENT_RADIUS2 = 2;
let econStateKey = '';
let econAssignments = new Map<string, string>(); // bot pos -> energy pos

export function economistStrategy(state: VisibleState): Move[] {
  const cfg = state.config;
  const key = stateKey(state);
  if (key !== econStateKey) {
    econStateKey = key;
    econAssignments = new Map();
  }
  const myID = state.you.id;

  const myBots = state.bots.filter(b => b.owner === myID);
  const enemySet = new Set(state.bots.filter(b => b.owner !== myID).map(b => posKey(b.position)));
  if (myBots.length === 0) return [];

  const energySet = new Set(state.energy.map(posKey));

  // Drop assignments whose bot moved/died or whose energy was consumed.
  for (const [bk, ek] of econAssignments) {
    const stillMine = myBots.some(b => posKey(b.position) === bk);
    if (!stillMine || !energySet.has(ek)) econAssignments.delete(bk);
  }

  const moves: Move[] = [];
  const usedPositions = new Set<string>();

  // Priority 1: maintain existing contest assignments — stay put once adjacent.
  for (const bot of myBots) {
    const bk = posKey(bot.position);
    const contest = econAssignments.get(bk);
    if (!contest || !energySet.has(contest)) continue;
    usedPositions.add(bk);
    const target = parseKey(contest);
    if (dist2(bot.position, target, cfg) > ECON_ADJACENT_RADIUS2) {
      const dir = towardDir(bot.position, target, cfg);
      if (dir !== null) moves.push({ position: bot.position, direction: dir });
    }
  }

  // Priority 2: contest visible energy nodes enemies can also reach.
  const priorities = state.energy.map(e => {
    let nearestMy = Infinity;
    for (const bot of myBots) {
      if (usedPositions.has(posKey(bot.position))) continue;
      const d = dist2(bot.position, e, cfg);
      if (d < nearestMy) nearestMy = d;
    }

    let enemyReachable = 0;
    let nearestEnemy = Infinity;
    if (enemySet.size > 0) {
      for (const ek of enemySet) {
        const ep = parseKey(ek);
        const d = dist2(ep, e, cfg);
        if (d < nearestEnemy) nearestEnemy = d;
        if (d <= 64) enemyReachable++;
      }
    } else {
      // No enemies visible — use distance to map center as a proxy.
      const center = { row: Math.floor(cfg.rows / 2), col: Math.floor(cfg.cols / 2) };
      nearestEnemy = dist2(e, center, cfg);
      if (nearestEnemy < 100) enemyReachable = 1;
    }

    let priority: number;
    if (enemyReachable > 0) priority = 10000.0 / (nearestEnemy + nearestMy + 1);
    else if (nearestEnemy < 100) priority = 1000.0 / (nearestMy + 1);
    else priority = 100.0 / (nearestMy + 1);
    return { pos: e, priority, myDist: nearestMy };
  });
  priorities.sort((a, b) => b.priority - a.priority);

  for (const ep of priorities) {
    // Nearest unassigned bot claims this node.
    let nearestBot: Position | null = null;
    let nearestDist = Infinity;
    for (const bot of myBots) {
      if (usedPositions.has(posKey(bot.position))) continue;
      const d = dist2(bot.position, ep.pos, cfg);
      if (d < nearestDist) { nearestDist = d; nearestBot = bot.position; }
    }
    if (!nearestBot) continue;
    usedPositions.add(posKey(nearestBot));
    econAssignments.set(posKey(nearestBot), posKey(ep.pos));
    if (nearestDist > ECON_ADJACENT_RADIUS2) {
      const dir = towardDir(nearestBot, ep.pos, cfg);
      if (dir !== null) moves.push({ position: nearestBot, direction: dir });
    }
  }

  // Priority 3: remaining bots drift toward the map center to find energy.
  const center = { row: Math.floor(cfg.rows / 2), col: Math.floor(cfg.cols / 2) };
  for (const bot of myBots) {
    if (!usedPositions.has(posKey(bot.position))) {
      const dir = towardDir(bot.position, center, cfg);
      if (dir !== null) moves.push({ position: bot.position, direction: dir });
    }
  }
  return moves;
}

// towardDir steps greedily toward a target (the economist's _move_toward):
// best squared-distance neighbor.
function towardDir(from: Position, to: Position, cfg: Config): Direction | null {
  let bestDir: Direction | null = null;
  let bestDist = Infinity;
  for (const st of cardinalSteps(from, cfg)) {
    const d = dist2(st.pos, to, cfg);
    if (d < bestDist) { bestDist = d; bestDir = st.dir; }
  }
  return bestDir;
}

// Assassin — decapitation archetype: every unit rushes the enemy core,
// ignoring enemies and economy (bots/assassin, Rust).
let assassinStateKey = '';
let assassinKnownTargets = new Map<string, boolean>(); // core pos -> last-known active

export function assassinStrategy(state: VisibleState): Move[] {
  const cfg = state.config;
  const myID = state.you.id;
  const key = stateKey(state);
  if (key !== assassinStateKey) {
    assassinStateKey = key;
    assassinKnownTargets = new Map();
  }

  for (const core of state.cores) {
    if (core.owner !== myID) {
      assassinKnownTargets.set(posKey(core.position), core.active);
    }
  }

  const myBots = state.bots.filter(b => b.owner === myID);
  if (myBots.length === 0) return [];
  const wallSet = new Set(state.walls.map(posKey));

  // Active targets sorted by distance from our center of mass.
  let center: Position = { row: 0, col: 0 };
  for (const bot of myBots) {
    center.row += bot.position.row;
    center.col += bot.position.col;
  }
  center.row = Math.trunc(center.row / myBots.length);
  center.col = Math.trunc(center.col / myBots.length);

  const targets: Position[] = [];
  for (const [k, active] of assassinKnownTargets) {
    if (active) targets.push(parseKey(k));
  }
  targets.sort((a, b) => dist2(center, a, cfg) - dist2(center, b, cfg));

  if (targets.length === 0) {
    // Explore outward to find enemy cores.
    const moves: Move[] = [];
    for (let i = 0; i < myBots.length; i++) {
      const bot = myBots[i];
      let targetRow = cfg.rows - 1;
      if (i % 3 === 0) targetRow = Math.floor(cfg.rows / 2);
      let targetCol = 0;
      if (i % 2 === 0) targetCol = cfg.cols - 1;
      const target = { row: targetRow, col: targetCol };
      const dir = closestStepDir(bot.position, target, wallSet, cfg);
      if (dir !== null) moves.push({ position: bot.position, direction: dir });
    }
    return moves;
  }

  const primary = targets[0];
  const claimed = new Set<string>();
  const moves: Move[] = [];
  for (const bot of myBots) {
    // Unlike rusher, walk straight through enemies — only walls block.
    const dir = assassinBFSDir(bot.position, primary, wallSet, claimed, cfg);
    if (dir !== null) {
      const dest = applyDir(bot.position, dir, cfg);
      claimed.add(posKey(dest));
      moves.push({ position: bot.position, direction: dir });
    }
  }
  return moves;
}

function assassinBFSDir(
  start: Position, goal: Position, wallSet: Set<string>, claimed: Set<string>, cfg: Config,
): Direction | null {
  if (posKey(start) === posKey(goal)) return null;
  const dir = bfsDirection(start, goal, p => !wallSet.has(posKey(p)), cfg);
  if (dir !== null) return dir;
  // No path — pick the direction that gets closest (skip claimed tiles).
  return closestStepDir(start, goal, wallSet, cfg, claimed);
}

function closestStepDir(
  start: Position, target: Position, wallSet: Set<string>, cfg: Config, claimed?: Set<string>,
): Direction | null {
  let bestDir: Direction | null = null;
  let bestDist = Infinity;
  for (const st of cardinalSteps(start, cfg)) {
    if (wallSet.has(posKey(st.pos))) continue;
    if (claimed?.has(posKey(st.pos))) continue;
    const d = torManhattan(st.pos, target, cfg);
    if (d < bestDist) { bestDist = d; bestDir = st.dir; }
  }
  return bestDir;
}

// Phalanx — tight formation combat: circular-mean centroid, hex formation
// slots, rally when cohesion breaks, advance on enemy concentration otherwise
// (bots/phalanx, Rust).
const PHX_FORMATION_RADIUS2 = 9.0;
const PHX_ADVANCE_WEIGHT = 10.0;
const PHX_FORMATION_WEIGHT = 8.0;
const PHX_ATTACK_RANGE_BONUS = 50.0;

let phxStateKey = '';
let phxCentroid: Position | null = null;

export function phalanxStrategy(state: VisibleState): Move[] {
  const cfg = state.config;
  const myID = state.you.id;
  const key = stateKey(state);
  if (key !== phxStateKey) {
    phxStateKey = key;
    phxCentroid = null;
  }

  const myBots = state.bots.filter(b => b.owner === myID);
  const enemyBots = state.bots.filter(b => b.owner !== myID);
  if (myBots.length === 0) return [];

  const myPositions = myBots.map(b => b.position);
  const wallSet = new Set(state.walls.map(posKey));
  const enemySet = new Set(enemyBots.map(b => posKey(b.position)));

  // Circular-mean centroid, smoothed with last turn's value (70% new).
  let centroid = circularMean(myPositions, cfg);
  if (phxCentroid !== null) {
    centroid = smoothCentroid(phxCentroid, centroid, cfg);
  }
  phxCentroid = centroid;

  let total = 0;
  for (const p of myPositions) total += dist2(p, centroid, cfg);
  const meanDist = total / myPositions.length;
  const rallying = meanDist > PHX_FORMATION_RADIUS2;

  let advanceTarget = centroid;
  if (!rallying) {
    if (enemyBots.length > 0) {
      advanceTarget = circularMean(enemyBots.map(b => b.position), cfg);
    } else {
      advanceTarget = { row: Math.floor(cfg.rows / 2), col: Math.floor(cfg.cols / 2) };
    }
  }

  const slots = phxFormationSlots(centroid, myPositions.length, cfg);
  const assignments = phxAssignSlots(myPositions, slots, cfg);

  const claimed = new Set<string>();
  const moves: Move[] = [];
  for (const bot of myBots) {
    const slot = assignments.get(posKey(bot.position));
    const scored = phxScoredDir(
      bot.position, slot !== undefined, slot ?? bot.position,
      advanceTarget, centroid, enemySet, wallSet, claimed, rallying, cfg,
    );
    if (scored !== null) {
      const dest = applyDir(bot.position, scored, cfg);
      claimed.add(posKey(dest));
      moves.push({ position: bot.position, direction: scored });
    } else {
      claimed.add(posKey(bot.position));
    }
  }
  return moves;
}

function phxScoredDir(
  pos: Position, hasSlot: boolean, slot: Position, advanceTarget: Position, centroid: Position,
  enemySet: Set<string>, wallSet: Set<string>, claimed: Set<string>, rallying: boolean, cfg: Config,
): Direction | null {
  let bestDir: Direction | null = null;
  let bestScore = -Infinity;
  for (const st of cardinalSteps(pos, cfg)) {
    if (wallSet.has(posKey(st.pos)) || enemySet.has(posKey(st.pos)) || claimed.has(posKey(st.pos))) continue;
    let score = 0.0;

    if (hasSlot) {
      score += (dist2(pos, slot, cfg) - dist2(st.pos, slot, cfg)) * PHX_FORMATION_WEIGHT;
    }
    score += (dist2(pos, centroid, cfg) - dist2(st.pos, centroid, cfg)) * (PHX_FORMATION_WEIGHT * 0.3);

    let advance = PHX_ADVANCE_WEIGHT;
    if (rallying) advance *= 2.0;
    score += (dist2(pos, advanceTarget, cfg) - dist2(st.pos, advanceTarget, cfg)) * advance;

    if (!rallying) {
      for (const ek of enemySet) {
        const ep = parseKey(ek);
        if (dist2(st.pos, ep, cfg) <= cfg.attack_radius2) {
          score += PHX_ATTACK_RANGE_BONUS;
        }
      }
    }

    if (bestDir === null || score > bestScore) {
      bestScore = score;
      bestDir = st.dir;
    }
  }
  return bestDir;
}

function circularMean(positions: Position[], cfg: Config): Position {
  if (positions.length === 0) {
    return { row: Math.floor(cfg.rows / 2), col: Math.floor(cfg.cols / 2) };
  }
  const rowScale = 2.0 * Math.PI / cfg.rows;
  const colScale = 2.0 * Math.PI / cfg.cols;
  const n = positions.length;

  let sinR = 0, cosR = 0, sinC = 0, cosC = 0;
  for (const p of positions) {
    sinR += Math.sin(p.row * rowScale);
    cosR += Math.cos(p.row * rowScale);
    sinC += Math.sin(p.col * colScale);
    cosC += Math.cos(p.col * colScale);
  }

  const avgRow = Math.atan2(sinR / n, cosR / n) / rowScale;
  const avgCol = Math.atan2(sinC / n, cosC / n) / colScale;
  const wrappedRow = ((avgRow % cfg.rows) + cfg.rows) % cfg.rows;
  const wrappedCol = ((avgCol % cfg.cols) + cfg.cols) % cfg.cols;
  return { row: Math.round(wrappedRow), col: Math.round(wrappedCol) };
}

function smoothCentroid(prev: Position, current: Position, cfg: Config): Position {
  const dr = toroidalDelta(prev.row, current.row, cfg.rows);
  const dc = toroidalDelta(prev.col, current.col, cfg.cols);
  return {
    row: wrapInt(Math.round(prev.row + 0.7 * dr), cfg.rows),
    col: wrapInt(Math.round(prev.col + 0.7 * dc), cfg.cols),
  };
}

function phxFormationSlots(centroid: Position, count: number, cfg: Config): Position[] {
  if (count === 0) return [];
  const slots: Position[] = [centroid];
  for (let ring = 1; slots.length < count && ring <= 20; ring++) {
    for (const [dr, dc] of hexRing(ring)) {
      if (slots.length >= count) break;
      slots.push({ row: wrapInt(centroid.row + dr, cfg.rows), col: wrapInt(centroid.col + dc, cfg.cols) });
    }
  }
  return slots;
}

// hexRing generates the 6*ring offsets of a hex ring in offset coordinates
// (axial hex → offset_col = q + r/2).
function hexRing(ring: number): [number, number][] {
  if (ring === 0) return [[0, 0]];
  const hexDirs: [number, number][] = [[1, 0], [0, 1], [-1, 1], [-1, 0], [0, -1], [1, -1]];
  const result: [number, number][] = [];
  let q = ring, r = 0;
  for (const [dq, dr] of hexDirs) {
    for (let i = 0; i < ring; i++) {
      result.push([r, q + Math.trunc(r / 2)]);
      q += dq;
      r += dr;
    }
  }
  return result;
}

function phxAssignSlots(bots: Position[], slots: Position[], cfg: Config): Map<string, Position> {
  const assignments = new Map<string, Position>();
  const used = new Array(slots.length).fill(false);
  for (const bot of bots) {
    let bestSlot = 0;
    let bestDist = Infinity;
    for (let si = 0; si < slots.length; si++) {
      if (used[si]) continue;
      const d = dist2(bot, slots[si], cfg);
      if (d < bestDist) { bestDist = d; bestSlot = si; }
    }
    if (slots.length > 0) {
      used[bestSlot] = true;
      assignments.set(posKey(bot), slots[bestSlot]);
    }
  }
  return assignments;
}

// Zone-driver — weaponizes the shrinking zone: saves own bots near the edge,
// blocks enemy escape routes from the kill band, sweeps to herd enemies
// (bots/zone-driver, Rust). The TS engine has no zone, so matches here play
// the defensive fallback; the zone logic runs when called back by the Go
// WASM engine, which does emit zone state.
export function zoneDriverStrategy(state: VisibleState): Move[] {
  const myID = state.you.id;
  const cfg = state.config;
  const myBots = state.bots.filter(b => b.owner === myID);
  const enemyBots = state.bots.filter(b => b.owner !== myID);
  if (myBots.length === 0) return [];

  const enemySet = new Set(enemyBots.map(b => posKey(b.position)));
  const wallSet = new Set(state.walls.map(posKey));

  if (!state.zone?.active) {
    // No active zone — play conservatively.
    return zdDefensiveFallback(myBots, enemySet, wallSet, cfg);
  }
  const zone = state.zone;

  const moves: Move[] = [];
  const assigned = new Set<string>();

  // PRIORITY 1: save own bots outside or on the zone edge.
  for (const bot of myBots) {
    if (zdDistanceToZoneEdge(bot.position, zone, cfg) <= 0) {
      const dir = zdRetreatDir(bot.position, zone, wallSet, cfg);
      if (dir !== null) {
        moves.push({ position: bot.position, direction: dir });
        assigned.add(posKey(bot.position));
      }
    }
  }

  // PRIORITY 2: block escape routes of enemies in the kill band (the ring
  // just inside the zone edge where enemies die next shrink).
  const killBandInner = Math.max(0, zone.radius - 2);
  const killBandOuter = zone.radius;
  for (const bot of myBots) {
    if (assigned.has(posKey(bot.position))) continue;
    const target = zdEnemyInKillBand(bot.position, enemyBots, zone, killBandInner, killBandOuter, cfg);
    if (target) {
      const dir = zdBlockEscapeDir(bot.position, target, zone, wallSet, cfg);
      if (dir !== null) {
        moves.push({ position: bot.position, direction: dir });
        assigned.add(posKey(bot.position));
      }
    }
  }

  // PRIORITY 3: sweep to apply pressure.
  for (const bot of myBots) {
    if (assigned.has(posKey(bot.position))) continue;
    const dir = zdAdvancePressureDir(bot.position, enemySet, zone, wallSet, cfg);
    if (dir !== null) {
      moves.push({ position: bot.position, direction: dir });
      assigned.add(posKey(bot.position));
    }
  }

  // Remaining bots hold position (the ladder bot emits a nominal N here).
  for (const bot of myBots) {
    if (!assigned.has(posKey(bot.position))) {
      moves.push({ position: bot.position, direction: 'N' });
    }
  }
  return moves;
}

function zdDistanceToZoneEdge(pos: Position, zone: { center: Position; radius: number }, cfg: Config): number {
  const dist = Math.sqrt(dist2(pos, zone.center, cfg));
  return Math.trunc(zone.radius - dist);
}

function zdRetreatDir(
  pos: Position, zone: { center: Position; radius: number }, wallSet: Set<string>, cfg: Config,
): Direction | null {
  let bestDir: Direction | null = null;
  let bestReduction = -Infinity;
  const current = dist2(pos, zone.center, cfg);
  for (const st of cardinalSteps(pos, cfg)) {
    if (wallSet.has(posKey(st.pos))) continue;
    const next = dist2(st.pos, zone.center, cfg);
    if (next < current) {
      const reduction = current - next;
      if (reduction > bestReduction) { bestReduction = reduction; bestDir = st.dir; }
    }
  }
  return bestDir;
}

function zdEnemyInKillBand(
  myPos: Position, enemies: VisibleBot[], zone: { center: Position; radius: number },
  inner: number, outer: number, cfg: Config,
): Position | null {
  let best: Position | null = null;
  let bestDist = Infinity;
  for (const bot of enemies) {
    const dist = Math.sqrt(dist2(bot.position, zone.center, cfg));
    if (dist < inner || dist > outer) continue;
    const d = dist2(myPos, bot.position, cfg);
    if (d < bestDist) { bestDist = d; best = bot.position; }
  }
  return best;
}

// zdBlockEscapeDir moves toward the tile one step inward of the enemy
// (between it and the zone center).
function zdBlockEscapeDir(
  myPos: Position, enemyPos: Position, zone: { center: Position; radius: number },
  wallSet: Set<string>, cfg: Config,
): Direction | null {
  const dr = zone.center.row - enemyPos.row;
  const dc = zone.center.col - enemyPos.col;
  const length = Math.sqrt(dr * dr + dc * dc);
  if (length < 0.1) return null;
  const idealRow = enemyPos.row + dr / length;
  const idealCol = enemyPos.col + dc / length;

  let bestDir: Direction | null = null;
  let bestDist = Infinity;
  for (const st of cardinalSteps(myPos, cfg)) {
    if (wallSet.has(posKey(st.pos))) continue;
    const fdr = idealRow - st.pos.row;
    const fdc = idealCol - st.pos.col;
    const d = Math.sqrt(fdr * fdr + fdc * fdc);
    if (d < bestDist) { bestDist = d; bestDir = st.dir; }
  }
  return bestDir;
}

function zdAdvancePressureDir(
  pos: Position, enemySet: Set<string>, zone: { center: Position; radius: number },
  wallSet: Set<string>, cfg: Config,
): Direction | null {
  // Enemies visible — advance toward the nearest.
  if (enemySet.size > 0) {
    let nearest: Position | null = null;
    let nearestDist = Infinity;
    for (const ek of enemySet) {
      const e = parseKey(ek);
      const d = dist2(pos, e, cfg);
      if (d < nearestDist) { nearestDist = d; nearest = e; }
    }
    let bestDir: Direction | null = null;
    let bestDist = Infinity;
    for (const st of cardinalSteps(pos, cfg)) {
      if (wallSet.has(posKey(st.pos))) continue;
      const d = dist2(st.pos, nearest!, cfg);
      if (d < bestDist) { bestDist = d; bestDir = st.dir; }
    }
    if (bestDir !== null) return bestDir;
  }

  // No enemies — move out to the pressure ring (radius − 3).
  const targetRadius = Math.max(0, zone.radius - 3);
  const targetDist2 = targetRadius * targetRadius;
  const current = dist2(pos, zone.center, cfg);
  if (current < targetDist2) {
    let bestDir: Direction | null = null;
    let bestIncrease = -Infinity;
    for (const st of cardinalSteps(pos, cfg)) {
      if (wallSet.has(posKey(st.pos))) continue;
      const increase = dist2(st.pos, zone.center, cfg) - current;
      if (increase > bestIncrease) { bestIncrease = increase; bestDir = st.dir; }
    }
    return bestDir;
  }
  return null;
}

function zdDefensiveFallback(
  myBots: VisibleBot[], enemySet: Set<string>, wallSet: Set<string>, cfg: Config,
): Move[] {
  const moves: Move[] = [];
  for (const bot of myBots) {
    if (enemySet.size === 0) continue;
    let nearest: Position | null = null;
    let nearestDist = Infinity;
    for (const ek of enemySet) {
      const e = parseKey(ek);
      const d = dist2(bot.position, e, cfg);
      if (d < nearestDist) { nearestDist = d; nearest = e; }
    }
    let bestDir: Direction | null = null;
    let bestDist = Infinity;
    for (const st of cardinalSteps(bot.position, cfg)) {
      if (wallSet.has(posKey(st.pos)) || enemySet.has(posKey(st.pos))) continue;
      const d = dist2(st.pos, nearest!, cfg);
      if (d < bestDist) { bestDist = d; bestDir = st.dir; }
    }
    if (bestDir !== null) {
      moves.push({ position: bot.position, direction: bestDir });
    }
  }
  return moves;
}

export const BUILTIN_STRATEGIES: Record<string, BotStrategy> = {
  random: randomStrategy,
  gatherer: gathererStrategy,
  rusher: rusherStrategy,
  guardian: guardianStrategy,
  swarm: swarmStrategy,
  hunter: hunterStrategy,
  farmer: farmerStrategy,
  opportunist: opportunistStrategy,
  siege: siegeStrategy,
  economist: economistStrategy,
  assassin: assassinStrategy,
  phalanx: phalanxStrategy,
  'zone-driver': zoneDriverStrategy,
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

function parseKey(k: string): Position {
  const [r, c] = k.split(',').map(Number);
  return { row: r, col: c };
}

interface Step { pos: Position; dir: Direction; }

// cardinalSteps returns the four wrapped neighbors of p in N, E, S, W order.
function cardinalSteps(p: Position, cfg: Config): Step[] {
  return DIRS.map(d => ({ pos: applyDir(p, d, cfg), dir: d }));
}

// toroidalDelta returns the signed shortest delta from a to b on a ring of
// size n (|delta| ≤ n/2).
function toroidalDelta(a: number, b: number, n: number): number {
  let d = b - a;
  if (d > n / 2) d -= n;
  else if (d < -n / 2) d += n;
  return d;
}

function wrapInt(v: number, n: number): number {
  return ((v % n) + n) % n;
}

// torManhattan returns toroidal Manhattan distance (farmer/opportunist/assassin
// pathing heuristic).
function torManhattan(a: Position, b: Position, cfg: Config): number {
  return Math.abs(toroidalDelta(a.row, b.row, cfg.rows)) + Math.abs(toroidalDelta(a.col, b.col, cfg.cols));
}

// bfsDirection returns the first direction of a shortest 4-directional path
// from start to goal on the toroidal grid, or null when unreachable. The goal
// itself must satisfy passable, mirroring the ladder bots' BFS.
function bfsDirection(start: Position, goal: Position, passable: (p: Position) => boolean, cfg: Config): Direction | null {
  if (posKey(start) === posKey(goal)) return null;
  const visited = new Set([posKey(start)]);
  const queue: Step[] = [];
  for (const st of cardinalSteps(start, cfg)) {
    if (posKey(st.pos) === posKey(goal) && passable(st.pos)) return st.dir;
    if (passable(st.pos) && !visited.has(posKey(st.pos))) {
      visited.add(posKey(st.pos));
      queue.push(st);
    }
  }
  while (queue.length > 0) {
    const cur = queue.shift()!;
    if (posKey(cur.pos) === posKey(goal)) return cur.dir;
    for (const st of cardinalSteps(cur.pos, cfg)) {
      if (!visited.has(posKey(st.pos)) && passable(st.pos)) {
        visited.add(posKey(st.pos));
        queue.push({ pos: st.pos, dir: cur.dir });
      }
    }
  }
  return null;
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
  format_version?: string;
  match_id: string;
  config: Config;
  start_time: string;
  end_time: string;
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
  return runMultiMatch(cfg, [s1, s2], seed);
}

export function runMultiMatch(
  cfg: Config,
  strategies: (BotStrategy | string)[],
  seed?: number,
): { replay: Replay; result: MatchResult } {
  const resolved = strategies.map(s =>
    typeof s === 'string' ? BUILTIN_STRATEGIES[s] ?? randomStrategy : s
  );
  const numPlayers = resolved.length;
  const gs = newGame(cfg, seed, numPlayers);

  const wallPositions: Position[] = [];
  for (const k of gs.walls) {
    const [r, c] = k.split(',').map(Number);
    wallPositions.push({ row: r, col: c });
  }

  const startTime = new Date().toISOString();
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
      const strategy = resolved[p.id];
      try {
        allMoves.set(p.id, strategy(visible));
      } catch {
        allMoves.set(p.id, []);
      }
    }
    result = executeTurn(gs, allMoves);
    turns.push(recordTurn());
  }

  const endTime = new Date().toISOString();
  const names = strategies.map((s, i) =>
    typeof s === 'string' ? s : (i === 0 ? 'Your Bot' : `Opponent ${i}`)
  );

  const replay: Replay = {
    format_version: '1.0',
    match_id: gs.matchId,
    config: cfg,
    start_time: startTime,
    end_time: endTime,
    result,
    players: names.map((name, i) => ({ id: i, name })),
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
