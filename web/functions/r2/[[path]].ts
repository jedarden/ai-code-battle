interface Env {
	ACB_BUCKET: R2Bucket;
}

export const onRequest: PagesFunction<Env> = async (context) => {
	try {
		const url = new URL(context.request.url);
		// Strip the leading /r2/ prefix to get the R2 object key
		const key = url.pathname.replace(/^\/r2\//, '');

		if (!key) {
			return new Response('Not Found', { status: 404 });
		}

		if (!context.env.ACB_BUCKET) {
			return new Response('R2 binding not configured', { status: 503 });
		}

		const object = await context.env.ACB_BUCKET.get(key);
		if (!object) {
			return new Response('Not Found', { status: 404 });
		}

		const headers = new Headers();
		object.writeHttpMetadata(headers);
		headers.set('Cache-Control', 'public, max-age=60');
		headers.set('Access-Control-Allow-Origin', '*');

		// Cloudflare CDN strips Content-Encoding: gzip from Worker responses even
		// when we set it, causing browsers to receive raw gzip bytes as JSON.
		// Decompress .gz objects inside the Worker instead so the response body is
		// always plain, browser-parseable content (no Content-Encoding needed).
		if (key.endsWith('.gz')) {
			headers.delete('Content-Encoding');
			const decompressed = object.body.pipeThrough(new DecompressionStream('gzip'));
			return new Response(decompressed, { headers });
		}

		return new Response(object.body, { headers });
	} catch (err: unknown) {
		const msg = err instanceof Error ? err.message : String(err);
		return new Response(`Error: ${msg}`, { status: 500 });
	}
};
