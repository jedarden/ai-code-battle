/**
 * Register page contract with the transport live (bead aicodeba-84d1d61b).
 *
 * Registration is a match-tier flow: acb-api is not deployed, so the
 * function answers POST /api/register with 503 JSON (code
 * match_tier_offline). The form is therefore fully live and interactive —
 * the compile-time kill switch is off — and the page surfaces the server's
 * answer honestly, whether that is the offline notice or, if the transport
 * ever regresses to the SPA HTML fallback, a non-JSON failure.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { API_TRANSPORT_ENABLED } from '../lib/api-transport';

// register.ts keeps module-level form state; a fresh import per test keeps
// a previous test's success view from leaking into the next one.
let renderRegisterPage: typeof import('./register')['renderRegisterPage'];

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => body,
  } as unknown as Response;
}

async function fillAndSubmit(): Promise<void> {
  (document.getElementById('bot-name') as HTMLInputElement).value = 'TestBot';
  (document.getElementById('endpoint-url') as HTMLInputElement).value = 'https://bot.example.com/move';
  (document.getElementById('owner-id') as HTMLInputElement).value = 'owner@example.com';
  (document.getElementById('register-form') as HTMLFormElement)
    .dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }));
  // Let the async submit handler settle before asserting.
  await new Promise(resolve => setTimeout(resolve, 25));
}

async function waitFor(what: string, condition: () => boolean): Promise<void> {
  const deadline = Date.now() + 2000;
  while (!condition()) {
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${what}`);
    await new Promise(resolve => setTimeout(resolve, 10));
  }
}

describe('renderRegisterPage with the transport live', () => {
  beforeEach(async () => {
    vi.resetModules();
    ({ renderRegisterPage } = await import('./register'));
    document.body.innerHTML = '<div id="app"></div>';
    expect(API_TRANSPORT_ENABLED).toBe(true);
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('renders a fully enabled form — no unavailable notice', () => {
    renderRegisterPage();

    expect(document.getElementById('register-unavailable-notice')).toBeNull();

    const form = document.getElementById('register-form') as HTMLFormElement;
    expect(form).not.toBeNull();
    const controls = Array.from(form.querySelectorAll('input, button'));
    expect(controls.length).toBeGreaterThan(0);
    for (const control of controls) {
      expect((control as HTMLInputElement).disabled).toBe(false);
    }
    const submit = form.querySelector('button[type="submit"]') as HTMLButtonElement;
    expect(submit.textContent?.trim()).toBe('Register Bot');
  });

  it('surfaces the server match-tier offline answer on submit', async () => {
    globalThis.fetch = vi.fn(async () =>
      jsonResponse(
        {
          error: 'Bot registration is offline: the compute tier that runs matches is not deployed.',
          code: 'match_tier_offline',
        },
        503,
      ),
    );
    renderRegisterPage();

    await fillAndSubmit();
    await waitFor('the server error to render', () => document.querySelector('.error-message') !== null);

    expect(document.querySelector('.error-message')!.textContent).toContain('registration is offline');
    expect(document.querySelector('.register-success')).toBeNull();
  });

  it('shows a success-shaped answer as success', async () => {
    globalThis.fetch = vi.fn(async () =>
      jsonResponse({ success: true, bot_id: 'bot-123', api_key: 'acb_key_1' }),
    );
    renderRegisterPage();

    await fillAndSubmit();
    await waitFor('the success view', () => document.querySelector('.register-success') !== null);

    expect(document.querySelector('.api-key')!.textContent).toBe('acb_key_1');
  });

  it('degrades honestly if /api ever regresses to the SPA HTML fallback', async () => {
    globalThis.fetch = vi.fn(async () =>
      ({
        ok: true,
        status: 200,
        headers: new Headers({ 'content-type': 'text/html' }),
        json: async () => {
          throw new Error('must not JSON-parse HTML');
        },
      }) as unknown as Response,
    );
    renderRegisterPage();

    await fillAndSubmit();
    await waitFor('the non-JSON error to render', () => document.querySelector('.error-message') !== null);

    expect(document.querySelector('.error-message')!.textContent).toContain('non-JSON');
    expect(document.querySelector('.register-success')).toBeNull();
  });
});
