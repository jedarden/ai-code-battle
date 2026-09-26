/**
 * Minimal compile-time stand-ins for the Cloudflare Pages runtime globals the
 * adapters in this directory use — just enough to keep web/functions/**
 * inside the repo's tsc gate (web/tsconfig.json includes this directory)
 * without pulling in the full @cloudflare/workers-types package, which is
 * not installed. The deployment runtime provides the real implementations;
 * these declarations only pin what we compile against.
 *
 * If an adapter ever needs more of the runtime surface than is declared
 * here, prefer swapping this file for @cloudflare/workers-types over
 * widening the declarations beyond what is actually used.
 */

/** Everything an onRequest handler receives from the Pages runtime. */
declare interface PagesEventContext<E> {
	request: Request;
	env: E;
	params: Record<string, string | string[] | undefined>;
	data: Record<string, unknown>;
	functionPath: string;
	next(input?: Request | string, init?: RequestInit): Promise<Response>;
	waitUntil(promise: Promise<unknown>): void;
	passThroughOnException(): void;
}

/**
 * The canonical annotation for a Pages Function handler. Declared here
 * rather than imported because @cloudflare/workers-types is not installed;
 * the name and shape match so a future swap is deleting this file plus a
 * tsconfig `types` entry.
 */
declare type PagesFunction<E = unknown, R = Response> = (
	context: PagesEventContext<E>,
) => R | Promise<R>;

/** The slice of an R2 object body the r2 adapter reads. */
declare interface PagesR2ObjectBody {
	body: ReadableStream;
	writeHttpMetadata(headers: Headers): void;
}

/** The slice of the R2 bucket binding the r2 adapter reads. */
declare interface R2Bucket {
	get(key: string): Promise<PagesR2ObjectBody | null>;
}
