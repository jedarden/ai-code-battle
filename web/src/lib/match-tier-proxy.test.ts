import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { handleApiRequest, type ApiEnv, type CommunityBucket } from './api-backend';

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

const originalFetch = globalThis.fetch;
let upstream: ReturnType<typeof vi.fn>;
let env: ApiEnv;

function request(path: string, init: RequestInit = {}): Request {
  return new Request(`https://ai-code-battle.pages.dev/api${path}`, init);
}

async function call(path: string, init: RequestInit = {}) {
  const response = await handleApiRequest(request(path, init), env, path.split('?')[0]);
  return {
    response,
    body: await response.json() as Record<string, unknown>,
  };
}

beforeEach(() => {
  env = {
    ACB_BUCKET: new MemBucket(),
    ACB_API_ORIGIN: 'https://api.example.test',
  };
  upstream = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    if (url.endsWith('/ready')) {
      return Response.json({ status: 'ready' }, { status: 200 });
    }
    return Response.json({ ok: true, path: new URL(url).pathname, method: init?.method });
  });
  globalThis.fetch = upstream as unknown as typeof fetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  vi.restoreAllMocks();
});

describe('match-tier Pages proxy', () => {
  it('proxies registration validation and preserves JSON status', async () => {
    upstream.mockImplementationOnce(async () => Response.json(
      { error: 'name, owner, and endpoint_url are required' },
      { status: 400 },
    ));

    const result = await call('/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'CF-Connecting-IP': '198.51.100.4' },
      body: '{}',
    });

    expect(result.response.status).toBe(400);
    expect(result.response.headers.get('content-type')).toContain('application/json');
    expect(result.body.error).toContain('owner');
    expect(upstream).toHaveBeenCalledTimes(1);
    const [url, init] = upstream.mock.calls[0] as [string, RequestInit];
    expect(String(url)).toBe('https://api.example.test/api/register');
    expect(init.method).toBe('POST');
    expect((init.headers as Headers).get('x-forwarded-for')).toBe('198.51.100.4');
  });

  it('delivers the registration secret without transforming the backend response', async () => {
    upstream.mockImplementationOnce(async () => Response.json(
      { bot_id: 'b_0123456789ab', shared_secret: 'a'.repeat(64) },
      { status: 201 },
    ));

    const result = await call('/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'bot', owner: 'owner', endpoint_url: 'https://bot.test' }),
    });

    expect(result.response.status).toBe(201);
    expect(result.body).toMatchObject({ bot_id: 'b_0123456789ab', shared_secret: 'a'.repeat(64) });
  });

  it.each([
    ['/rotate-key', 200, { bot_id: 'b_1', shared_secret: 'new-secret' }],
    ['/revoke-key', 200, { bot_id: 'b_1', status: 'retired' }],
    ['/predict', 201, { id: 7, match_id: 'm_1', predicted: 'b_1', predictor: 'fan_1' }],
    ['/predictions/open?predictor_id=fan_1', 200, { matches: [] }],
    ['/predictions/history?predictor_id=fan_1', 200, { predictions: [] }],
  ])('proxies %s responses', async (path, status, payload) => {
    upstream.mockImplementationOnce(async () => Response.json(payload, { status }));
    const result = await call(path, {
      method: path.startsWith('/predictions/') ? 'GET' : 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: path.startsWith('/predictions/') ? undefined : '{}',
    });
    expect(result.response.status).toBe(status);
    expect(result.body).toEqual(payload);
  });

  it('publishes match-tier capabilities only after the readiness probe is healthy', async () => {
    const ready = await call('/health');
    expect(ready.body).toMatchObject({
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

    upstream.mockImplementation(async (input: RequestInfo | URL) => {
      if (String(input).endsWith('/ready')) return Response.json({ error: 'not ready' }, { status: 503 });
      return Response.json({ ok: true });
    });
    const notReady = await call('/health');
    expect(notReady.body).toMatchObject({
      capabilities: { register: false, rotate_key: false, revoke_key: false, predictions: false },
    });
  });

  it('converts an upstream HTML error into JSON instead of leaking the SPA seam', async () => {
    upstream.mockImplementationOnce(async () => new Response('<html>gateway error</html>', {
      status: 502,
      headers: { 'Content-Type': 'text/html' },
    }));

    const result = await call('/predict', { method: 'POST', body: '{}' });
    expect(result.response.status).toBe(503);
    expect(result.response.headers.get('content-type')).toContain('application/json');
    expect(result.body.code).toBe('match_tier_unavailable');
  });

  it('keeps the explicit offline JSON contract when no backend origin is configured', async () => {
    env = { ACB_BUCKET: new MemBucket() };
    const result = await call('/rotate-key', { method: 'POST', body: '{}' });
    expect(result.response.status).toBe(503);
    expect(result.body.code).toBe('match_tier_offline');
    expect(upstream).not.toHaveBeenCalled();
  });
});
