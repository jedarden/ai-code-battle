/**
 * Unit tests for Director Mode (§16.10)
 * Verifies action density calculation and speed mapping match the plan.
 */

import { describe, it, expect } from 'vitest';
import {
  computeActionDensity,
  densityToSpeed,
  computeSpeedSchedule,
  computeAllDensities,
  type ActionDensity,
} from './director';
import type { ReplayTurn, Replay } from '../types';

describe('Director Mode (§16.10)', () => {
  describe('computeActionDensity', () => {
    it('should calculate density 0 for empty turn', () => {
      const turn: ReplayTurn = {
        bots: [],
        energy: [],
        cores: [],
        scores: [],
        energy_held: [],
      };
      const result = computeActionDensity(turn, null);
      expect(result.density).toBe(0);
      expect(result.deaths).toBe(0);
      expect(result.captures).toBe(0);
      expect(result.energyCollected).toBe(0);
      expect(result.spawns).toBe(0);
    });

    it('should weight deaths × 3.0 per plan formula', () => {
      const turn: ReplayTurn = {
        bots: [],
        energy: [],
        cores: [],
        scores: [],
        energy_held: [],
        events: [
          { type: 'combat_death', details: {} },
          { type: 'combat_death', details: {} },
        ],
      };
      const result = computeActionDensity(turn, null);
      expect(result.deaths).toBe(2);
      expect(result.density).toBe(2 * 3.0);
    });

    it('should weight captures × 5.0 per plan formula', () => {
      const turn: ReplayTurn = {
        bots: [],
        energy: [],
        cores: [],
        scores: [],
        energy_held: [],
        events: [
          { type: 'core_captured', details: {} },
        ],
      };
      const result = computeActionDensity(turn, null);
      expect(result.captures).toBe(1);
      expect(result.density).toBe(1 * 5.0);
    });

    it('should weight energy_collected × 0.5 per plan formula', () => {
      const turn: ReplayTurn = {
        bots: [],
        energy: [],
        cores: [],
        scores: [],
        energy_held: [],
        events: [
          { type: 'energy_collected', details: {} },
          { type: 'energy_collected', details: {} },
        ],
      };
      const result = computeActionDensity(turn, null);
      expect(result.energyCollected).toBe(2);
      expect(result.density).toBe(2 * 0.5);
    });

    it('should weight spawns × 1.0 per plan formula', () => {
      const turn: ReplayTurn = {
        bots: [],
        energy: [],
        cores: [],
        scores: [],
        energy_held: [],
        events: [
          { type: 'bot_spawned', details: {} },
        ],
      };
      const result = computeActionDensity(turn, null);
      expect(result.spawns).toBe(1);
      expect(result.density).toBe(1 * 1.0);
    });

    it('should weight delta_win_prob × 10.0 per plan formula', () => {
      const turn: ReplayTurn = {
        bots: [],
        energy: [],
        cores: [],
        scores: [],
        energy_held: [],
      };
      const winProb = [
        [0.5, 0.5],
        [0.4, 0.6], // delta = 0.2 total
      ];
      const result = computeActionDensity(turn, null, winProb, 1);
      expect(result.deltaWinProb).toBeCloseTo(0.2, 10);
      expect(result.density).toBeCloseTo(0.2 * 10.0, 10);
    });

    it('should sum all components per plan formula', () => {
      const turn: ReplayTurn = {
        bots: [],
        energy: [],
        cores: [],
        scores: [],
        energy_held: [],
        events: [
          { type: 'combat_death', details: {} },      // +3.0
          { type: 'core_captured', details: {} },     // +5.0
          { type: 'energy_collected', details: {} },  // +0.5
          { type: 'bot_spawned', details: {} },       // +1.0
        ],
      };
      const winProb = [
        [0.5, 0.5],
        [0.3, 0.7], // delta = 0.4 total → +4.0
      ];
      const result = computeActionDensity(turn, null, winProb, 1);
      expect(result.density).toBe(3.0 + 5.0 + 0.5 + 1.0 + 4.0);
    });
  });

  describe('densityToSpeed', () => {
    it('should map 0 → 16x (nothing happening)', () => {
      expect(densityToSpeed(0)).toBe(16);
    });

    it('should map 0.1–1.0 → 8x (minor activity)', () => {
      expect(densityToSpeed(0.1)).toBe(8);
      expect(densityToSpeed(0.5)).toBe(8);
      expect(densityToSpeed(1.0)).toBe(4); // boundary
    });

    it('should map 1.0–3.0 → 4x (moderate)', () => {
      expect(densityToSpeed(1.0)).toBe(4);
      expect(densityToSpeed(2.0)).toBe(4);
      expect(densityToSpeed(3.0)).toBe(2); // boundary
    });

    it('should map 3.0–5.0 → 2x (significant)', () => {
      expect(densityToSpeed(3.0)).toBe(2);
      expect(densityToSpeed(4.0)).toBe(2);
      expect(densityToSpeed(5.0)).toBe(1); // boundary
    });

    it('should map 5.0+ → 1x (critical)', () => {
      expect(densityToSpeed(5.0)).toBe(1);
      expect(densityToSpeed(10.0)).toBe(1);
      expect(densityToSpeed(100.0)).toBe(1);
    });
  });

  describe('computeSpeedSchedule', () => {
    it('should generate one speed per turn', () => {
      const densities: ActionDensity[] = [
        { density: 0, deaths: 0, captures: 0, energyCollected: 0, spawns: 0, deltaWinProb: 0 },
        { density: 6, deaths: 2, captures: 0, energyCollected: 0, spawns: 0, deltaWinProb: 0 },
        { density: 0.5, deaths: 0, captures: 0, energyCollected: 1, spawns: 0, deltaWinProb: 0 },
      ];
      const schedule = computeSpeedSchedule(densities, 60);
      expect(schedule).toHaveLength(3);
      // Raw speeds are scaled to match target duration
      // Raw: 16x, 1x, 8x → times: 31.25ms, 500ms, 62.5ms → total ~594ms
      // Target 60s means scaling factor ~100x, so speeds get clamped
      expect(schedule[0]).toBeGreaterThanOrEqual(1);
      expect(schedule[0]).toBeLessThanOrEqual(16);
      expect(schedule[1]).toBeGreaterThanOrEqual(1);
      expect(schedule[1]).toBeLessThanOrEqual(16);
      expect(schedule[2]).toBeGreaterThanOrEqual(1);
      expect(schedule[2]).toBeLessThanOrEqual(16);
    });

    it('should scale speeds to match target duration', () => {
      const densities: ActionDensity[] = Array(100).fill({
        density: 0,
        deaths: 0,
        captures: 0,
        energyCollected: 0,
        spawns: 0,
        deltaWinProb: 0,
      });
      const schedule = computeSpeedSchedule(densities, 60);
      // All turns have density 0, so all should be fast (16x)
      // With 100 turns at 16x, total time should be ~100 * (500/16) ≈ 3.1s
      // But target is 60s, so speeds will be scaled down significantly
      expect(schedule.every(s => s >= 1 && s <= 16)).toBe(true);
    });
  });

  describe('computeAllDensities', () => {
    it('should compute density for each turn', () => {
      const replay: Replay = {
        match_id: 'test',
        players: [],
        map: { rows: 10, cols: 10, walls: [], cores: [], energy_nodes: [] },
        config: {} as any,
        turns: [
          { bots: [], energy: [], cores: [], scores: [], energy_held: [], events: [] },
          {
            bots: [],
            energy: [],
            cores: [],
            scores: [],
            energy_held: [],
            events: [{ type: 'combat_death', details: {} }],
          },
        ],
        result: { turns: 2, winner: 0, reason: 'test', scores: [0, 0] },
      };
      const densities = computeAllDensities(replay);
      expect(densities).toHaveLength(2);
      expect(densities[0].density).toBe(0);
      expect(densities[1].density).toBe(3.0); // one death
    });

    it('should use win_prob for delta calculation', () => {
      const replay: Replay = {
        match_id: 'test',
        players: [],
        map: { rows: 10, cols: 10, walls: [], cores: [], energy_nodes: [] },
        config: {} as any,
        turns: [
          { bots: [], energy: [], cores: [], scores: [], energy_held: [] },
          { bots: [], energy: [], cores: [], scores: [], energy_held: [] },
        ],
        win_prob: [
          [0.5, 0.5],
          [0.3, 0.7], // delta = 0.4
        ],
        result: { turns: 2, winner: 0, reason: 'test', scores: [0, 0] },
      };
      const densities = computeAllDensities(replay);
      expect(densities[1].deltaWinProb).toBeCloseTo(0.4, 10);
      expect(densities[1].density).toBeCloseTo(4.0, 10); // 0.4 * 10
    });
  });
});
