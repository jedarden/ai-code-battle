/**
 * Pins the replay viewer's per-player fog-of-war perspective toggle
 * (docs/notes/requirements.md, "Replay Visualization"):
 *
 *   - Per-player perspective toggle (fog of war from one player's view)
 *   - Full-map omniscient view (the default)
 *   - Playback speed control
 *   - Score/resource overlay per turn
 *
 * Per-turn visibility is not carried in the replay data — it is
 * reconstructed client-side per turn from the selected player's alive bots,
 * config.vision_radius2 (squared, toroidal), plus that player's own cores
 * (ReplayViewer.computeVisibility). These tests pin that reconstruction:
 * a cell is visible iff it is within sight radius of an alive bot of the
 * selected player, or holds that player's core.
 *
 * Style note: like replay-swap-parity.spec.ts, everything here imports the
 * real production code rather than mirroring it — the real ReplayViewer
 * driven through its public API, and the real replayPageMarkup() markup the
 * page mounts — so a change to either side shows up here as a failure, not
 * as drift between a test copy and the app. The harness is the vitest/jsdom
 * suite rather than the Playwright layout harness because nothing pinned
 * here is layout: it is canvas draw geometry and control markup, both
 * deterministic in jsdom with a recording 2D context (same approach as
 * replay-viewer.test.ts).
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ReplayViewer } from './replay-viewer';
import { replayPageMarkup } from './pages/replay';
import type { Replay } from './types';

// ── Fixture ─────────────────────────────────────────────────────────────────────
//
// 10x10 grid, vision_radius2 = 8 (so offsets with dr²+dc² ≤ 8 — a 25-cell
// disc, NOT the full 7x7 square). Player 0 ("Alpha") has one bot; player 1
// ("Beta") has one. cellSize 20 turns cells into pixels: cell (r,c) centers
// at (c*20+10, r*20+10).

const CS = 20;
const VISION_RADIUS2 = 8;
const FOG_COLOR = 'rgba(10,10,30,0.7)'; // renderFogOverlay's non-high-contrast fill

const GRID = 10;
// Cells within VISION_RADIUS2 of a bot, as (dr, dc) offsets: exactly 25.
// The boundary matters: (±3,0)/(0,±3) are 9 > 8 and excluded; (±2,±2) are
// exactly 8 and included.
const RADIUS_OFFSETS: Array<[number, number]> = (() => {
  const offsets: Array<[number, number]> = [];
  for (let dr = -3; dr <= 3; dr++) {
    for (let dc = -3; dc <= 3; dc++) {
      if (dr * dr + dc * dc <= VISION_RADIUS2) offsets.push([dr, dc]);
    }
  }
  return offsets;
})();

function makeReplay(): Replay {
  return {
    match_id: 'fog-test',
    config: {
      rows: GRID,
      cols: GRID,
      max_turns: 2,
      vision_radius2: VISION_RADIUS2,
      attack_radius2: 2,
      spawn_cost: 5,
      energy_interval: 5,
    },
    start_time: '2026-09-15T00:00:00Z',
    end_time: '2026-09-15T00:01:00Z',
    result: { winner: -1, reason: 'max_turns', turns: 2, scores: [15, 7], energy: [0, 0], bots_alive: [1, 1] },
    players: [
      { id: 0, name: 'Alpha' },
      { id: 1, name: 'Beta' },
    ],
    map: {
      rows: GRID,
      cols: GRID,
      walls: [],
      // Alpha's core sits OUTSIDE her own sight radius from her bot (4²+4²=32 > 8):
      // a core must be visible to its owner without extending their vision.
      cores: [
        { position: { row: 5, col: 5 }, owner: 0 },
        { position: { row: 8, col: 8 }, owner: 1 },
      ],
      energy_nodes: [],
    },
    turns: [
      {
        turn: 0,
        bots: [
          { id: 0, owner: 0, position: { row: 1, col: 1 }, alive: true },
          { id: 1, owner: 1, position: { row: 8, col: 7 }, alive: true },
        ],
        cores: [
          { position: { row: 5, col: 5 }, owner: 0, active: true },
          { position: { row: 8, col: 8 }, owner: 1, active: true },
        ],
        energy: [
          { row: 1, col: 2 }, // dist² 1 from Alpha's bot — inside
          { row: 8, col: 6 }, // dist² ≥ 9 (toroidal) — outside
        ],
        scores: [12, 7],
        energy_held: [30, 3],
        events: [],
      },
      {
        turn: 1,
        bots: [
          // Alpha advances next to the enemy: her bot is now at (7,7), so
          // Beta's bot at (8,7) is dist² 1 away, and (1,1) is no longer seen.
          { id: 0, owner: 0, position: { row: 7, col: 7 }, alive: true },
          { id: 1, owner: 1, position: { row: 8, col: 7 }, alive: true },
        ],
        cores: [
          { position: { row: 5, col: 5 }, owner: 0, active: true },
          { position: { row: 8, col: 8 }, owner: 1, active: true },
        ],
        energy: [
          { row: 1, col: 2 },
          { row: 8, col: 6 },
        ],
        scores: [15, 7],
        energy_held: [33, 3],
        events: [],
      },
    ],
  };
}

// ── Recording 2D context ────────────────────────────────────────────────────────

interface RecordedCall {
  method: string;
  args: unknown[];
  /** ctx.fillStyle captured at invocation time (for fillRect/fillText calls). */
  fillStyle: string;
}

const CANVAS_METHODS = [
  'fillRect', 'strokeRect', 'clearRect', 'save', 'restore', 'beginPath',
  'moveTo', 'lineTo', 'closePath', 'stroke', 'fill', 'translate', 'scale',
  'rotate', 'arc', 'ellipse', 'arcTo', 'bezierCurveTo', 'quadraticCurveTo',
  'rect', 'roundRect', 'clip', 'fillText', 'strokeText', 'measureText',
  'transform', 'setTransform', 'resetTransform', 'drawImage',
  'createImageData', 'getImageData', 'putImageData', 'setLineDash',
  'getLineDash',
] as const;

function makeRecordingCtx(): { ctx: any; calls: RecordedCall[] } {
  const calls: RecordedCall[] = [];
  const ctx: any = {
    fillStyle: '',
    strokeStyle: '',
    lineWidth: 1,
    lineCap: 'butt',
    lineJoin: 'miter',
    globalAlpha: 1,
    font: '',
    textAlign: 'left',
    textBaseline: 'alphabetic',
    shadowBlur: 0,
    shadowColor: '',
  };
  for (const method of CANVAS_METHODS) {
    ctx[method] = (...args: unknown[]) => {
      calls.push({ method, args, fillStyle: ctx.fillStyle });
    };
  }
  ctx.measureText = () => ({ width: 0 });
  ctx.getLineDash = () => [];
  const gradient = () => ({ addColorStop: () => {} });
  ctx.createRadialGradient = gradient;
  ctx.createLinearGradient = gradient;
  ctx.createPattern = () => ({});
  return { ctx, calls };
}

/** Frames are delimited manually: every viewer entry point that renders does
 * so synchronously, so a frame is just the calls recorded after a marker. */
type Frame = RecordedCall[];

function fogRectCells(frame: Frame): Array<[number, number]> {
  return frame
    .filter((c) => c.method === 'fillRect' && c.fillStyle === FOG_COLOR)
    .map((c) => [(c.args[1] as number) / CS, (c.args[0] as number) / CS]); // (row, col)
}

/** Distinct fogged cells as "row,col" keys — immune to a cell being fogged
 * by more than one render landing in the same recorded frame. */
function fogCellKeys(frame: Frame): Set<string> {
  return new Set(fogRectCells(frame).map(([r, c]) => `${r},${c}`));
}

function arcCenters(frame: Frame): Array<[number, number]> {
  return frame
    .filter((c) => c.method === 'arc')
    .map((c) => [c.args[0] as number, c.args[1] as number]);
}

/** Pixel center of cell (row, col): x from col, y from row. */
function cellCenter(row: number, col: number): [number, number] {
  return [col * CS + CS / 2, row * CS + CS / 2];
}

function hasArcAt(frame: Frame, x: number, y: number): boolean {
  return arcCenters(frame).some(([ax, ay]) => ax === x && ay === y);
}

function hasArcInCell(frame: Frame, row: number, col: number): boolean {
  const [x, y] = cellCenter(row, col);
  return hasArcAt(frame, x, y);
}

function coreFill(frame: Frame, row: number, col: number): RecordedCall | undefined {
  // drawCore: fillRect(col*CS + CS/2 - 9, row*CS + CS/2 - 9, CS-2, CS-2)
  const x = col * CS + CS / 2 - 9;
  const y = row * CS + CS / 2 - 9;
  return frame.find(
    (c) => c.method === 'fillRect' && c.args[0] === x && c.args[1] === y && c.args[2] === CS - 2,
  );
}

function newViewer(ctx: any, canvas: HTMLCanvasElement): ReplayViewer {
  canvas.getContext = vi.fn(() => ctx) as any;
  // showShapes: false → every bot renders as exactly one arc at its center,
  // regardless of owner, keeping bot assertions shape-independent.
  // reducedMotion: true → no lerp/pulse/spawn scaling, so draw geometry is
  // exactly position*cellSize and threat-line/effect passes are skipped.
  return new ReplayViewer(canvas, {
    cellSize: CS,
    showShapes: false,
    showGrid: false,
    reducedMotion: true,
  });
}

describe('replay fog-of-war perspective toggle (requirements: Replay Visualization)', () => {
  let canvas: HTMLCanvasElement;
  let ctx: any;
  let calls: RecordedCall[];

  beforeEach(() => {
    canvas = document.createElement('canvas');
    const recorded = makeRecordingCtx();
    ctx = recorded.ctx;
    calls = recorded.calls;
  });

  /** Frames since `marker`, for assertions scoped to one render. */
  function frameSince(marker: number): Frame {
    return calls.slice(marker);
  }

  it('defaults to the full-map omniscient view', () => {
    const viewer = newViewer(ctx, canvas);
    const replay = makeReplay();

    // loadReplay renders synchronously — that frame is the page's first paint.
    const marker = calls.length;
    viewer.loadReplay(replay);
    const firstFrame = frameSince(marker);

    expect(viewer.getFogOfWar()).toBeNull();
    expect(fogRectCells(firstFrame)).toEqual([]); // no fog anywhere
    // Both bots drawn — nothing is hidden from the omniscient view.
    expect(hasArcInCell(firstFrame, 1, 1)).toBe(true); // Alpha
    expect(hasArcInCell(firstFrame, 8, 7)).toBe(true); // Beta
  });

  it('toggling back to omniscient clears the fog', () => {
    const viewer = newViewer(ctx, canvas);
    viewer.loadReplay(makeReplay());

    let marker = calls.length;
    viewer.setFogOfWar(0);
    expect(fogRectCells(frameSince(marker)).length).toBeGreaterThan(0);

    marker = calls.length;
    viewer.setFogOfWar(null);
    expect(viewer.getFogOfWar()).toBeNull();
    expect(fogRectCells(frameSince(marker))).toEqual([]);
  });

  it('hides exactly the cells outside the selected player\'s sight radius', () => {
    const viewer = newViewer(ctx, canvas);
    viewer.loadReplay(makeReplay());

    const marker = calls.length;
    viewer.setFogOfWar(0); // synchronous render in Alpha's perspective
    const frame = frameSince(marker);

    const foggedSet = fogCellKeys(frame);
    // 100 cells − 25 within radius² of Alpha's bot − 1 (her own core, which
    // is outside the radius but always visible to her). loadReplay also
    // starts the render loop, whose first tick renders synchronously too —
    // so the frame may hold the same render twice; distinct keys stay exact.
    expect(foggedSet.size).toBe(GRID * GRID - RADIUS_OFFSETS.length - 1);

    const visibleSet = new Set(
      RADIUS_OFFSETS.map(
        ([dr, dc]) => `${(1 + dr + GRID) % GRID},${(1 + dc + GRID) % GRID}`,
      ),
    );

    // Every radius cell is visible, accounting for the toroidal wrap.
    for (const key of visibleSet) expect(foggedSet.has(key)).toBe(false);

    // Boundary: (1,4) is Chebyshev distance 3 but squared distance 9 > 8 — fogged.
    expect(foggedSet.has('1,4')).toBe(true);
    // Boundary: (9,9) wraps to squared distance 8 ≤ 8 — visible.
    expect(foggedSet.has('9,9')).toBe(false);
    // Alpha's own core at (5,5) is outside the radius but always hers to see.
    expect(foggedSet.has('5,5')).toBe(false);
    // A cell adjacent to that core is still fogged: cores grant no vision.
    expect(foggedSet.has('5,4')).toBe(true);
    // Beta's bot cell and her core are deep in the fog.
    expect(foggedSet.has('8,7')).toBe(true);
    expect(foggedSet.has('8,8')).toBe(true);
  });

  it('hides units and structures outside the perspective', () => {
    const viewer = newViewer(ctx, canvas);
    viewer.loadReplay(makeReplay());

    const marker = calls.length;
    viewer.setFogOfWar(0);
    const frame = frameSince(marker);

    // Alpha's own bot is drawn; Beta's bot at (8,7) is not.
    expect(hasArcInCell(frame, 1, 1)).toBe(true);
    expect(hasArcInCell(frame, 8, 7)).toBe(false);

    // Alpha's core (inside her knowledge) is drawn; Beta's core is not.
    expect(coreFill(frame, 5, 5)).toBeDefined();
    expect(coreFill(frame, 8, 8)).toBeUndefined();

    // Energy inside her radius is drawn; the far node is not.
    expect(hasArcInCell(frame, 1, 2)).toBe(true); // (1,2)
    expect(hasArcInCell(frame, 8, 6)).toBe(false); // (8,6)
  });

  it('recomputes visibility per turn as bots move', () => {
    const viewer = newViewer(ctx, canvas);
    viewer.loadReplay(makeReplay());

    const marker0 = calls.length;
    viewer.setFogOfWar(0); // setFogOfWar renders synchronously — deterministic frame
    const frame0 = frameSince(marker0);
    // Turn 0: Beta's bot at (8,7) is far from Alpha's bot at (1,1) — hidden.
    expect(hasArcInCell(frame0, 8, 7)).toBe(false);
    expect(fogCellKeys(frame0)).toContain('8,7');

    // setTurn hands playback to the next turn; the live page's rAF loop then
    // re-renders continuously. In the test, the next setFogOfWar call forces
    // the same synchronous render that loop would produce for turn 1.
    viewer.setTurn(1);
    const marker1 = calls.length;
    viewer.setFogOfWar(0);
    const frame1 = frameSince(marker1);

    // Turn 1: Alpha's bot moved next to Beta's — he is revealed, and the cell
    // Alpha vacated is fogged again (visibility is per turn, not accumulated).
    expect(hasArcInCell(frame1, 8, 7)).toBe(true);
    const fog1 = fogCellKeys(frame1);
    expect(fog1.has('8,7')).toBe(false);
    expect(fog1.has('1,1')).toBe(true);
  });

  it('draws a per-turn score and resource overlay below the map', () => {
    const viewer = newViewer(ctx, canvas);
    const replay = makeReplay();
    viewer.loadReplay(replay);

    const mapBottom = GRID * CS; // y where the playfield ends
    const scoreLines = (frame: Frame) =>
      frame.filter((c) => c.method === 'fillText' && String(c.args[0]).includes('score:'));

    // Turn 0: each player's line carries their score, live bot count, and
    // energy. loadReplay's frame may hold the render twice (the render loop's
    // first tick is synchronous) — distinct texts are what must match 1:1.
    let frame = frameSince(0);
    let lines = scoreLines(frame);
    for (const line of lines) {
      const [, , y] = line.args as [string, number, number];
      expect(y).toBeGreaterThanOrEqual(mapBottom); // below the map, not over it
    }
    let texts = new Set(lines.map((l) => l.args[0]));
    expect(texts).toEqual(new Set([
      'Alpha  score:12  bots:1  energy:30',
      'Beta  score:7  bots:1  energy:3',
    ]));

    // The overlay is per turn: advancing updates both players' numbers.
    viewer.setTurn(1);
    const marker = calls.length;
    viewer.setFogOfWar(null); // force the synchronous turn-1 render
    frame = frameSince(marker);
    lines = scoreLines(frame);
    texts = new Set(lines.map((l) => l.args[0]));
    expect(texts).toEqual(new Set([
      'Alpha  score:15  bots:1  energy:33',
      'Beta  score:7  bots:1  energy:3',
    ]));
  });

  it('clamps playback speed to the supported range', () => {
    const viewer = newViewer(ctx, canvas);
    viewer.loadReplay(makeReplay());

    expect(viewer.getSpeed()).toBe(100); // ships at 100ms/turn
    viewer.setSpeed(300);
    expect(viewer.getSpeed()).toBe(300);
    viewer.setSpeed(5);
    expect(viewer.getSpeed()).toBe(10); // floor
    viewer.setSpeed(5000);
    expect(viewer.getSpeed()).toBe(2000); // ceiling
  });
});

describe('replay page perspective and speed controls (imported markup)', () => {
  const doc = () => new DOMParser().parseFromString(replayPageMarkup(), 'text/html');

  it('ships a fog perspective select defaulting to Omniscient', () => {
    const select = doc().getElementById('fog-select') as HTMLSelectElement | null;
    expect(select).not.toBeNull();
    expect(select!.options.length).toBeGreaterThanOrEqual(1);
    const first = select!.options[0];
    expect(first.value).toBe('');
    expect(first.textContent).toBe('Omniscient');
    // An empty-valued first option is the select's default — the page loads
    // omniscient before any player is chosen.
    expect(select!.value).toBe('');
  });

  it('labels the fog select so the toggle is discoverable', () => {
    const dom = doc();
    const select = dom.getElementById('fog-select');
    const label = dom.querySelector('label[for="fog-select"]');
    expect(select).not.toBeNull();
    expect(label?.textContent).toContain('Fog of War');
  });

  it('ships playback speed controls: slider, presets, and mobile cycle', () => {
    const dom = doc();

    const slider = dom.getElementById('speed-slider') as HTMLInputElement | null;
    expect(slider?.type).toBe('range');
    expect(slider?.min).toBe('20');
    expect(slider?.max).toBe('1000');
    expect(slider?.value).toBe('100');

    const presets = dom.getElementById('speed-select') as HTMLSelectElement | null;
    const values = Array.from(presets?.options ?? []).map((o) => o.value);
    // Multipliers of the 100ms baseline plus the Director mode.
    expect(values).toEqual(['500', '250', '125', '62', '31', 'director']);

    expect(dom.getElementById('mobile-speed-btn')).not.toBeNull();
  });
});
