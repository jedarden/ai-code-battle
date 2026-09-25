// SPA→API transport (bead aicodeba-84d1d61b).
//
// The transport is a Cloudflare Pages Function under `web/functions/api/`
// serving same-origin `/api/*` — the path-based route
// docs/notes/public-api-descope.md named as the realistic option. It is
// deployed with the site by the existing `acb-site-pages-build` pipeline and
// needs no new account resources: its store is the already-bound `ACB_BUCKET`
// R2 bucket. See src/lib/api-backend.ts for the server contract and
// docs/notes/api-transport.md for the decision record.
//
// The function implements what storage alone can support — community
// feedback, replay annotations, and map voting — and answers the match-tier
// routes (registration, key rotation, predictions) with 503 JSON,
// code "match_tier_offline", because acb-api's PostgreSQL/Valkey backend is
// not deployed anywhere (compute tier decommissioned 2026-07-21; revival is a
// documented operator decision).
//
// Availability is server-driven, in two layers:
//   - API_TRANSPORT_ENABLED is the compile-time kill switch. False restores
//     the pre-transport behavior (clients refuse to issue /api requests,
//     pages render their unavailable states). It is true because a real
//     transport now exists; web/test-api-workflows.js live-probes the origin
//     and fails loudly if /api ever stops answering with JSON.
//   - At runtime the function's answers decide per-flow availability: the
//     match-tier routes answer 503 JSON with code "match_tier_offline"
//     (detected by matchTierOfflineMessage below), and GET /api/health
//     reports the live capability set for operators and the live probe.
//     A revived compute tier flips those answers — no client change needed.

export const API_TRANSPORT_ENABLED = true;

/** Thrown by the `/api` client functions while API_TRANSPORT_ENABLED is false. */
export class ApiTransportUnavailableError extends Error {
  readonly code = 'acb_api_transport_unavailable';

  constructor(action: string) {
    super(
      `${action} is unavailable: the AI Code Battle API has no public endpoint ` +
        '(see docs/notes/public-api-descope.md).',
    );
    this.name = 'ApiTransportUnavailableError';
  }
}

/**
 * Thrown when the transport is up but the server reports the flow depends on
 * the match tier (acb-api + PostgreSQL + workers), which is not deployed.
 * Carries the server's user-facing message.
 */
export class MatchTierOfflineError extends Error {
  readonly code = 'match_tier_offline';

  constructor(message: string) {
    super(message);
    this.name = 'MatchTierOfflineError';
  }
}

/**
 * Detect a match-tier 503 from the function. Returns the parsed body's
 * user-facing message when the response is one, else null.
 */
export function matchTierOfflineMessage(status: number, body: unknown): string | null {
  if (status !== 503) return null;
  const code = (body as { code?: unknown } | null)?.code;
  const error = (body as { error?: unknown } | null)?.error;
  if (code === 'match_tier_offline' && typeof error === 'string') return error;
  return null;
}

/**
 * Guard for `/api` client functions. Call first — while the kill switch is
 * off this throws before any fetch is issued, so the caller's error path
 * renders instead of a misleading success against the SPA HTML fallback.
 */
export function requireApiTransport(action: string): void {
  if (!API_TRANSPORT_ENABLED) throw new ApiTransportUnavailableError(action);
}
