// SPA→API transport status (bead aicodeba-07caa4ec).
//
// The SPA is deployed to Cloudflare Pages, and Pages serves no `/api` backend:
// every same-origin `/api/*` request is answered by the SPA fallback with this
// app's own index.html (HTTP 200, text/html). No acb-api Deployment runs in
// any fleet cluster and no public route to one exists — the public API
// endpoint was descoped (docs/notes/public-api-descope.md) until social
// features justify a transport (Pages Function proxy or registered-domain
// IngressRoute).
//
// While the transport is absent, every workflow that writes to or syncs
// through `/api` — registration, predictions, community feedback, and map
// voting — is disabled at two layers instead of being left to fail
// mid-submit or, worse, report success against the HTML fallback:
//
//   1. API level: the `/api` client functions in api-types.ts and
//      components/annotation.ts refuse to issue the request (they throw
//      ApiTransportUnavailableError or degrade to their static/local
//      fallback), so no dead call can escape from any code path.
//   2. UI level: the pages render explicit "unavailable" notices and
//      disabled forms/buttons so nothing looks half-broken to a visitor.
//
// Static `/data` reads are unaffected — they are bundled into the Pages
// deploy by the index builder. Flipping API_TRANSPORT_ENABLED back to true
// (only after a real transport exists) restores every `/api` code path; each
// call site guards on this one flag, so the forms come back with their
// notices gone. web/test-api-workflows.js live-probes the deployed origin
// and fails loudly if `/api` ever stops answering with the SPA fallback,
// which is the signal that this flag needs revisiting.

export const API_TRANSPORT_ENABLED = false;

/** Thrown by the `/api` client functions while API_TRANSPORT_ENABLED is false. */
export class ApiTransportUnavailableError extends Error {
  readonly code = 'acb_api_transport_unavailable';

  constructor(action: string) {
    super(
      `${action} is unavailable: the AI Code Battle API has no public endpoint yet ` +
        '(see docs/notes/public-api-descope.md).',
    );
    this.name = 'ApiTransportUnavailableError';
  }
}

/**
 * Guard for `/api` client functions. Call first — while the transport is
 * disabled this throws before any fetch is issued, so the caller's error
 * path renders instead of a misleading success against the SPA HTML fallback.
 */
export function requireApiTransport(action: string): void {
  if (!API_TRANSPORT_ENABLED) throw new ApiTransportUnavailableError(action);
}
