/**
 * AI Code Battle - Grid Utilities
 *
 * Toroidal grid distance calculations, neighbor enumeration,
 * and BFS pathfinding.
 */

import type { Position } from "./types.js";

/**
 * Toroidal Manhattan distance (shortest path with wrap-around).
 *
 * @returns Distance in grid cells
 */
export function toroidalManhattan(
  r1: number,
  c1: number,
  r2: number,
  c2: number,
  cols: number,
  rows: number
): number {
  const dr = Math.min(Math.abs(r1 - r2), rows - Math.abs(r1 - r2));
  const dc = Math.min(Math.abs(c1 - c2), cols - Math.abs(c1 - c2));
  return dr + dc;
}

/**
 * Toroidal Chebyshev distance (shortest path with wrap-around, 8-directional).
 *
 * @returns Distance in grid cells
 */
export function toroidalChebyshev(
  r1: number,
  c1: number,
  r2: number,
  c2: number,
  cols: number,
  rows: number
): number {
  const dr = Math.min(Math.abs(r1 - r2), rows - Math.abs(r1 - r2));
  const dc = Math.min(Math.abs(c1 - c2), cols - Math.abs(c1 - c2));
  return Math.max(dr, dc);
}

/**
 * Toroidal squared Euclidean distance.
 * Faster than sqrt for comparisons.
 */
export function toroidalDistanceSquared(
  r1: number,
  c1: number,
  r2: number,
  c2: number,
  cols: number,
  rows: number
): number {
  const dr = Math.min(Math.abs(r1 - r2), rows - Math.abs(r1 - r2));
  const dc = Math.min(Math.abs(c1 - c2), cols - Math.abs(c1 - c2));
  return dr * dr + dc * dc;
}

/**
 * Get all 8 neighboring positions with toroidal wrap.
 *
 * @returns Array of [row, col] pairs
 */
export function neighbors(
  row: number,
  col: number,
  rows: number,
  cols: number
): Position[] {
  const offsets = [
    [-1, -1],
    [-1, 0],
    [-1, 1],
    [0, -1],
    [0, 1],
    [1, -1],
    [1, 0],
    [1, 1],
  ];
  return offsets.map(([dr, dc]) => ({
    row: (row + dr + rows) % rows,
    col: (col + dc + cols) % cols,
  }));
}

/**
 * Get 4 cardinal neighboring positions (N, E, S, W).
 */
export function cardinalNeighbors(
  row: number,
  col: number,
  rows: number,
  cols: number
): Array<{ pos: Position; dir: "N" | "E" | "S" | "W" }> {
  return [
    { pos: { row: (row - 1 + rows) % rows, col }, dir: "N" },
    { pos: { row, col: (col + 1) % cols }, dir: "E" },
    { pos: { row: (row + 1) % rows, col }, dir: "S" },
    { pos: { row, col: (col - 1 + cols) % cols }, dir: "W" },
  ];
}

/**
 * BFS pathfinding on a toroidal grid.
 *
 * @param start - Starting position [row, col]
 * @param goal - Goal position [row, col]
 * @param passable - Function returning true if a tile is walkable
 * @returns Path from start to goal (excluding start), or null if unreachable
 */
export function bfs(
  start: Position,
  goal: Position,
  passable: (row: number, col: number) => boolean,
  rows: number,
  cols: number
): Position[] | null {
  if (start.row === goal.row && start.col === goal.col) {
    return [];
  }

  const key = (r: number, c: number) => `${r},${c}`;
  const visited = new Set<string>([key(start.row, start.col)]);

  interface QueueItem {
    pos: Position;
    path: Position[];
  }

  const queue: QueueItem[] = [{ pos: start, path: [] }];

  while (queue.length > 0) {
    const { pos, path } = queue.shift()!;

    for (const next of neighbors(pos.row, pos.col, rows, cols)) {
      const newPath = [...path, next];

      if (next.row === goal.row && next.col === goal.col) {
        return newPath;
      }

      const k = key(next.row, next.col);
      if (!visited.has(k) && passable(next.row, next.col)) {
        visited.add(k);
        queue.push({ pos: next, path: newPath });
      }
    }
  }

  return null;
}

/**
 * Find the nearest position from a list of targets.
 *
 * @returns The nearest target or null if targets is empty
 */
export function findNearest(
  from: Position,
  targets: Position[],
  rows: number,
  cols: number
): Position | null {
  if (targets.length === 0) return null;

  let nearest = targets[0];
  let bestDist = toroidalDistanceSquared(
    from.row,
    from.col,
    nearest.row,
    nearest.col,
    cols,
    rows
  );

  for (let i = 1; i < targets.length; i++) {
    const dist = toroidalDistanceSquared(
      from.row,
      from.col,
      targets[i].row,
      targets[i].col,
      cols,
      rows
    );
    if (dist < bestDist) {
      bestDist = dist;
      nearest = targets[i];
    }
  }

  return nearest;
}
