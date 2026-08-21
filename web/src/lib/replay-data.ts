// Replay data access.
//
// Replays are served as gzipped static assets under /data/replays/ on Cloudflare
// Pages: B2 is the private cold archive, and the index-builder bundles the warm
// set into the Pages deploy (cmd/acb-index-builder bundleWarmReplays). Because
// Pages serves the .json.gz bytes verbatim (no Content-Encoding), the browser
// must gunzip them with DecompressionStream.
import type { Replay } from '../types';

/** Base path for replay assets bundled into the Pages deploy. */
export const REPLAY_BASE = '/data/replays';

/** Same-origin URL for a replay by match ID. */
export function replayUrl(matchId: string): string {
  return `${REPLAY_BASE}/${matchId}.json.gz`;
}

/**
 * Fetch and parse a replay from a URL, transparently gunzipping .gz responses.
 * Throws `Error("HTTP <status>")` on a non-OK response so callers can branch on 404.
 */
export async function fetchReplayFromUrl(url: string): Promise<Replay> {
  const resp = await fetch(url);
  if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
  if (url.endsWith('.gz') && resp.body && typeof DecompressionStream !== 'undefined') {
    const stream = resp.body.pipeThrough(new DecompressionStream('gzip'));
    return JSON.parse(await new Response(stream).text()) as Replay;
  }
  return (await resp.json()) as Replay;
}

/** Fetch a replay by match ID from the bundled Pages assets. */
export function fetchReplay(matchId: string): Promise<Replay> {
  return fetchReplayFromUrl(replayUrl(matchId));
}

/**
 * Reconstructs delta-encoded replay data by filling forward omitted scores/energy_held.
 * In format version 2.1+, scores and energy_held use delta encoding: if omitted,
 * the viewer uses the previous turn's values (fill-forward).
 *
 * This function mutates the replay in place and returns it for convenience.
 */
export function reconstructReplay(replay: Replay): Replay {
  const version = replay.format_version;

  // Only reconstruct for v2.1+ (delta encoding was added in 2.1)
  // For v2.0 and earlier, all turns have full data (no reconstruction needed)
  const needsReconstruction = version === '2.1' ||
                            (version && parseFloat(version) >= 2.1);

  if (!needsReconstruction) {
    return replay;
  }

  let lastScores: number[] | null = null;
  let lastEnergy: number[] | null = null;
  const numPlayers = replay.players.length;

  for (const turn of replay.turns) {
    // Fill-forward scores if omitted (delta encoding)
    if (!turn.scores || turn.scores.length === 0) {
      if (lastScores) {
        turn.scores = [...lastScores]; // Clone to avoid mutation
      } else {
        // First turn: initialize to zeros
        turn.scores = new Array(numPlayers).fill(0);
      }
    } else {
      lastScores = [...turn.scores]; // Update last seen values
    }

    // Fill-forward energy_held if omitted (delta encoding)
    if (!turn.energy_held || turn.energy_held.length === 0) {
      if (lastEnergy) {
        turn.energy_held = [...lastEnergy]; // Clone to avoid mutation
      } else {
        // First turn: initialize to zeros
        turn.energy_held = new Array(numPlayers).fill(0);
      }
    } else {
      lastEnergy = [...turn.energy_held]; // Update last seen values
    }
  }

  return replay;
}
