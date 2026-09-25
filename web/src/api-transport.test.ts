/**
 * Transport-disable contract (bead aicodeba-07caa4ec).
 *
 * The SPA is hosted on Cloudflare Pages, where every same-origin `/api/*`
 * route is answered by the SPA fallback with index.html — there is no acb-api
 * Deployment in any fleet cluster and no public route to one (see
 * docs/notes/public-api-descope.md). Until a transport exists, every `/api`
 * client function must refuse to issue its request: these tests pin that no
 * dead call can escape from ANY code path (pages render disabled states on
 * top, but the functions are the backstop), and that the flag is false —
 * flipping it back on is a deliberate act that must come with a transport
 * and updates to these expectations.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { API_TRANSPORT_ENABLED, ApiTransportUnavailableError } from './lib/api-transport';
import {
  registerBot,
  rotateApiKey,
  fetchPredictionHistory,
  fetchOpenPredictions,
  submitPrediction,
  submitMapVote,
  fetchMapVotes,
} from './api-types';

beforeEach(() => {
  localStorage.clear();
  // Any fetch that still fires is a bug: the guard must throw first.
  globalThis.fetch = vi.fn(() => {
    throw new Error('fetch must not be called while API_TRANSPORT_ENABLED is false');
  });
});

describe('API transport flag', () => {
  it('is disabled — no public API endpoint exists (see public-api-descope.md)', () => {
    // Deliberate pin: turning this back on requires a real transport and
    // revisiting web/test-api-workflows.js, which live-probes the origin.
    expect(API_TRANSPORT_ENABLED).toBe(false);
  });

  it('ApiTransportUnavailableError names the unavailable action', () => {
    const err = new ApiTransportUnavailableError('Bot registration');
    expect(err).toBeInstanceOf(Error);
    expect(err.message).toContain('Bot registration');
    expect(err.message).toContain('no public endpoint');
  });
});

describe('/api client functions refuse to issue dead requests', () => {
  it('registerBot throws before fetching', async () => {
    await expect(registerBot({ name: 'x', endpoint_url: 'https://x', owner_id: 'o' }))
      .rejects.toBeInstanceOf(ApiTransportUnavailableError);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it('rotateApiKey throws before fetching', async () => {
    await expect(rotateApiKey('bot-1', 'secret')).rejects.toBeInstanceOf(ApiTransportUnavailableError);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it('fetchPredictionHistory throws before fetching', async () => {
    await expect(fetchPredictionHistory('predictor-1', 20)).rejects.toBeInstanceOf(ApiTransportUnavailableError);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it('fetchOpenPredictions throws before fetching', async () => {
    await expect(fetchOpenPredictions('predictor-1')).rejects.toBeInstanceOf(ApiTransportUnavailableError);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it('submitPrediction throws before fetching', async () => {
    await expect(submitPrediction('match-1', 'bot-1', 'predictor-1'))
      .rejects.toBeInstanceOf(ApiTransportUnavailableError);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it('submitMapVote throws before fetching (and writes no voter id)', async () => {
    await expect(submitMapVote('map-1', 1)).rejects.toBeInstanceOf(ApiTransportUnavailableError);
    expect(globalThis.fetch).not.toHaveBeenCalled();
    expect(localStorage.getItem('acb_voter_id')).toBeNull();
  });

  it('fetchMapVotes throws before fetching', async () => {
    await expect(fetchMapVotes('map-1')).rejects.toBeInstanceOf(ApiTransportUnavailableError);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });
});
