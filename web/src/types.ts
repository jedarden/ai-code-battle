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
  debug?: Record<number, DebugInfo>;
}

export interface Replay {
  format_version?: string; // semver, e.g. "1.0" — absent in pre-v1 replays
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

// Debug telemetry types
export interface DebugTarget {
  position: Position;
  label?: string;
  color?: string;
}

export interface DebugInfo {
  reasoning?: string;
  targets?: DebugTarget[];
}

// Extended ReplayTurn with debug support
export interface ReplayTurnWithDebug extends ReplayTurn {
  debug?: Record<number, DebugInfo>;
}

// View mode types for replay viewer
export type ViewMode = 'standard' | 'dots' | 'voronoi' | 'influence';

// Series types
export interface SeriesGame {
  match_id: string;
  game_number: number;
  winner_id: string | null;
  winner_slot: number | null;
  turns: number | null;
  completed_at: string | null;
}

export interface Series {
  id: string;
  bot1_id: string;
  bot2_id: string;
  bot1_name: string;
  bot2_name: string;
  best_of: number;
  status: 'pending' | 'active' | 'completed';
  bot1_wins: number;
  bot2_wins: number;
  winner_id: string | null;
  scheduled_at: string | null;
  completed_at: string | null;
  games: SeriesGame[];
}

export interface SeriesIndex {
  updated_at: string;
  series: Series[];
}

// Season types
export interface SeasonMatch {
  match_id: string;
  week: number;
  bot1_id: string;
  bot2_id: string;
  winner_id: string | null;
}

export interface SeasonSnapshot {
  bot_id: string;
  bot_name: string;
  rating: number;
  rank: number;
  wins: number;
  losses: number;
}

export interface Season {
  id: string;
  name: string;
  theme: string;
  rules_version: string;
  status: 'upcoming' | 'active' | 'completed';
  starts_at: string;
  ends_at: string | null;
  champion_id: string | null;
  champion_name: string | null;
  total_matches: number;
  final_snapshot: SeasonSnapshot[] | null;
}

export interface SeasonIndex {
  updated_at: string;
  active_season: Season | null;
  seasons: Season[];
}

// Prediction types
export interface Prediction {
  id: string;
  match_id: string;
  predictor_id: string;
  predicted_winner_slot: number;
  actual_winner_slot: number | null;
  correct: boolean | null;
  created_at: string;
  resolved_at: string | null;
}

export interface PredictorStats {
  predictor_id: string;
  predictor_name: string;
  total_predictions: number;
  correct_predictions: number;
  accuracy: number;
  streak: number;
  best_streak: number;
}

export interface PredictionLeaderboard {
  updated_at: string;
  leaders: PredictorStats[];
}
