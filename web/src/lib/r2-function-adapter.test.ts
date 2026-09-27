/**
 * @vitest-environment node
 *
 * Contract tests for the deployed /r2 Pages Function adapter itself
 * (bead aicodeba-a0c15607).
 *
 * The /r2 route is the replay bucket's seam on the Pages origin, and the
 * transport contract needs it isolated from /api: its catch-all
 * (functions/r2/[[path]].ts) is what keeps /r2/* away from the SPA fallback,
 * and the two function mounts must never answer each other's paths. These
 * tests drive that exported onRequest end-to-end the way the Pages runtime
 * would (a full https Request in, its Response out) against an in-memory R2
 * bucket, and pin:
 *
 *   - the mount boundary: /r2/api/health is a bucket key lookup, never the
 *     /api function's JSON health envelope — the catch-alls do not cross-serve
 *   - the seam's answer shape while the bucket is empty: 404 text/plain.
 *     text/plain is the function's signature; text/html would be the SPA
 *     fallback, i.e. the r2 catch-all is missing from the deploy
 *   - the bare /r2/ root (empty key) refuses without touching the bucket
 *   - a deploy whose binding is absent answers 503 text/plain, loudly
 *   - a stored object round-trips with its R2 metadata (content type,
 *     cache-control, CORS), and a .gz object is decompressed inside the
 *     worker — the CDN strips Content-Encoding from worker responses, so the
 *     body must arrive plain (the in-file comment on functions/r2/[[path]].ts)
 *
 * Runs under the node environment (not the repo's jsdom default) because the
 * .gz contract streams through DecompressionStream, which the browser
 * environment does not provide; nothing here touches the DOM. The deployed
 * routing layer itself — the platform handing /r2/* to this file — is pinned
 * live by part E of web/test-api-workflows.js, the one seam no in-process
 * test sees.
 */

import { describe, it, expect } from 'vitest';
import { onRequest } from '../../functions/r2/[[path]]';

type R2Context = Parameters<typeof onRequest>[0];
type R2Env = R2Context['env'];

/** In-memory stand-in for the R2 bucket binding, with get() observation. */
class MemR2Bucket {
  /** Every key a probe caused the adapter to look up. */
  getRequestKeys: string[] = [];

  private objects = new Map<string, { bytes: Uint8Array; contentType: string }>();

  put(key: string, bytes: Uint8Array, contentType: string) {
    this.objects.set(key, { bytes, contentType });
  }

  async get(key: string) {
    this.getRequestKeys.push(key);
    const obj = this.objects.get(key);
    if (!obj) return null;
    return {
      body: new Response(obj.bytes).body as ReadableStream,
      writeHttpMetadata(headers: Headers) {
        headers.set('Content-Type', obj.contentType);
      },
    };
  }
}

/** Text as R2 would store it. */
function bytes(text: string): Uint8Array {
  return new TextEncoder().encode(text);
}

/** Text actually gzipped — a `.gz` R2 object holds compressed bytes. */
async function gzBytes(text: string): Promise<Uint8Array> {
  const stream = new Blob([text]).stream().pipeThrough(new CompressionStream('gzip'));
  return new Uint8Array(await new Response(stream).arrayBuffer());
}

/** The slice of the Pages event context the adapter actually touches. */
function pagesContext(request: Request, env: R2Env): R2Context {
  return {
    request,
    env,
    params: {},
    data: {},
    functionPath: '/r2',
    next: () => {
      throw new Error('next() must not be called by the /r2 adapter');
    },
    waitUntil: () => {},
    passThroughOnException: () => {},
  };
}

async function call(path: string, env: R2Env) {
  const request = new Request(`https://ai-code-battle.pages.dev${path}`);
  const response = await onRequest(pagesContext(request, env));
  return {
    status: response.status,
    contentType: response.headers.get('content-type') ?? '',
    headers: response.headers,
    text: await response.text(),
  };
}

describe('the /r2 and /api mounts do not cross-serve', () => {
  it('/r2/api/health is a bucket key lookup, never the /api health envelope', async () => {
    const res = await call('/r2/api/health', { ACB_BUCKET: new MemR2Bucket() });
    expect(res.status).toBe(404);
    expect(res.text).toBe('Not Found');
    expect(res.contentType).toContain('text/plain');
    expect(res.contentType).not.toContain('application/json');
  });

  it('an api-shaped path is looked up as a literal key, not re-routed', async () => {
    const bucket = new MemR2Bucket();
    await call('/r2/api/health', { ACB_BUCKET: bucket });
    // The mount boundary is the key namespace: the adapter asked the bucket
    // for "api/health" — the /api prefix stripped only the route's own /r2.
    expect(bucket.getRequestKeys).toEqual(['api/health']);
  });
});

describe('the seam answers text/plain while the bucket is empty', () => {
  it('a missing key answers 404 text/plain "Not Found"', async () => {
    const res = await call('/r2/replays/route-isolation-probe-a0c15607.json.gz', {
      ACB_BUCKET: new MemR2Bucket(),
    });
    expect(res.status).toBe(404);
    expect(res.text).toBe('Not Found');
    expect(res.contentType).toContain('text/plain');
    expect(res.contentType).not.toContain('text/html');
  });

  it('the bare /r2/ root refuses without touching the bucket', async () => {
    const bucket = new MemR2Bucket();
    const res = await call('/r2/', { ACB_BUCKET: bucket });
    expect(res.status).toBe(404);
    expect(res.text).toBe('Not Found');
    expect(res.contentType).toContain('text/plain');
    // The empty key short-circuits before the binding is consulted — the
    // same probe must stay 404 text/plain even on a deploy with no bucket.
    expect(bucket.getRequestKeys).toEqual([]);
  });

  it('a deploy with no R2 binding answers 503 text/plain, not a crash', async () => {
    const res = await call('/r2/replays/whatever.json.gz', {} as R2Env);
    expect(res.status).toBe(503);
    expect(res.text).toBe('R2 binding not configured');
    expect(res.contentType).toContain('text/plain');
  });
});

describe('a stored object round-trips through the worker', () => {
  it('serves the object body with its R2 metadata, cache-control, and CORS', async () => {
    const bucket = new MemR2Bucket();
    bucket.put('community/map-votes.json', bytes('{"map-1":{"voter-a":1}}'), 'application/json');
    const res = await call('/r2/community/map-votes.json', { ACB_BUCKET: bucket });
    expect(res.status).toBe(200);
    expect(res.contentType).toContain('application/json');
    expect(res.text).toBe('{"map-1":{"voter-a":1}}');
    expect(res.headers.get('cache-control')).toBe('public, max-age=60');
    expect(res.headers.get('access-control-allow-origin')).toBe('*');
  });

  it('decompresses a .gz object in the worker — the body arrives plain', async () => {
    const bucket = new MemR2Bucket();
    const replay = JSON.stringify({ id: 'm1', turns: [{ t: 0 }] });
    bucket.put('replays/m1.json.gz', await gzBytes(replay), 'application/json');
    const res = await call('/r2/replays/m1.json.gz', { ACB_BUCKET: bucket });
    expect(res.status).toBe(200);
    // The CDN strips Content-Encoding: gzip from worker responses, so the
    // adapter decompresses instead — the client must receive plain JSON with
    // no Content-Encoding to undo.
    expect(res.text).toBe(replay);
    expect(res.headers.get('content-encoding')).toBeNull();
  });
});
