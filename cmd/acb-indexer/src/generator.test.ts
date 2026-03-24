// Index Generator Tests

import { describe, it, expect } from 'vitest';
import { IndexGenerator } from './generator.js';
import type { ExportData, ExportBot, ExportMatch } from './types.js';

function createMockData(): ExportData {
  const bots: ExportBot[] = [
    {
      id: 'bot-1',
      name: 'TestBot1',
      owner_id: 'owner-1',
      rating: 1500,
      rating_deviation: 50,
      rating_volatility: 0.06,
      matches_played: 10,
      matches_won: 7,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-03-01T00:00:00Z',
      health_status: 'healthy',
    },
    {
      id: 'bot-2',
      name: 'TestBot2',
      owner_id: 'owner-2',
      rating: 1450,
      rating_deviation: 60,
      rating_volatility: 0.07,
      matches_played: 5,
      matches_won: 2,
      created_at: '2026-01-15T00:00:00Z',
      updated_at: '2026-03-01T00:00:00Z',
      health_status: 'healthy',
    },
    {
      id: 'bot-3',
      name: 'UnrankedBot',
      owner_id: 'owner-3',
      rating: 1200,
      rating_deviation: 350,
      rating_volatility: 0.06,
      matches_played: 0,
      matches_won: 0,
      created_at: '2026-02-01T00:00:00Z',
      updated_at: '2026-02-01T00:00:00Z',
      health_status: 'unknown',
    },
  ];

  const matches: ExportMatch[] = [
    {
      id: 'match-1',
      status: 'completed',
      winner_id: 'bot-1',
      turns: 50,
      end_reason: 'domination',
      map_id: 'map-1',
      created_at: '2026-03-01T10:00:00Z',
      completed_at: '2026-03-01T10:05:00Z',
      participants: [
        {
          bot_id: 'bot-1',
          player_index: 0,
          score: 100,
          rating_before: 1480,
          rating_after: 1500,
        },
        {
          bot_id: 'bot-2',
          player_index: 1,
          score: 50,
          rating_before: 1470,
          rating_after: 1450,
        },
      ],
    },
  ];

  return {
    bots,
    matches,
    rating_history: [
      {
        bot_id: 'bot-1',
        rating: 1480,
        rating_deviation: 55,
        recorded_at: '2026-02-15T00:00:00Z',
      },
      {
        bot_id: 'bot-1',
        rating: 1500,
        rating_deviation: 50,
        recorded_at: '2026-03-01T00:00:00Z',
      },
    ],
    generated_at: '2026-03-24T08:00:00Z',
  };
}

describe('IndexGenerator', () => {
  it('generates leaderboard with correct rankings', () => {
    const generator = new IndexGenerator(createMockData());
    const leaderboard = generator.generateLeaderboard();

    expect(leaderboard.updated_at).toBe('2026-03-24T08:00:00Z');
    expect(leaderboard.entries).toHaveLength(2); // Only bots with matches
    expect(leaderboard.entries[0].bot_id).toBe('bot-1');
    expect(leaderboard.entries[0].rank).toBe(1);
    expect(leaderboard.entries[0].rating).toBe(1500);
    expect(leaderboard.entries[0].win_rate).toBe(70); // 7/10 * 100
  });

  it('generates bot directory', () => {
    const generator = new IndexGenerator(createMockData());
    const directory = generator.generateBotDirectory();

    expect(directory.bots).toHaveLength(3);
    expect(directory.bots[0].id).toBe('bot-1');
    expect(directory.bots[0].name).toBe('TestBot1');
  });

  it('generates bot profile with rating history', () => {
    const generator = new IndexGenerator(createMockData());
    const profile = generator.generateBotProfile('bot-1');

    expect(profile).not.toBeNull();
    expect(profile!.id).toBe('bot-1');
    expect(profile!.name).toBe('TestBot1');
    expect(profile!.rating_history).toHaveLength(2);
    expect(profile!.recent_matches).toHaveLength(1);
    expect(profile!.recent_matches[0].participants[0].won).toBe(true);
  });

  it('returns null for non-existent bot profile', () => {
    const generator = new IndexGenerator(createMockData());
    const profile = generator.generateBotProfile('non-existent');

    expect(profile).toBeNull();
  });

  it('generates match index', () => {
    const generator = new IndexGenerator(createMockData());
    const matchIndex = generator.generateMatchIndex();

    expect(matchIndex.matches).toHaveLength(1);
    expect(matchIndex.matches[0].id).toBe('match-1');
    expect(matchIndex.matches[0].winner_id).toBe('bot-1');
    expect(matchIndex.matches[0].participants).toHaveLength(2);
  });

  it('generates all indexes at once', () => {
    const generator = new IndexGenerator(createMockData());
    const all = generator.generateAll();

    expect(all.leaderboard.entries).toHaveLength(2);
    expect(all.botDirectory.bots).toHaveLength(3);
    expect(all.botProfiles.size).toBe(3);
    expect(all.matchIndex.matches).toHaveLength(1);
  });
});
