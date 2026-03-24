/**
 * Game state types for AI Code Battle protocol.
 */

export interface Position {
  row: number;
  col: number;
}

export interface GameConfig {
  rows: number;
  cols: number;
  max_turns: number;
  vision_radius2: number;
  attack_radius2: number;
  spawn_cost: number;
  energy_interval: number;
}

export interface PlayerInfo {
  id: number;
  energy: number;
  score: number;
}

export interface VisibleBot {
  position: Position;
  owner: number;
}

export interface VisibleCore {
  position: Position;
  owner: number;
  active: boolean;
}

export interface GameState {
  match_id: string;
  turn: number;
  config: GameConfig;
  you: PlayerInfo;
  bots: VisibleBot[];
  energy: Position[];
  cores: VisibleCore[];
  walls: Position[];
  dead: VisibleBot[];
}

export type Direction = 'N' | 'E' | 'S' | 'W';

export interface Move {
  position: Position;
  direction: Direction;
}

export interface MoveResponse {
  moves: Move[];
}

// Utility functions

export function posKey(pos: Position): string {
  return `${pos.row},${pos.col}`;
}

export function posEquals(a: Position, b: Position): boolean {
  return a.row === b.row && a.col === b.col;
}

export function moveToward(pos: Position, dir: Direction, rows: number, cols: number): Position {
  switch (dir) {
    case 'N':
      return { row: (pos.row - 1 + rows) % rows, col: pos.col };
    case 'E':
      return { row: pos.row, col: (pos.col + 1) % cols };
    case 'S':
      return { row: (pos.row + 1) % rows, col: pos.col };
    case 'W':
      return { row: pos.row, col: (pos.col - 1 + cols) % cols };
  }
}

export function distance2(a: Position, b: Position, rows: number, cols: number): number {
  let dr = Math.abs(a.row - b.row);
  let dc = Math.abs(a.col - b.col);
  dr = Math.min(dr, rows - dr);
  dc = Math.min(dc, cols - dc);
  return dr * dr + dc * dc;
}

export function manhattanDistance(a: Position, b: Position, rows: number, cols: number): number {
  let dr = Math.abs(a.row - b.row);
  let dc = Math.abs(a.col - b.col);
  dr = Math.min(dr, rows - dr);
  dc = Math.min(dc, cols - dc);
  return dr + dc;
}

export const ALL_DIRECTIONS: Direction[] = ['N', 'E', 'S', 'W'];

export function buildPositionSet(positions: Position[]): Set<string> {
  return new Set(positions.map(posKey));
}
