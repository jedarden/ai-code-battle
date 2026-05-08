/**
 * AI Code Battle - Strategy Implementation
 *
 * This is where you implement your bot's strategy.
 * The computeMoves function is called each turn with the current game state.
 */

import type { GameState, Move, Direction } from "./types.js";
import { toroidalManhattan, cardinalNeighbors } from "./grid.js";

/**
 * Compute moves for all your bots.
 *
 * This is a placeholder strategy that moves bots toward nearby energy.
 * Replace this with your own strategy!
 *
 * @param state - Current game state (fog-filtered for your player)
 * @returns Array of moves for your bots
 */
export function computeMoves(state: GameState): Move[] {
  const moves: Move[] = [];
  const { rows, cols } = state.config;
  const myBotId = state.you.id;

  // Find all my bots
  const myBots = state.bots.filter((b) => b.owner === myBotId);

  // For each bot, decide where to move
  for (const bot of myBots) {
    const move = decideBotMove(bot, state);
    if (move) {
      moves.push(move);
    }
  }

  return moves;
}

/**
 * Decide a single bot's move.
 *
 * @param bot - The bot to move
 * @param state - Current game state
 * @returns Move command, or undefined to hold position
 */
function decideBotMove(
  bot: { row: number; col: number; owner: number },
  state: GameState
): Move | undefined {
  const { rows, cols } = state.config;

  // If there's energy visible, move toward the nearest one
  if (state.energy.length > 0) {
    let bestDir: Direction | null = null;
    let bestDist = Infinity;

    for (const { pos, dir } of cardinalNeighbors(bot.row, bot.col, rows, cols)) {
      // Skip if there's a wall
      if (state.walls.some((w) => w.row === pos.row && w.col === pos.col)) {
        continue;
      }

      // Find distance to nearest energy from this neighbor position
      for (const e of state.energy) {
        const dist = toroidalManhattan(pos.row, pos.col, e.row, e.col, cols, rows);
        if (dist < bestDist) {
          bestDist = dist;
          bestDir = dir;
        }
      }
    }

    if (bestDir) {
      return { row: bot.row, col: bot.col, direction: bestDir };
    }
  }

  // No energy nearby - move randomly or hold
  // You can replace this with more sophisticated logic
  return undefined; // Hold position
}

/**
 * Example: Check if a position is safe (no visible enemies nearby).
 */
export function isSafe(
  row: number,
  col: number,
  state: GameState,
  radius2: number = 5
): boolean {
  const { rows, cols } = state.config;
  const myBotId = state.you.id;

  for (const enemy of state.bots) {
    if (enemy.owner === myBotId) continue;

    const dr = Math.min(Math.abs(row - enemy.row), rows - Math.abs(row - enemy.row));
    const dc = Math.min(Math.abs(col - enemy.col), cols - Math.abs(col - enemy.col));
    const dist2 = dr * dr + dc * dc;

    if (dist2 <= radius2) {
      return false; // Enemy nearby
    }
  }

  return true;
}

/**
 * Example: Find a position to gather energy safely.
 */
export function findSafeGatherTarget(
  bot: { row: number; col: number },
  state: GameState
): { row: number; col: number } | null {
  const { rows, cols } = state.config;

  for (const energy of state.energy) {
    if (isSafe(energy.row, energy.col, state)) {
      return energy;
    }
  }

  // No safe energy found - go to nearest anyway
  if (state.energy.length > 0) {
    // Simple approach: first energy in list
    return state.energy[0];
  }

  return null;
}
