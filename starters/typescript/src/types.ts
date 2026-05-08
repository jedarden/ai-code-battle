/**
 * AI Code Battle - TypeScript Type Definitions
 *
 * Complete type definitions for the game protocol.
 * All types match the JSON schema documented in the protocol spec.
 */

/**
 * Cardinal movement directions.
 */
export type Direction = "N" | "E" | "S" | "W";

/** All four cardinal directions. */
export const ALL_DIRECTIONS: Direction[] = ["N", "E", "S", "W"];

/**
 * Grid position (row, column).
 */
export interface Position {
  row: number;
  col: number;
}

/**
 * Match configuration parameters.
 * These are identical for all players and do not change between turns.
 */
export interface GameConfig {
  rows: number;
  cols: number;
  max_turns: number;
  vision_radius2: number;
  attack_radius2: number;
  spawn_cost: number;
  energy_interval: number;
  season_id?: string;
  rules_version?: number;
  special_tiles?: string[];
}

/**
 * Information about the current player (you).
 */
export interface PlayerInfo {
  id: number;
  energy: number;
  score: number;
}

/**
 * A bot visible within fog of war.
 */
export interface VisibleBot {
  row: number;
  col: number;
  owner: number;
}

/**
 * A core (spawn point) visible within fog of war.
 */
export interface VisibleCore {
  row: number;
  col: number;
  owner: number;
  active: boolean;
}

/**
 * Energy tile position.
 */
export interface EnergyTile {
  row: number;
  col: number;
}

/**
 * Game state sent by the engine each turn.
 * Contains only tiles visible within the player's fog of war.
 */
export interface GameState {
  match_id: string;
  turn: number;
  config: GameConfig;
  you: PlayerInfo;
  bots: VisibleBot[];
  energy: EnergyTile[];
  cores: VisibleCore[];
  walls: Position[];
  dead: VisibleBot[];
}

/**
 * A movement order for a single bot.
 * References the bot's current position and the direction to move.
 */
export interface Move {
  row: number;
  col: number;
  direction: Direction;
}

/**
 * Optional debug telemetry for replay visualization.
 * Stored in the replay but never parsed by the engine.
 * Max 10KB per turn.
 */
export interface DebugTelemetry {
  reasoning?: string;
  targets?: DebugTarget[];
  values?: Record<string, string | number>;
  heatmap?: DebugHeatmap;
}

export interface DebugTarget {
  row: number;
  col: number;
  label: string;
  priority: number;
}

export interface DebugHeatmap {
  name: string;
  data: number[][];
}

/**
 * Response sent back to the engine.
 */
export interface MoveResponse {
  moves: Move[];
  debug?: DebugTelemetry;
}

/**
 * Authentication headers from incoming requests.
 */
export interface AuthHeaders {
  "x-acb-match-id": string;
  "x-acb-turn": string;
  "x-acb-timestamp": string;
  "x-acb-bot-id": string;
  "x-acb-signature": string;
}
