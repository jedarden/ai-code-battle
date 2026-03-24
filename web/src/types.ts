// Replay format types matching the Go engine

export interface Position {
  row: number;
  col: number;
}

export interface Config {
  rows: number;
  cols: number;
  max_turns: number;
  vision_radius2: number;
  attack_radius2: number;
  spawn_cost: number;
  energy_interval: number;
}

export interface MatchResult {
  winner: number;
  reason: string;
  turns: number;
  scores: number[];
  energy: number[];
  bots_alive: number[];
}

export interface ReplayPlayer {
  id: number;
  name: string;
}

export interface ReplayCore {
  position: Position;
  owner: number;
}

export interface ReplayMap {
  rows: number;
  cols: number;
  walls: Position[];
  cores: ReplayCore[];
  energy_nodes: Position[];
}

export interface ReplayBot {
  id: number;
  owner: number;
  position: Position;
  alive: boolean;
}

export interface ReplayCoreState {
  position: Position;
  owner: number;
  active: boolean;
}

export interface GameEvent {
  type: string;
  turn: number;
  details: unknown;
}

export interface ReplayTurn {
  turn: number;
  bots: ReplayBot[];
  cores: ReplayCoreState[];
  energy: Position[];
  scores: number[];
  energy_held: number[];
  events?: GameEvent[];
}

export interface Replay {
  match_id: string;
  config: Config;
  start_time: string;
  end_time: string;
  result: MatchResult;
  players: ReplayPlayer[];
  map: ReplayMap;
  turns: ReplayTurn[];
}

// Event detail types
export interface BotSpawnedDetails {
  bot_id: number;
  owner: number;
  position: Position;
}

export interface BotDiedDetails {
  bot_id: number;
  owner: number;
  position: Position;
}

export interface EnergyCollectedDetails {
  bot_id: number;
  owner: number;
  position: Position;
}

export interface CoreCapturedDetails {
  position: Position;
  old_owner: number;
  new_owner: number;
}

export interface CombatDeathDetails {
  attacker_id: number;
  attacker_owner: number;
  defender_id: number;
  defender_owner: number;
  position: Position;
}

export interface CollisionDeathDetails {
  bot_ids: number[];
  position: Position;
}
