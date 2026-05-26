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
