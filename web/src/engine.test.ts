/**
 * Tests for the extended sandbox opponent roster (plan §13.1): farmer,
 * opportunist, siege, economist, assassin, phalanx, zone-driver.
 *
 * These mirror the Go-side tests in cmd/acb-wasm/strategies/strategies_test.go:
 * each strategy must accept a VisibleState and return only valid moves for
 * owned bots, and each must run a full match through the TS engine without
 * throwing or stalling.
 */

import { describe, it, expect } from 'vitest';
import {
  BUILTIN_STRATEGIES,
  defaultConfig,
  runMultiMatch,
  type VisibleState,
} from './engine';

const EXTENDED_ROSTER = [
  'farmer',
  'opportunist',
  'siege',
  'economist',
  'assassin',
  'phalanx',
  'zone-driver',
] as const;

/** Small synthetic visible state: player 0 has two bots and a core top-left,
 *  player 1 mirrors bottom-right, energy sits mid-map behind a wall column. */
function testState(): VisibleState {
  return {
    match_id: 'test-match',
    turn: 5,
    config: { ...defaultConfig(), rows: 20, cols: 20 },
    you: { id: 0, energy: 10, score: 0 },
    bots: [
      { position: { row: 2, col: 2 }, owner: 0 },
      { position: { row: 3, col: 2 }, owner: 0 },
      { position: { row: 17, col: 17 }, owner: 1 },
    ],
    energy: [{ row: 10, col: 10 }, { row: 10, col: 11 }],
    cores: [
      { position: { row: 1, col: 1 }, owner: 0, active: true },
      { position: { row: 18, col: 18 }, owner: 1, active: true },
    ],
    walls: Array.from({ length: 20 }, (_, r) => ({ row: r, col: 5 })),
    dead: [],
  };
}

describe('extended roster strategies (plan §13.1)', () => {
  it.each(EXTENDED_ROSTER)('%s is registered in BUILTIN_STRATEGIES', name => {
    expect(typeof BUILTIN_STRATEGIES[name]).toBe('function');
  });

  it.each(EXTENDED_ROSTER)('%s returns valid moves for owned bots only', name => {
    const moves = BUILTIN_STRATEGIES[name](testState());
    expect(Array.isArray(moves)).toBe(true);
    const own = new Set(['2,2', '3,2']);
    for (const mv of moves) {
      expect(mv.position.row).toBeGreaterThanOrEqual(0);
      expect(mv.position.row).toBeLessThan(20);
      expect(mv.position.col).toBeGreaterThanOrEqual(0);
      expect(mv.position.col).toBeLessThan(20);
      expect(['N', 'E', 'S', 'W', '']).toContain(mv.direction);
      expect(own.has(`${mv.position.row},${mv.position.col}`)).toBe(true);
    }
  });

  it.each(EXTENDED_ROSTER)('%s completes a full match vs gatherer without throwing', name => {
    const cfg = { ...defaultConfig(), rows: 20, cols: 20, max_turns: 100 };
    const { replay, result } = runMultiMatch(cfg, [name, 'gatherer'], 42);
    expect(result).not.toBeNull();
    expect(result.turns).toBeLessThanOrEqual(100);
    // Both players must appear in the replay with sane final scores.
    expect(replay.players).toHaveLength(2);
    for (const score of result.scores) {
      expect(score).toBeGreaterThanOrEqual(0);
      expect(Number.isFinite(score)).toBe(true);
    }
    // The opponent must actually act: over 100 turns at least some turns in
    // the replay contain bot positions for player 0 (spawns or movement).
    const acted = replay.turns.filter(t => t.bots.some(b => b.owner === 0));
    expect(acted.length).toBeGreaterThan(10);
  });

  it('runs a 4-player match mixing original and extended opponents', () => {
    const cfg = { ...defaultConfig(), rows: 30, cols: 30, max_turns: 80 };
    const { result } = runMultiMatch(cfg, ['gatherer', 'phalanx', 'economist', 'assassin'], 7);
    expect(result.scores).toHaveLength(4);
    expect(result.turns).toBeLessThanOrEqual(80);
  });
});
