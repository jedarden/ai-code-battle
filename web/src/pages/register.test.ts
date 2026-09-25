/**
 * Register page disabled-state contract (bead aicodeba-07caa4ec).
 *
 * The register form posts to same-origin `/api/register`, which the Pages
 * SPA fallback answers with HTML — the API has no public endpoint yet. With
 * the transport disabled the page must say so up front, render every control
 * inert, and refuse submits before any request is attempted.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderRegisterPage } from './register';
import { API_TRANSPORT_ENABLED } from '../lib/api-transport';

describe('renderRegisterPage with no API transport', () => {
  beforeEach(() => {
    document.body.innerHTML = '<div id="app"></div>';
    globalThis.fetch = vi.fn(() => {
      throw new Error('fetch must not be called while API_TRANSPORT_ENABLED is false');
    });
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('is reachable only because the transport is disabled — pins the premise', () => {
    expect(API_TRANSPORT_ENABLED).toBe(false);
  });

  it('renders the unavailable notice explaining why the form is disabled', () => {
    renderRegisterPage();

    const notice = document.getElementById('register-unavailable-notice');
    expect(notice).not.toBeNull();
    expect(notice!.textContent).toContain('Bot registration is unavailable right now');
    expect(notice!.textContent).toContain('registration API has no public endpoint');
    // Points at where the request contract actually lives now.
    expect(notice!.querySelector('a')?.getAttribute('href')).toBe('#/compete/docs');
  });

  it('disables every input and the submit button', () => {
    renderRegisterPage();

    const form = document.getElementById('register-form') as HTMLFormElement;
    expect(form).not.toBeNull();

    const controls = Array.from(form.querySelectorAll('input, button'));
    expect(controls.length).toBeGreaterThan(0);
    for (const control of controls) {
      expect((control as HTMLInputElement).disabled).toBe(true);
    }

    const submit = form.querySelector('button[type="submit"]') as HTMLButtonElement;
    expect(submit.textContent?.trim()).toBe('Registration Unavailable');
  });

  it('refuses form submits without attempting any request', async () => {
    renderRegisterPage();

    const form = document.getElementById('register-form') as HTMLFormElement;
    form.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }));
    // Let any stray async handler settle before asserting.
    await new Promise(resolve => setTimeout(resolve, 25));

    expect(globalThis.fetch).not.toHaveBeenCalled();
    // The page must not have flipped into its success/error states either.
    expect(document.querySelector('.register-success')).toBeNull();
    expect(document.querySelector('.error-message')).toBeNull();
  });
});
