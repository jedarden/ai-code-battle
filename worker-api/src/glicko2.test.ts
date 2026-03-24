import { describe, it, expect } from 'vitest';
import {
  toGlicko2,
  fromGlicko2,
  updateRating,
  g,
  E,
} from './glicko2';

describe('Glicko-2 Rating System', () => {
  describe('Scale Conversion', () => {
    it('converts rating to Glicko-2 scale correctly', () => {
      // Default rating 1500 should map to mu=0
      const result = toGlicko2(1500, 350);
      expect(result.mu).toBe(0);
      expect(result.phi).toBeCloseTo(350 / 173.7178, 10);
    });

    it('converts rating above default correctly', () => {
      const result = toGlicko2(1900, 100);
      expect(result.mu).toBeCloseTo(400 / 173.7178, 10);
      expect(result.phi).toBeCloseTo(100 / 173.7178, 10);
    });

    it('converts rating below default correctly', () => {
      const result = toGlicko2(1300, 200);
      expect(result.mu).toBeCloseTo(-200 / 173.7178, 10);
      expect(result.phi).toBeCloseTo(200 / 173.7178, 10);
    });

    it('round-trips correctly', () => {
      const originalRating = 1650;
      const originalRd = 150;

      const g2 = toGlicko2(originalRating, originalRd);
      const result = fromGlicko2(g2);

      expect(result.rating).toBeCloseTo(originalRating, 10);
      expect(result.rd).toBeCloseTo(originalRd, 10);
    });
  });

  describe('g function', () => {
    it('returns 1 when phi is 0', () => {
      expect(g(0)).toBe(1);
    });

    it('decreases as phi increases', () => {
      const g1 = g(0.1);
      const g2 = g(0.5);
      const g3 = g(1.0);

      expect(g1).toBeGreaterThan(g2);
      expect(g2).toBeGreaterThan(g3);
    });

    it('returns correct values for known inputs', () => {
      // g(0.2) ≈ 0.9955 (from paper example)
      expect(g(0.2)).toBeCloseTo(0.9955, 4);
    });
  });

  describe('E function', () => {
    it('returns 0.5 when ratings are equal', () => {
      const e = E(0, 0, 0.2);
      expect(e).toBeCloseTo(0.5, 10);
    });

    it('returns > 0.5 when player rating is higher', () => {
      const e = E(0.5, 0, 0.2); // Player rated higher
      expect(e).toBeGreaterThan(0.5);
    });

    it('returns < 0.5 when opponent rating is higher', () => {
      const e = E(0, 0.5, 0.2); // Opponent rated higher
      expect(e).toBeLessThan(0.5);
    });
  });

  describe('Rating Updates', () => {
    it('increases rating after win against equal opponent', () => {
      const bot = {
        id: 'test',
        name: 'Test',
        owner_id: 'owner',
        endpoint_url: 'http://example.com',
        api_key_hash: 'hash',
        rating: 1500,
        rating_deviation: 200,
        rating_volatility: 0.06,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
        last_health_check: null,
        health_status: 'healthy' as const,
        matches_played: 0,
        matches_won: 0,
      };

      const opponents = [
        { rating: 1500, rd: 200, score: 1 }, // Win
      ];

      const result = updateRating(bot, opponents);

      // Rating should increase after winning
      expect(result.rating).toBeGreaterThan(1500);
      // RD should decrease after playing
      expect(result.rd).toBeLessThan(200);
    });

    it('decreases rating after loss against equal opponent', () => {
      const bot = {
        id: 'test',
        name: 'Test',
        owner_id: 'owner',
        endpoint_url: 'http://example.com',
        api_key_hash: 'hash',
        rating: 1500,
        rating_deviation: 200,
        rating_volatility: 0.06,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
        last_health_check: null,
        health_status: 'healthy' as const,
        matches_played: 0,
        matches_won: 0,
      };

      const opponents = [
        { rating: 1500, rd: 200, score: 0 }, // Loss
      ];

      const result = updateRating(bot, opponents);

      // Rating should decrease after losing
      expect(result.rating).toBeLessThan(1500);
      // RD should decrease after playing
      expect(result.rd).toBeLessThan(200);
    });

    it('handles draw correctly', () => {
      const bot = {
        id: 'test',
        name: 'Test',
        owner_id: 'owner',
        endpoint_url: 'http://example.com',
        api_key_hash: 'hash',
        rating: 1500,
        rating_deviation: 200,
        rating_volatility: 0.06,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
        last_health_check: null,
        health_status: 'healthy' as const,
        matches_played: 0,
        matches_won: 0,
      };

      const opponents = [
        { rating: 1500, rd: 200, score: 0.5 }, // Draw
      ];

      const result = updateRating(bot, opponents);

      // Rating should stay roughly the same against equal opponent
      expect(result.rating).toBeCloseTo(1500, 1);
      // RD should decrease after playing
      expect(result.rd).toBeLessThan(200);
    });

    it('handles multiple opponents', () => {
      const bot = {
        id: 'test',
        name: 'Test',
        owner_id: 'owner',
        endpoint_url: 'http://example.com',
        api_key_hash: 'hash',
        rating: 1500,
        rating_deviation: 200,
        rating_volatility: 0.06,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
        last_health_check: null,
        health_status: 'healthy' as const,
        matches_played: 0,
        matches_won: 0,
      };

      const opponents = [
        { rating: 1600, rd: 150, score: 1 }, // Win vs higher rated
        { rating: 1400, rd: 150, score: 0 }, // Loss vs lower rated
      ];

      const result = updateRating(bot, opponents);

      // Both rating and RD should be updated
      expect(result.rating).toBeGreaterThan(0);
      expect(result.rd).toBeLessThan(200);
    });

    it('increases RD when no games played (rating decay)', () => {
      const bot = {
        id: 'test',
        name: 'Test',
        owner_id: 'owner',
        endpoint_url: 'http://example.com',
        api_key_hash: 'hash',
        rating: 1500,
        rating_deviation: 100,
        rating_volatility: 0.06,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
        last_health_check: null,
        health_status: 'healthy' as const,
        matches_played: 0,
        matches_won: 0,
      };

      const result = updateRating(bot, []);

      // Rating should stay the same
      expect(result.rating).toBe(1500);
      // RD should increase (rating decay)
      expect(result.rd).toBeGreaterThan(100);
    });

    it('constrains RD to maximum', () => {
      const bot = {
        id: 'test',
        name: 'Test',
        owner_id: 'owner',
        endpoint_url: 'http://example.com',
        api_key_hash: 'hash',
        rating: 1500,
        rating_deviation: 340,
        rating_volatility: 0.5, // High volatility
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
        last_health_check: null,
        health_status: 'healthy' as const,
        matches_played: 0,
        matches_won: 0,
      };

      const result = updateRating(bot, []);

      // RD should not exceed 350
      expect(result.rd).toBeLessThanOrEqual(350);
    });
  });

  describe('Real-world scenarios', () => {
    it('matches expected rating change from Glicko-2 paper example', () => {
      // This is a simplified test based on the Glicko-2 paper
      // Player with rating 1500, RD 200 playing against:
      // - Opponent 1: 1400, 30, win (score=1)
      // - Opponent 2: 1550, 100, loss (score=0)
      // - Opponent 3: 1700, 300, loss (score=0)

      const bot = {
        id: 'test',
        name: 'Test',
        owner_id: 'owner',
        endpoint_url: 'http://example.com',
        api_key_hash: 'hash',
        rating: 1500,
        rating_deviation: 200,
        rating_volatility: 0.06,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
        last_health_check: null,
        health_status: 'healthy' as const,
        matches_played: 0,
        matches_won: 0,
      };

      const opponents = [
        { rating: 1400, rd: 30, score: 1 },
        { rating: 1550, rd: 100, score: 0 },
        { rating: 1700, rd: 300, score: 0 },
      ];

      const result = updateRating(bot, opponents);

      // The new rating should be in a reasonable range
      // Based on the paper, expected new rating is approximately 1464
      expect(result.rating).toBeGreaterThan(1400);
      expect(result.rating).toBeLessThan(1550);
      expect(result.rd).toBeLessThan(200);
    });
  });
});
