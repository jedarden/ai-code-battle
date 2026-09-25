// Same-origin /api/* route (bead aicodeba-84d1d61b).
//
// All logic lives in src/lib/api-backend.ts so it is covered by the repo's
// tsc gate and its contract suite (src/lib/api-backend.test.ts); like the
// sibling r2/ wrapper this file sits outside the tsconfig program (no
// workers-types package) and only adapts the Pages Function contract.
// Functions take precedence over static assets on Pages, so these routes
// are no longer answered by the SPA fallback.
import { handleApiRequest } from '../../src/lib/api-backend';

interface Env {
	ACB_BUCKET: R2Bucket;
}

export const onRequest: PagesFunction<Env> = async (context) => {
	const url = new URL(context.request.url);
	const path = url.pathname.replace(/^\/api/, '');
	return handleApiRequest(context.request, context.env, path);
};
