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
  season_id?: string;
  rules_version?: string;
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

export interface ReplayCriticalMoment {
  turn: number;
  delta: number;       // change in p0 win probability (positive = p0 improved)
  description: string;
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
  win_prob?: number[][];             // [[p0, p1], ...] one entry per turn
  critical_moments?: ReplayCriticalMoment[];
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
  priority?: number;  // 0.0–1.0; controls marker opacity (1 = fully opaque)
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

// Enriched commentary types (§13.3)
export interface CommentaryEntry {
  turn: number;
  text: string;
  type: 'setup' | 'action' | 'reaction' | 'climax' | 'denouement';
}

export interface EnrichedCommentary {
  match_id: string;
  generated_at: string;
  criteria: string[];
  entries: CommentaryEntry[];
}

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
  bracket_round?: string;
  bracket_position?: number;
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

export interface ChampionshipBracketSeries {
  id: string;
  bot1_id: string;
  bot2_id: string;
  bot1_name: string;
  bot2_name: string;
  best_of: number;
  bot1_wins: number;
  bot2_wins: number;
  status: string;
  winner_id: string | null;
  round: string;
  bracket_position: number;
  games: SeriesGame[];
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
  championship_bracket?: ChampionshipBracketSeries[];
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

// Community replay feedback (plan §13.6, §8.3)

export type FeedbackType = 'insight' | 'mistake' | 'idea' | 'highlight';

export interface FeedbackEntry {
  feedback_id: string;
  match_id: string;
  turn: number;
  type: FeedbackType;
  body: string;
  author: string;
  upvotes: number;
  created_at: string;
}

export interface FeedbackResponse {
  match_id: string;
  feedback: FeedbackEntry[];
}

// Evolution live.json schema (plan §14) — real-time dashboard feed from acb-evolver

export interface EvolutionIslandStat {
  population: number;
  best_rating: number;
  best_bot: string;
  language_div?: string;
}

export interface EvolutionParentInfo {
  id: string;
  rating: number;
}

export interface EvolutionStageResult {
  passed: boolean;
  time_ms: number;
  error?: string;
}

export interface EvolutionValidationStatus {
  syntax?: EvolutionStageResult;
  schema?: EvolutionStageResult;
  smoke?: EvolutionStageResult;
}

export interface EvaluationMatchResult {
  opponent: string;
  won: boolean;
  score: string;
}

export interface EvolutionEvaluationStatus {
  matches_total: number;
  matches_played: number;
  results: EvaluationMatchResult[];
}

export interface EvolutionCandidate {
  id: string;
  island: string;
  language: string;
  parents: EvolutionParentInfo[];
  validation?: EvolutionValidationStatus;
  evaluation?: EvolutionEvaluationStatus;
}

export interface EvolutionCycleInfo {
  generation: number;
  started_at: string;
  phase: string; // generating, validating, evaluating, promoting, idle
  candidate?: EvolutionCandidate;
}

export interface EvolutionActivityEntry {
  time: string;
  generation: number;
  candidate: string;
  island: string;
  result: string; // promoted, rejected
  reason: string;
  stage: string; // validation, promotion, deployment
  bot_id?: string;
  initial_rating?: number;
}

export interface EvolutionTotals {
  generations_total: number;
  candidates_today: number;
  promoted_today: number;
  promotion_rate_7d: number;
  highest_evolved_rating: number;
  evolved_in_top_10: number;
  mutations_per_hour: number;
}

export interface EvolutionGenerationEntry {
  generation: number;
  island: string;
  evaluated_at: string;
  count: number;
  promoted: number;
  best_fitness: number;
  avg_fitness: number;
}

export interface EvolutionLineageNode {
  id: number;
  parent_ids: number[];
  generation: number;
  island: string;
  fitness: number;
  promoted: boolean;
  language: string;
  created_at: string;
}

export interface EvolutionMetaSnapshot {
  generation: number;
  island_counts: Record<string, number>;
  island_best_fitness: Record<string, number>;
}

export interface LiveJSON {
  updated_at: string;
  cycle?: EvolutionCycleInfo;
  recent_activity?: EvolutionActivityEntry[];
  islands: Record<string, EvolutionIslandStat>;
  totals: EvolutionTotals;
  // Legacy fields for backward compatibility
  total_programs?: number;
  promoted_count?: number;
  generation_log?: EvolutionGenerationEntry[];
  lineage?: EvolutionLineageNode[];
  meta_snapshots?: EvolutionMetaSnapshot[];
}
