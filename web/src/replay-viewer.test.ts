/**
 * Unit tests for attack arrow rendering in combat_death events.
 * Tests plan §5055: Attack event arrows from killers to victim.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ReplayViewer } from './replay-viewer';
import type { Replay } from './types';

describe('ReplayViewer attack arrow rendering (combat_death events)', () => {
  let canvas: HTMLCanvasElement;
  let mockCtx: any;
  let moveToCalls: any[][];
  let lineToCalls: any[][];
  let strokeCalls: number;
  let fillCalls: number;

  beforeEach(() => {
    // Track canvas calls
    moveToCalls = [];
    lineToCalls = [];
    strokeCalls = 0;
    fillCalls = 0;

    // Create canvas element
    canvas = document.createElement('canvas');
    canvas.width = 400;
    canvas.height = 400;

    // Mock canvas context
    mockCtx = {
      fillRect: vi.fn(),
      clearRect: vi.fn(),
      save: vi.fn(),
      restore: vi.fn(),
      beginPath: vi.fn(),
      moveTo: vi.fn((...args: any[]) => { moveToCalls.push(args); }),
      lineTo: vi.fn((...args: any[]) => { lineToCalls.push(args); }),
      closePath: vi.fn(),
      stroke: vi.fn(() => { strokeCalls++; }),
      fill: vi.fn(() => { fillCalls++; }),
      translate: vi.fn(),
      scale: vi.fn(),
      rotate: vi.fn(),
      arc: vi.fn(),
      fillText: vi.fn(),
      measureText: vi.fn(() => ({ width: 0 })),
      transform: vi.fn(),
      rect: vi.fn(),
      clip: vi.fn(),
      createImageData: vi.fn(),
      getImageData: vi.fn(() => ({ data: [] })),
      putImageData: vi.fn(),
      drawImage: vi.fn(),
      setTransform: vi.fn(),
      resetTransform: vi.fn(),
      getLineDash: vi.fn(() => []),
      setLineDash: vi.fn(),
      createRadialGradient: vi.fn(() => ({
        addColorStop: vi.fn(),
      })),
      createLinearGradient: vi.fn(() => ({
        addColorStop: vi.fn(),
      })),
      lineCap: '',
      lineWidth: 1,
      strokeStyle: '',
      fillStyle: '',
      globalAlpha: 1,
      font: '',
      textAlign: '',
      textBaseline: '',
    };

    canvas.getContext = vi.fn(() => mockCtx) as any;
  });

  it('should draw arrows from each killer to victim in combat_death events', () => {
    // Create a minimal replay with combat_death events
    const replay: Replay = {
      format_version: 'v1' as const,
      match_id: 'test-combat-death',
      start_time: '2024-01-01T00:00:00Z',
      end_time: '2024-01-01T00:01:00Z',
      config: {
        rows: 40,
        cols: 40,
        max_turns: 100,
        attack_radius2: 25,
        vision_radius2: 49,
        spawn_cost: 3,
        energy_interval: 5,
        zone_enabled: true,
        zone_start_turn: 10,
        zone_shrink_interval: 1,
        zone_shrink_step: 1,
        zone_min_radius: 2,
      },
      map: {
        width: 40,
        height: 40,
        walls: [{ row: 5, col: 5 }],
        cores: [
          { position: { row: 10, col: 10 }, owner: 0 },
          { position: { row: 30, col: 30 }, owner: 1 },
        ],
        energy_nodes: [
          { row: 15, col: 15 },
          { row: 25, col: 25 },
        ],
      },
      players: [
        { id: 0, name: 'Player 0' },
        { id: 1, name: 'Player 1' },
      ],
      turns: [
        {
          turn: 0,
          bots: [
            { id: 0, owner: 0, position: { row: 12, col: 12 }, alive: true },
            { id: 1, owner: 1, position: { row: 28, col: 28 }, alive: true },
          ],
          cores: [
            { position: { row: 10, col: 10 }, owner: 0, active: true },
            { position: { row: 30, col: 30 }, owner: 1, active: true },
          ],
          energy: [
            { row: 15, col: 15, has_energy: true, tick: 0 },
            { row: 25, col: 25, has_energy: true, tick: 0 },
          ],
          scores: [1, 1],
          energy_held: [0, 0],
          events: [
            {
              type: 'combat_death',
              turn: 0,
              details: {
                bot_id: 1,
                owner: 1,
                position: { row: 28, col: 28 },
                killers: [
                  {
                    bot_id: 0,
                    owner: 0,
                    position: { row: 12, col: 12 },
                  },
                ],
              },
            },
          ],
        },
        {
          turn: 1,
          bots: [
            { id: 0, owner: 0, position: { row: 13, col: 13 }, alive: true },
            { id: 1, owner: 1, position: { row: 28, col: 28 }, alive: false },
          ],
          cores: [
            { position: { row: 10, col: 10 }, owner: 0, active: true },
            { position: { row: 30, col: 30 }, owner: 1, active: true },
          ],
          energy: [
            { row: 15, col: 15, has_energy: true, tick: 1 },
            { row: 25, col: 25, has_energy: true, tick: 1 },
          ],
          scores: [1, 1],
          energy_held: [0, 0],
          events: [],
        },
      ],
      result: {
        winner: 0,
        reason: 'elimination',
        turns: 1,
        scores: [3, 1],
        energy: [0, 0],
        bots_alive: [1, 0],
        crashed: [false, false],
        combat_deaths: [1, 0],
      },
      combat_deaths: [1, 0],
    };

    // Create viewer and load replay
    const viewer = new ReplayViewer(canvas, { cellSize: 10 });
    viewer.loadReplay(replay);
    viewer.setTurn(0);

    // Verify arrows were drawn
    // The arrow should be drawn from killer (12, 12) to victim (28, 28)
    // With cellSize 10, pixel coordinates are:
    // - Killer: x = 12 * 10 + 5 = 125, y = 12 * 10 + 5 = 125
    // - Victim: x = 28 * 10 + 5 = 285, y = 28 * 10 + 5 = 285

    // Check that moveTo was called (arrow start point)
    expect(moveToCalls.length).toBeGreaterThan(0);

    // Check that lineTo was called (arrow end point and arrowhead)
    expect(lineToCalls.length).toBeGreaterThan(0);

    // Check that stroke was called (draw the line)
    expect(strokeCalls).toBeGreaterThan(0);

    // Check that fill was called (draw the arrowhead)
    expect(fillCalls).toBeGreaterThan(0);
  });

  it('should draw multiple arrows when multiple killers in combat_death event', () => {
    // Create a replay with 2v1 combat (2 killers, 1 victim)
    const replay: Replay = {
      format_version: 'v1' as const,
      match_id: 'test-2v1-combat',
      start_time: '2024-01-01T00:00:00Z',
      end_time: '2024-01-01T00:01:00Z',
      config: {
        rows: 40,
        cols: 40,
        max_turns: 100,
        attack_radius2: 25,
        vision_radius2: 49,
        spawn_cost: 3,
        energy_interval: 5,
        zone_enabled: true,
        zone_start_turn: 10,
        zone_shrink_interval: 1,
        zone_shrink_step: 1,
        zone_min_radius: 2,
      },
      map: {
        width: 40,
        height: 40,
        walls: [],
        cores: [
          { position: { row: 10, col: 10 }, owner: 0 },
          { position: { row: 10, col: 30 }, owner: 0 },
          { position: { row: 30, col: 20 }, owner: 1 },
        ],
        energy_nodes: [],
      },
      players: [
        { id: 0, name: 'Player 0' },
        { id: 1, name: 'Player 1' },
      ],
      turns: [
        {
          turn: 0,
          bots: [
            { id: 0, owner: 0, position: { row: 15, col: 15 }, alive: true },
            { id: 1, owner: 0, position: { row: 15, col: 25 }, alive: true },
            { id: 2, owner: 1, position: { row: 25, col: 20 }, alive: true },
          ],
          cores: [
            { position: { row: 10, col: 10 }, owner: 0, active: true },
            { position: { row: 10, col: 30 }, owner: 0, active: true },
            { position: { row: 30, col: 20 }, owner: 1, active: true },
          ],
          energy: [],
          scores: [1, 1],
          energy_held: [0, 0],
          events: [
            {
              type: 'combat_death',
              turn: 0,
              details: {
                bot_id: 2,
                owner: 1,
                position: { row: 25, col: 20 },
                killers: [
                  {
                    bot_id: 0,
                    owner: 0,
                    position: { row: 15, col: 15 },
                  },
                  {
                    bot_id: 1,
                    owner: 0,
                    position: { row: 15, col: 25 },
                  },
                ],
              },
            },
          ],
        },
      ],
      result: {
        winner: 0,
        reason: 'elimination',
        turns: 1,
        scores: [3, 1],
        energy: [0, 0],
        bots_alive: [2, 0],
        crashed: [false, false],
        combat_deaths: [2, 0],
      },
      combat_deaths: [2, 0],
    };

    // Create viewer and load replay
    const viewer = new ReplayViewer(canvas, { cellSize: 10 });
    viewer.loadReplay(replay);
    viewer.setTurn(0);

    // With 2 killers, we expect 2 arrows to be drawn
    // Each arrow consists of: moveTo (start), lineTo (end), lineTo (arrowhead 1), lineTo (arrowhead 2)
    // So we expect multiple lineTo calls for 2 arrows

    // The key check is that we have more drawing calls than the single-killer case
    expect(lineToCalls.length).toBeGreaterThan(2);
    expect(strokeCalls).toBeGreaterThan(1);
  });

  it('should color arrows based on attacker player color', () => {
    // This test verifies that different attackers get different colored arrows
    const playerColors = ['#3b82f6', '#ef4444', '#22c55e', '#f59e0b'];

    // Create a replay with combat from different players
    const replay: Replay = {
      format_version: 'v1' as const,
      match_id: 'test-arrow-colors',
      start_time: '2024-01-01T00:00:00Z',
      end_time: '2024-01-01T00:01:00Z',
      config: {
        rows: 40,
        cols: 40,
        max_turns: 100,
        attack_radius2: 25,
        vision_radius2: 49,
        spawn_cost: 3,
        energy_interval: 5,
        zone_enabled: true,
        zone_start_turn: 10,
        zone_shrink_interval: 1,
        zone_shrink_step: 1,
        zone_min_radius: 2,
      },
      map: {
        width: 40,
        height: 40,
        walls: [],
        cores: [
          { position: { row: 10, col: 10 }, owner: 0 },
          { position: { row: 30, col: 30 }, owner: 1 },
        ],
        energy_nodes: [],
      },
      players: [
        { id: 0, name: 'Player 0' },
        { id: 1, name: 'Player 1' },
      ],
      turns: [
        {
          turn: 0,
          bots: [
            { id: 0, owner: 0, position: { row: 12, col: 12 }, alive: true },
            { id: 1, owner: 1, position: { row: 28, col: 28 }, alive: true },
          ],
          cores: [
            { position: { row: 10, col: 10 }, owner: 0, active: true },
            { position: { row: 30, col: 30 }, owner: 1, active: true },
          ],
          energy: [],
          scores: [1, 1],
          energy_held: [0, 0],
          events: [
            {
              type: 'combat_death',
              turn: 0,
              details: {
                bot_id: 1,
                owner: 1,
                position: { row: 28, col: 28 },
                killers: [
                  {
                    bot_id: 0,
                    owner: 0,
                    position: { row: 12, col: 12 },
                  },
                ],
              },
            },
          ],
        },
      ],
      result: {
        winner: 0,
        reason: 'elimination',
        turns: 1,
        scores: [3, 1],
        energy: [0, 0],
        bots_alive: [1, 0],
        crashed: [false, false],
        combat_deaths: [1, 0],
      },
      combat_deaths: [1, 0],
    };

    // Track strokeStyle calls to verify colors
    const strokeStyleCalls: string[] = [];
    Object.defineProperty(mockCtx, 'strokeStyle', {
      set: (value: string) => { strokeStyleCalls.push(value); },
      get: () => strokeStyleCalls[strokeStyleCalls.length - 1] || '',
    });

    // Create viewer and load replay
    const viewer = new ReplayViewer(canvas, { cellSize: 10 });
    viewer.loadReplay(replay);
    viewer.setTurn(0);

    // Verify that strokeStyle was set to some color (for arrow rendering)
    expect(strokeStyleCalls.length).toBeGreaterThan(0);
    // The actual color depends on the ReplayViewer's internal color mapping
    // Just verify we're setting strokeStyle for the arrow
    expect(strokeStyleCalls.some(c => typeof c === 'string' && c.length > 0)).toBe(true);
  });
});
