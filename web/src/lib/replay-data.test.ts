/**
 * Decode-parity tests for the replay serving pipeline.
 *
 * Replays are stored gzipped (worker uploads replays/<id>.json.gz) and served
 * three ways depending on origin: Pages serves the .json.gz bytes verbatim
 * with no Content-Encoding (the client gunzips), a server may set
 * Content-Encoding: gzip (fetch decompresses transparently), and demo
 * replays ship as plain .json. Whatever the serving mode, the decoded result
 * must be identical — these tests pin that parity with real gzip bytes.
 *
 * The loader is the decode half of the storage format: the worker stores the
 * v2.1 delta-encoded form (scores/energy_held omitted when unchanged), so
 * fetchReplayFromUrl fills those forward before returning — every assertion
 * below compares against the reconstructed form of the fixture, and one test
 * pins the fill-forward itself.
 */

import { describe, it, expect, vi, afterEach } from 'vitest';
import { gzipSync } from 'node:zlib';
import {
  fetchReplayFromUrl,
  reconstructReplay,
  replayUrl,
  REPLAY_BASE,
} from './replay-data';
import type { Replay, ReplayTurn } from '../types';

// A realistic small replay: 2 players, 3 turns, delta-encoded scores and
// energy_held (omitted when unchanged, per format v2.1).
function fixtureReplay(): Replay {
  const turn = (n: number, extra: Partial<ReplayTurn>): ReplayTurn => ({
    turn: n,
    bots: [
      { id: 0, owner: 0, position: { row: 3 + n, col: 4 }, alive: true },
      { id: 1, owner: 1, position: { row: 30, col: 35 - n }, alive: true },
    ],
    cores: [
      { position: { row: 2, col: 2 }, owner: 0, active: true },
      { position: { row: 32, col: 32 }, owner: 1, active: true },
    ],
    energy: [{ row: 10, col: 10 }, { row: 20, col: 20 }],
    events: n === 1 ? [{ type: 'bot_spawned', turn: 1, player: 0, details: {} }] : [],
    ...extra,
  });
  return {
    format_version: '2.1',
    match_id: 'm_parity_test',
    config: { rows: 40, cols: 40, max_turns: 500 } as Replay['config'],
    start_time: '2026-09-16T00:00:00Z',
    end_time: '2026-09-16T00:01:00Z',
    result: { winner: 0, reason: 'elimination', turns: 3, scores: [5, 1] } as Replay['result'],
    players: [
      { id: 0, name: 'Parity A' },
      { id: 1, name: 'Parity B' },
    ],
    map: { rows: 40, cols: 40, walls: [], cores: [], energy_nodes: [] } as Replay['map'],
    turns: [
      turn(0, { scores: [0, 0], energy_held: [1, 1] }),
      turn(1, { scores: [2, 0] }), // energy_held omitted: unchanged
      turn(2), // both omitted: unchanged
    ],
  };
}

function jsonResponse(body: string | Uint8Array, headers: Record<string, string> = {}): Response {
  return new Response(body as BodyInit, { status: 200, headers });
}

// What the loader must return for the fixture: the v2.1 fill-forward applied
// (turn 1's omitted energy_held and turn 2's omitted scores/energy_held
// carried from the previous turn). fixtureReplay() builds a fresh object per
// call, so reconstructing this copy in place cannot alias the fixtures used
// as fetch bodies.
function decodedFixture(): Replay {
  return reconstructReplay(fixtureReplay());
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('fetchReplayFromUrl decode parity', () => {
  it('decodes verbatim .json.gz bytes (Pages serving, no Content-Encoding)', async () => {
    const replay = fixtureReplay();
    const gz = gzipSync(Buffer.from(JSON.stringify(replay)));
    // Pages serves .json.gz assets byte-for-byte with no content-encoding header.
    const resp = jsonResponse(gz, { 'content-type': 'application/gzip' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(resp));

    const decoded = await fetchReplayFromUrl(`${REPLAY_BASE}/m_parity_test.json.gz`);
    expect(decoded).toEqual(decodedFixture());
  });

  it('decodes a .gz URL served with Content-Encoding: gzip without double-decompressing', async () => {
    const replay = fixtureReplay();
    // If the server sets Content-Encoding, fetch() already decompressed the
    // body — handing plain bytes. The loader must not gunzip a second time.
    const resp = jsonResponse(JSON.stringify(replay), { 'content-encoding': 'gzip' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(resp));

    const decoded = await fetchReplayFromUrl(`${REPLAY_BASE}/m_parity_test.json.gz`);
    expect(decoded).toEqual(decodedFixture());
  });

  it('still gunzips when Content-Encoding: identity is set (raw .gz bytes)', async () => {
    const replay = fixtureReplay();
    // identity means "no transformation": the body is still the stored gzip
    // bytes, so the manual DecompressionStream pass must run.
    const gz = gzipSync(Buffer.from(JSON.stringify(replay)));
    const resp = jsonResponse(gz, { 'content-encoding': 'identity' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(resp));

    const decoded = await fetchReplayFromUrl(`${REPLAY_BASE}/m_parity_test.json.gz`);
    expect(decoded).toEqual(decodedFixture());
  });

  it('decodes plain .json (demo replays) identically', async () => {
    const replay = fixtureReplay();
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(JSON.stringify(replay))));

    const decoded = await fetchReplayFromUrl('/data/demo-replay-v2.json');
    expect(decoded).toEqual(decodedFixture());
  });

  it('all serving modes decode to the identical object', async () => {
    const replay = fixtureReplay();
    const gz = gzipSync(Buffer.from(JSON.stringify(replay)));
    const cases: Array<[string, Response]> = [
      [`${REPLAY_BASE}/m.json.gz`, jsonResponse(gz)],
      [`${REPLAY_BASE}/m.json.gz`, jsonResponse(JSON.stringify(replay), { 'content-encoding': 'gzip' })],
      [`${REPLAY_BASE}/m.json.gz`, jsonResponse(gz, { 'content-encoding': 'identity' })],
      ['/data/demo.json', jsonResponse(JSON.stringify(replay))],
    ];
    for (const [url, resp] of cases) {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue(resp));
      await expect(fetchReplayFromUrl(url)).resolves.toEqual(decodedFixture());
    }
  });

  it('fills delta-omitted scores/energy_held forward (decode half of v2.1 storage)', async () => {
    const replay = fixtureReplay();
    // Stored form: turn 1 omits energy_held, turn 2 omits both — the omissions
    // the engine writes whenever a value is unchanged since the prior turn.
    expect(replay.turns[1].energy_held).toBeUndefined();
    expect(replay.turns[2].scores).toBeUndefined();
    expect(replay.turns[2].energy_held).toBeUndefined();

    const gz = gzipSync(Buffer.from(JSON.stringify(replay)));
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(gz)));

    // Consumers (embed score overlay, win probability) read full per-turn
    // values off the loaded replay and must never see the stored omissions
    // as zeros.
    const decoded = await fetchReplayFromUrl(`${REPLAY_BASE}/m_parity_test.json.gz`);
    expect(decoded.turns[1].energy_held).toEqual([1, 1]);
    expect(decoded.turns[2].scores).toEqual([2, 0]);
    expect(decoded.turns[2].energy_held).toEqual([1, 1]);
  });

  it('throws HTTP <status> on a non-OK response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('Not Found', { status: 404 })));
    await expect(fetchReplayFromUrl(`${REPLAY_BASE}/missing.json.gz`)).rejects.toThrow('HTTP 404');
  });
});

describe('replayUrl contract', () => {
  it('keeps the /data/replays/<id>.json.gz URL shape (backward compatibility)', () => {
    expect(replayUrl('m_7f3a9b2c')).toBe('/data/replays/m_7f3a9b2c.json.gz');
  });
});

describe('reconstructReplay delta fill-forward', () => {
  it('fills forward omitted scores and energy_held for v2.1', () => {
    const replay = fixtureReplay();
    const out = reconstructReplay(replay);
    // Turn 1 omitted energy_held: carries turn 0's values forward.
    expect(out.turns[1].energy_held).toEqual([1, 1]);
    // Turn 2 omitted both: carries turn 1's values forward.
    expect(out.turns[2].scores).toEqual([2, 0]);
    expect(out.turns[2].energy_held).toEqual([1, 1]);
  });

  it('does not alias filled arrays across turns', () => {
    const replay = fixtureReplay();
    const out = reconstructReplay(replay);
    expect(out.turns[1].energy_held).not.toBe(out.turns[0].energy_held);
    expect(out.turns[2].scores).not.toBe(out.turns[1].scores);
  });

  it('initializes omitted first-turn values to zeros', () => {
    const replay = fixtureReplay();
    delete replay.turns[0].scores;
    delete replay.turns[0].energy_held;
    const out = reconstructReplay(replay);
    expect(out.turns[0].scores).toEqual([0, 0]);
    expect(out.turns[0].energy_held).toEqual([0, 0]);
  });

  it('leaves pre-2.1 replays untouched', () => {
    const replay = fixtureReplay();
    replay.format_version = '2.0';
    delete replay.turns[2].scores;
    const out = reconstructReplay(replay);
    // v2.0 semantics: all turns carry full data; nothing is filled in.
    expect(out.turns[2].scores).toBeUndefined();
  });

  it('passes changed values through unchanged', () => {
    const replay = fixtureReplay();
    replay.turns[1].scores = [3, 1];
    const out = reconstructReplay(replay);
    expect(out.turns[1].scores).toEqual([3, 1]);
  });
});
