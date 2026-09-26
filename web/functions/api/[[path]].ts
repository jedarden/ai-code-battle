// Same-origin /api/* route (bead aicodeba-84d1d61b).
//
// All logic lives in src/lib/api-backend.ts so the repo's tsc gate and its
// contract suite (src/lib/api-backend.test.ts) cover it. This adapter is
// part of that program too: web/tsconfig.json includes web/functions/**
// (runtime globals via functions/pages-runtime.d.ts, minimal stand-ins for
// the uninstalled @cloudflare/workers-types), and the env contract here is
// the very ApiEnv the contract suite tests — not a parallel R2Bucket shape.
// The wiring itself — URL parse, one-time /api prefix strip, env hand-off —
// is pinned by src/lib/api-function-adapter.test.ts, which drives this
// exported onRequest end-to-end.
// Functions take precedence over static assets on Pages, so these routes
// are no longer answered by the SPA fallback.
import { handleApiRequest, type ApiEnv } from '../../src/lib/api-backend';

export const onRequest: PagesFunction<ApiEnv> = async (context) => {
	const url = new URL(context.request.url);
	const path = url.pathname.replace(/^\/api/, '');
	return handleApiRequest(context.request, context.env, path);
};
