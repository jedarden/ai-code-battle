/**
 * Through-transport match-tier coverage.
 *
 * match-tier-proxy.test.ts exercises the proxy helper directly. These tests
 * drive the exported Pages `onRequest` adapter with `/api/*` URLs and a
 * configured upstream, which covers the path strip, environment hand-off,
 * readiness response, and one-time credential response at the transport seam
 * the deployed runtime actually invokes.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { onRequest } from '../../functions/api/[[path]]';
import type { ApiEnv, CommunityBucket } from './api-backend';

class MemBucket implements CommunityBucket {
  async get(): Promise<{ etag: string; text(): Promise<string> } | null> {
    return null;
  }

  async head(): Promise<{ etag: string } | null> {
    return null;
  }

  async put(): Promise<unknown | null> {
    return {};
  }
}

let env: ApiEnv;
let upstream: ReturnType<typeof vi.fn>;

function pagesContext(request: Request): Parameters<typeof onRequest>[0] {
  return {
    request,
    env,
    params: {},
    data: {},
    functionPath: '/api',
    next: () => {
      throw new Error('next() must not be called by the /api adapter');
    },
    waitUntil: () => {},
    passThroughOnException: () => {},
  };
}

async function call(path: string, init: RequestInit = {}): Promise<Response> {
  const request = new Request(`https://ai-code-battle.pages.dev${path}`, init);
  return onRequest(pagesContext(request));
}

beforeEach(() => {
  env = {
    ACB_BUCKET: new MemBucket(),
    ACB_API_ORIGIN: 'https://api.example.test',
  };
  upstream = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input));
    if (url.pathname === '/ready') return Response.json({ status: 'ready' });
    return Response.json({ ok: true, method: init?.method, path: url.pathname });
  });
  vi.stubGlobal('fetch', upstream);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('configured match-tier transport through the Pages adapter', () => {
  it('forwards registration and preserves the one-time secret response', async () => {
    upstream.mockImplementationOnce(async (_input: RequestInfo | URL, init?: RequestInit) => {
      const body = await new Response(init?.body).text();
      expect(init?.method).toBe('POST');
      expect(body).toContain('"name":"transport-bot"');
      expect((init?.headers as Headers).get('x-forwarded-for')).toBe('198.51.100.25');
      return Response.json(
        { bot_id: 'b_0123456789ab', shared_secret: 'a'.repeat(64) },
        { status: 201 },
      );
    });

    const response = await call('/api/register', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'CF-Connecting-IP': '198.51.100.25',
      },
      body: JSON.stringify({
        name: 'transport-bot',
        owner: 'transport-test',
        endpoint_url: 'https://bot.example.test',
      }),
    });

    expect(response.status).toBe(201);
    expect(response.headers.get('content-type')).toContain('application/json');
    expect(response.headers.get('cache-control')).toBe('no-store');
    await expect(response.json()).resolves.toEqual({
      bot_id: 'b_0123456789ab',
      shared_secret: 'a'.repeat(64),
    });
    expect(upstream).toHaveBeenCalledTimes(1);
    expect(String(upstream.mock.calls[0][0])).toBe('https://api.example.test/api/register');
  });

  it.each([
    ['/api/rotate-key', 'POST', 200, { bot_id: 'b_1', shared_secret: 'new' }],
    ['/api/revoke-key', 'POST', 200, { bot_id: 'b_1', status: 'retired' }],
    ['/api/predict', 'POST', 201, { id: 7, match_id: 'm_1' }],
    ['/api/predictions/open?predictor_id=fan_1', 'GET', 200, { matches: [] }],
    ['/api/predictions/history?predictor_id=fan_1', 'GET', 200, { predictions: [] }],
  ])('forwards %s through the adapter', async (path, method, status, payload) => {
    upstream.mockImplementationOnce(async () => Response.json(payload, { status }));
    const response = await call(path, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: method === 'POST' ? '{}' : undefined,
    });

    expect(response.status).toBe(status);
    await expect(response.json()).resolves.toEqual(payload);
    expect(String(upstream.mock.calls[0][0])).toContain('/api/');
  });

  it.each([
    ['/api/register', 400, { error: 'name, owner, and endpoint_url are required' }],
    ['/api/register', 400, { error: 'bot endpoint validation failed' }],
    ['/api/register', 409, { error: 'name already taken' }],
    ['/api/rotate-key', 401, { error: 'invalid shared_secret' }],
    ['/api/revoke-key', 401, { error: 'invalid shared_secret' }],
  ])('preserves %s validation failures as JSON', async (path, status, payload) => {
    upstream.mockImplementationOnce(async () => Response.json(payload, { status }));

    const response = await call(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{}',
    });

    expect(response.status).toBe(status);
    expect(response.headers.get('content-type')).toContain('application/json');
    expect(response.headers.get('content-type')).not.toContain('text/html');
    await expect(response.json()).resolves.toEqual(payload);
  });

  it.each(['/api/register', '/api/rotate-key', '/api/revoke-key'])(
    'converts an HTML failure for %s into a JSON API error',
    async (path) => {
      upstream.mockImplementationOnce(async () => new Response('<html>gateway error</html>', {
        status: 502,
        headers: { 'Content-Type': 'text/html' },
      }));

      const response = await call(path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      });

      expect(response.status).toBe(503);
      expect(response.headers.get('content-type')).toContain('application/json');
      const body = await response.json();
      expect(body).toMatchObject({ code: 'match_tier_unavailable' });
      expect(JSON.stringify(body)).not.toContain('<html>');
    },
  );

  it('publishes live capabilities only after the upstream readiness probe', async () => {
    const response = await call('/api/health');

    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      status: 'ok',
      capabilities: {
        register: true,
        rotate_key: true,
        revoke_key: true,
        predictions: true,
        feedback: true,
        map_votes: true,
      },
    });
    expect(String(upstream.mock.calls[0][0])).toBe('https://api.example.test/ready');
  });

  it('normalizes an unavailable upstream to a JSON 503', async () => {
    upstream.mockRejectedValueOnce(new Error('connection refused'));
    const response = await call('/api/predict', { method: 'POST', body: '{}' });

    expect(response.status).toBe(503);
    expect(response.headers.get('content-type')).toContain('application/json');
    await expect(response.json()).resolves.toMatchObject({ code: 'match_tier_unavailable' });
  });
});
