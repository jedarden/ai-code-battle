// Tests for extract-significant-events module

import { describe, it, expect } from 'vitest';
import {
  extractSignificantEvents,
  getEventsInRange,
  groupEventsByTurn,
  getEventSummary,
  type SignificantEvent,
} from './extract-significant-events';
import type { Replay, ReplayTurn, GameEvent, Position } from './types';

describe('extractSignificantEvents', () => {
  it('should handle empty replay data', () => {
    const emptyReplay = {} as Replay;
    const events = extractSignificantEvents(emptyReplay);
    expect(events).toEqual([]);
  });

  it('should handle replay with no turns', () => {
    const replay: Partial<Replay> = {
      turns: [],
      players: [],
    } as Replay;
    const events = extractSignificantEvents(replay);
    expect(events).toEqual([]);
  });

  it('should extract combat events from bot_died events', () => {
    const position: Position = { row: 5, col: 10 };
    const replay: Partial<Replay> = {
      players: [
        { id: 0, name: 'AlphaBot' },
        { id: 1, name: 'BetaBot' },
      ],
      turns: [
        {
          turn: 10,
          bots: [],
          cores: [],
          energy: [],
          scores: [10, 20],
          energy_held: [5, 15],
          events: [
            {
              type: 'bot_died',
              turn: 10,
              details: {
                bot_id: 1,
                owner: 0,
                position,
              },
            },
          ],
        },
      ],
      result: {
        winner: 1,
        reason: 'dominance',
        turns: 11,
        scores: [10, 20],
        energy: [5, 15],
        bots_alive: [0, 2],
      },
    } as Replay;

    const events = extractSignificantEvents(replay);
    const combatEvents = events.filter(e => e.type === 'combat');
    expect(combatEvents.length).toBeGreaterThan(0);
    expect(combatEvents[0].turn).toBe(10);
    expect(combatEvents[0].description).toContain('AlphaBot');
    expect(combatEvents[0].emoji).toBe('⚔️');
  });

  it('should extract core capture events', () => {
    const position: Position = { row: 3, col: 7 };
    const replay: Partial<Replay> = {
      players: [
        { id: 0, name: 'AlphaBot' },
        { id: 1, name: 'BetaBot' },
      ],
      turns: [
        {
          turn: 25,
          bots: [],
          cores: [],
          energy: [],
          scores: [15, 15],
          energy_held: [10, 10],
          events: [
            {
              type: 'core_captured',
              turn: 25,
              details: {
                position,
                old_owner: 1,
                new_owner: 0,
              },
            },
          ],
        },
      ],
      result: {
        winner: 0,
        reason: 'dominance',
        turns: 26,
        scores: [15, 15],
        energy: [10, 10],
        bots_alive: [2, 2],
      },
    } as Replay;

    const events = extractSignificantEvents(replay);
    const coreEvents = events.filter(e => e.type === 'core_capture');
    expect(coreEvents.length).toBeGreaterThan(0);

    const captureEvent = coreEvents.find(e => e.description.includes('captured'));
    expect(captureEvent).toBeDefined();
    expect(captureEvent!.description).toContain('AlphaBot');
    expect(captureEvent!.description).toContain('BetaBot');
    expect(captureEvent!.emoji).toBe('🏰');
  });

  it('should detect mass death events', () => {
    const replay: Partial<Replay> = {
      players: [
        { id: 0, name: 'AlphaBot' },
        { id: 1, name: 'BetaBot' },
      ],
      turns: [
        {
          turn: 10,
          bots: [],
          cores: [],
          energy: [],
          scores: [10, 20],
          energy_held: [5, 15],
          events: [
            { type: 'bot_died', turn: 10, details: { bot_id: 1, owner: 0, position: { row: 0, col: 0 } } },
            { type: 'bot_died', turn: 10, details: { bot_id: 2, owner: 1, position: { row: 0, col: 0 } } },
            { type: 'bot_died', turn: 10, details: { bot_id: 3, owner: 0, position: { row: 0, col: 0 } } },
          ],
        },
      ],
      result: {
        winner: 1,
        reason: 'dominance',
        turns: 11,
        scores: [10, 20],
        energy: [5, 15],
        bots_alive: [0, 2],
      },
    } as Replay;

    const events = extractSignificantEvents(replay);
    const massDeathEvents = events.filter(e => e.type === 'mass_death');
    expect(massDeathEvents.length).toBeGreaterThan(0);
    expect(massDeathEvents[0].emoji).toBe('💀');
  });

  it('should detect momentum shifts when lead changes', () => {
    const replay: Partial<Replay> = {
      players: [
        { id: 0, name: 'AlphaBot' },
        { id: 1, name: 'BetaBot' },
      ],
      turns: [
        {
          turn: 5,
          bots: [],
          cores: [],
          energy: [],
          scores: [10, 20],
          energy_held: [5, 10],
        },
        {
          turn: 10,
          bots: [],
          cores: [],
          energy: [],
          scores: [25, 20],
          energy_held: [10, 10],
        },
      ],
      result: {
        winner: 0,
        reason: 'dominance',
        turns: 11,
        scores: [25, 20],
        energy: [10, 10],
        bots_alive: [2, 2],
      },
    } as Replay;

    const events = extractSignificantEvents(replay);
    const momentumEvents = events.filter(e => e.type === 'momentum_shift');
    expect(momentumEvents.length).toBeGreaterThan(0);
    expect(momentumEvents[0].description).toContain('AlphaBot');
    expect(momentumEvents[0].description).toContain('takes lead');
    expect(momentumEvents[0].emoji).toBe('📈');
  });

  it('should detect energy milestones', () => {
    const replay: Partial<Replay> = {
      players: [
        { id: 0, name: 'AlphaBot' },
        { id: 1, name: 'BetaBot' },
      ],
      turns: [
        {
          turn: 15,
          bots: [],
          cores: [],
          energy: [],
          scores: [10, 20],
          energy_held: [50, 25], // AlphaBot hits 50 energy
        },
      ],
      result: {
        winner: 1,
        reason: 'dominance',
        turns: 16,
        scores: [10, 20],
        energy: [50, 25],
        bots_alive: [2, 2],
      },
    } as Replay;

    const events = extractSignificantEvents(replay);
    const energyEvents = events.filter(e => e.type === 'energy_milestone');
    expect(energyEvents.length).toBeGreaterThan(0);
    expect(energyEvents[0].description).toContain('50');
    expect(energyEvents[0].emoji).toBe('💎');
  });

  it('should detect spawn wave events', () => {
    const position: Position = { row: 2, col: 3 };
    const replay: Partial<Replay> = {
      players: [
        { id: 0, name: 'AlphaBot' },
        { id: 1, name: 'BetaBot' },
      ],
      turns: [
        {
          turn: 5,
          bots: [],
          cores: [],
          energy: [],
          scores: [5, 10],
          energy_held: [10, 5],
          events: [
            {
              type: 'bot_spawned',
              turn: 5,
              details: {
                bot_id: 5,
                owner: 0,
                position,
              },
            },
          ],
        },
      ],
      result: {
        winner: 1,
        reason: 'dominance',
        turns: 6,
        scores: [5, 10],
        energy: [10, 5],
        bots_alive: [2, 2],
      },
    } as Replay;

    const events = extractSignificantEvents(replay);
    const spawnEvents = events.filter(e => e.type === 'spawn_wave');
    expect(spawnEvents.length).toBeGreaterThan(0);
    expect(spawnEvents[0].description).toContain('AlphaBot');
    expect(spawnEvents[0].emoji).toBe('🐣');
  });

  it('should add game end event', () => {
    const replay: Partial<Replay> = {
      players: [
        { id: 0, name: 'AlphaBot' },
        { id: 1, name: 'BetaBot' },
      ],
      turns: [
        {
          turn: 0,
          bots: [],
          cores: [],
          energy: [],
          scores: [0, 0],
          energy_held: [0, 0],
        },
        {
          turn: 1,
          bots: [],
          cores: [],
          energy: [],
          scores: [10, 5],
          energy_held: [5, 2],
        },
      ],
      result: {
        winner: 0,
        reason: 'dominance',
        turns: 2,
        scores: [10, 5],
        energy: [5, 2],
        bots_alive: [2, 1],
      },
    } as Replay;

    const events = extractSignificantEvents(replay);
    const gameEndEvents = events.filter(e => e.type === 'critical_moment');
    expect(gameEndEvents.length).toBeGreaterThan(0);
    expect(gameEndEvents[0].description).toContain('Game ends');
    expect(gameEndEvents[0].emoji).toBe('🏆');
  });

  it('should sort events by turn', () => {
    const replay: Partial<Replay> = {
      players: [
        { id: 0, name: 'AlphaBot' },
        { id: 1, name: 'BetaBot' },
      ],
      turns: [
        {
          turn: 20,
          bots: [],
          cores: [],
          energy: [],
          scores: [20, 10],
          energy_held: [10, 5],
          events: [
            { type: 'core_captured', turn: 20, details: { old_owner: 1, new_owner: 0, position: { row: 0, col: 0 } } },
          ],
        },
        {
          turn: 10,
          bots: [],
          cores: [],
          energy: [],
          scores: [10, 20],
          energy_held: [5, 10],
          events: [
            { type: 'bot_died', turn: 10, details: { bot_id: 1, owner: 0, position: { row: 0, col: 0 } } },
          ],
        },
      ],
      result: {
        winner: 0,
        reason: 'dominance',
        turns: 21,
        scores: [20, 10],
        energy: [10, 5],
        bots_alive: [2, 1],
      },
    } as Replay;

    const events = extractSignificantEvents(replay);
    expect(events.length).toBeGreaterThan(1);

    for (let i = 1; i < events.length; i++) {
      expect(events[i].turn).toBeGreaterThanOrEqual(events[i - 1].turn);
    }
  });
});

describe('getEventsInRange', () => {
  it('should filter events by turn range', () => {
    const events: SignificantEvent[] = [
      { type: 'combat', turn: 5, description: 'Early fight', emoji: '⚔️' },
      { type: 'core_capture', turn: 10, description: 'Core taken', emoji: '🏰' },
      { type: 'mass_death', turn: 15, description: 'Many died', emoji: '💀' },
      { type: 'momentum_shift', turn: 20, description: 'Lead change', emoji: '📈' },
    ];

    const filtered = getEventsInRange(events, 10, 15);
    expect(filtered).toHaveLength(2);
    expect(filtered.every(e => e.turn >= 10 && e.turn <= 15)).toBe(true);
  });
});

describe('groupEventsByTurn', () => {
  it('should group events by turn number', () => {
    const events: SignificantEvent[] = [
      { type: 'combat', turn: 10, description: 'Fight 1', emoji: '⚔️' },
      { type: 'combat', turn: 10, description: 'Fight 2', emoji: '⚔️' },
      { type: 'core_capture', turn: 15, description: 'Core taken', emoji: '🏰' },
    ];

    const grouped = groupEventsByTurn(events);
    expect(grouped.size).toBe(2);
    expect(grouped.get(10)).toHaveLength(2);
    expect(grouped.get(15)).toHaveLength(1);
  });
});

describe('getEventSummary', () => {
  it('should count events by type', () => {
    const events: SignificantEvent[] = [
      { type: 'combat', turn: 5, description: 'Fight', emoji: '⚔️' },
      { type: 'combat', turn: 10, description: 'Fight', emoji: '⚔️' },
      { type: 'core_capture', turn: 15, description: 'Core', emoji: '🏰' },
      { type: 'energy_milestone', turn: 20, description: 'Energy', emoji: '💎' },
    ];

    const summary = getEventSummary(events);
    expect(summary.combat).toBe(2);
    expect(summary.core_capture).toBe(1);
    expect(summary.energy_milestone).toBe(1);
  });
});
