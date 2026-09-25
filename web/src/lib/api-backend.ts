// Same-origin community API for the Pages-hosted SPA (bead aicodeba-84d1d61b).
//
// Transport decision: a Cloudflare Pages Function under `web/functions/api/`
// serving `/api/*` — the path-based route docs/notes/public-api-descope.md
// named as the realistic option. It is deployed by the existing
// `acb-site-pages-build` pipeline with zero new account resources: state
// lives in the already-bound `ACB_BUCKET` R2 bucket as small JSON documents
// updated through etag-conditional puts (R2 is strongly consistent, so the
// conditional put is a compare-and-swap).
//
// Capability split. Two flows need only storage and are LIVE here:
//   - map voting        (POST /api/vote/map, GET /api/vote/map/{map_id})
//   - replay feedback   (POST /api/feedback, GET /api/feedback/{match_id},
//                        POST /api/feedback/{id}/upvote)
// plus the agentation overlay's site feedback, which shares POST /api/feedback
// (disambiguated by the presence of `markdown` in the body).
//
// The match-tier flows — bot registration, key rotation, and predictions —
// need acb-api's PostgreSQL/Valkey backend, which is not deployed anywhere
// (compute tier decommissioned 2026-07-21; revival is a documented operator
// decision). Those routes answer 503 JSON with code "match_tier_offline" so
// the SPA renders its unavailable states from the server's answer instead of
// a compile-time flag. When acb-api is revived and exposed, this function
// becomes a thin proxy for those routes and no client change is needed.
//
// Request/response shapes mirror cmd/acb-api/server.go so the SPA clients in
// api-types.ts / components/annotation.ts work against either backend.

// ─── Environment ────────────────────────────────────────────────────────────

/**
 * Structural subset of the R2 bucket binding the function actually uses.
 * The real R2Bucket satisfies this; tests supply an in-memory equivalent
 * with the same documented conditional-put semantics (failed precondition
 * → put resolves null).
 */
export interface CommunityBucket {
  get(key: string): Promise<{ etag: string; text(): Promise<string> } | null>;
  head(key: string): Promise<{ etag: string } | null>;
  put(
    key: string,
    value: string,
    opts?: { onlyIf?: { etagMatches?: string } },
  ): Promise<unknown | null>;
}

export interface ApiEnv {
  ACB_BUCKET: CommunityBucket;
}

// ─── R2 document keys and bounds ────────────────────────────────────────────

const VOTES_KEY = 'community/map-votes.json';
const REPLAY_FEEDBACK_KEY = 'community/replay-feedback.json';
const SITE_FEEDBACK_KEY = 'community/site-feedback.json';

const MAX_BODY_BYTES = 32 * 1024; // hard cap on any request body
const MAX_FEEDBACK_ENTRIES = 500; // FIFO cap on replay feedback entries
const MAX_SITE_FEEDBACK_ENTRIES = 200; // FIFO cap on agentation feedback
const MAX_VOTERS_PER_ENTRY = 1000; // dedupe set cap per feedback entry
const MAX_VOTERS_PER_MAP = 5000; // dedupe set cap per map

// ─── Capabilities ───────────────────────────────────────────────────────────

export interface ApiCapabilities {
  register: boolean;
  rotate_key: boolean;
  predictions: boolean;
  feedback: boolean;
  map_votes: boolean;
}

const MATCH_TIER_CAPABILITIES: ApiCapabilities = {
  register: false,
  rotate_key: false,
  predictions: false,
  feedback: true,
  map_votes: true,
};

// ─── Response helpers ───────────────────────────────────────────────────────

function json(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: {
      'Content-Type': 'application/json',
      // Community state changes on every POST; nothing here is cacheable.
      'Cache-Control': 'no-store',
    },
  });
}

function writeError(status: number, message: string, code?: string): Response {
  return json(code ? { error: message, code } : { error: message }, status);
}

// ─── Spam filter (ported from cmd/acb-api/spamfilter.go) ────────────────────

const SPAM_BLOCK_LIST = [
  // Profanity and offensive language
  'fuck', 'shit', 'ass', 'bitch', 'damn', 'crap',
  // Common spam patterns
  'buy now', 'click here', 'free money', 'winner', 'congratulations',
  'viagra', 'cialis', 'porn', 'xxx', 'casino', 'lottery',
  // Scam patterns
  'send bitcoin', 'crypto giveaway', 'urgent', 'act now',
  // All-caps spam (normalized to lowercase)
  'clickbait', 'subscribe', 'like and subscribe',
];

const SPAM_MIN_LENGTH = 10;

const UNICODE_SUBSTITUTIONS: Record<string, string> = {
  '0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't',
  '@': 'a', '$': 's', '+': 't', '|': 'i', '!': 'i', '©': 'c', '®': 'r',
};

function normalizeForSpam(s: string): string {
  let out = '';
  for (const ch of s.toLowerCase()) {
    out += UNICODE_SUBSTITUTIONS[ch] ?? ch;
  }
  return out;
}

/** Returns an error message when the content looks like spam, else null. */
function checkSpam(body: string): string | null {
  const normalized = normalizeForSpam(body);
  if (normalized.trim().length < SPAM_MIN_LENGTH) {
    return 'content is too short (minimum ' + SPAM_MIN_LENGTH + ' characters)';
  }
  for (const term of SPAM_BLOCK_LIST) {
    if (normalized.includes(term)) {
      return 'content contains blocked terms';
    }
  }
  return null;
}

// ─── Per-isolate rate limiting (approximate) ────────────────────────────────
// Workers isolates come and go, so these are per-isolate fixed windows: they
// bound accidental hammering and make abuse cost something, but are NOT a
// global guarantee. acb-api's limiter is the reference intent
// (feedback 20/h, votes and upvotes lighter allowances).

const RATE_LIMITS: Record<string, { limit: number; windowMs: number }> = {
  feedback: { limit: 20, windowMs: 3600_000 },
  upvote: { limit: 60, windowMs: 3600_000 },
  'map-vote': { limit: 30, windowMs: 3600_000 },
};

const rateWindows = new Map<string, { start: number; count: number }>();

function clientIp(request: Request): string {
  return request.headers.get('CF-Connecting-IP')
    ?? request.headers.get('X-Forwarded-For')?.split(',')[0]?.trim()
    ?? 'unknown';
}

function rateLimit(bucket: string, request: Request): boolean {
  const cfg = RATE_LIMITS[bucket];
  if (!cfg) return true;
  const now = Date.now();
  if (rateWindows.size > 10_000) rateWindows.clear(); // crude memory bound
  const key = bucket + ':' + clientIp(request);
  const win = rateWindows.get(key);
  if (!win || now - win.start >= cfg.windowMs) {
    rateWindows.set(key, { start: now, count: 1 });
    return true;
  }
  win.count += 1;
  return win.count <= cfg.limit;
}

// ─── Validation helpers ─────────────────────────────────────────────────────

const ID_PATTERN = /^[A-Za-z0-9_.:-]{1,128}$/;

function isId(value: unknown): value is string {
  return typeof value === 'string' && ID_PATTERN.test(value);
}

function isNonEmptyString(value: unknown, max: number): value is string {
  return typeof value === 'string' && value.length > 0 && value.length <= max;
}

function isInt(value: unknown, min: number, max: number): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= min && value <= max;
}

function generateId(prefix: string, bytes = 6): string {
  const buf = new Uint8Array(bytes);
  crypto.getRandomValues(buf);
  let hex = '';
  for (const b of buf) hex += b.toString(16).padStart(2, '0');
  return prefix + hex;
}

// ─── R2 document store (etag CAS) ───────────────────────────────────────────

const CAS_ATTEMPTS = 6;

/** Read a JSON document without writing (GET paths must stay read-only). */
async function readDoc<T>(bucket: CommunityBucket, key: string, initial: () => T): Promise<T> {
  const obj = await bucket.get(key);
  if (obj === null) return initial();
  try {
    return JSON.parse(await obj.text()) as T;
  } catch {
    return initial(); // corrupt/foreign payload: serve as empty
  }
}

/**
 * Read-modify-write a JSON document under optimistic concurrency.
 *
 * The update closure receives the parsed document and mutates it in place;
 * on success the mutated document is persisted. Between attempts the loop
 * re-reads the latest etag, so concurrent writers serialize instead of
 * clobbering (failed conditional put → null, per the R2 Workers API).
 *
 * First-ever write is unconditional (nothing to compare against yet): the
 * window is the moment the document is created, both racing writers hold
 * near-empty documents, and losing one entry there is accepted over
 * blocking every first write on an undocumented create-if-not-exists trick.
 */
async function updateDoc<T>(bucket: CommunityBucket, key: string, initial: () => T, mutate: (doc: T) => T | null): Promise<T> {
  for (let attempt = 0; attempt < CAS_ATTEMPTS; attempt++) {
    const obj = await bucket.get(key);
    if (obj === null) {
      const doc = mutate(initial());
      if (doc === null) return initial(); // mutation declined (e.g. duplicate)
      await bucket.put(key, JSON.stringify(doc));
      return doc;
    }
    let doc: T;
    try {
      doc = JSON.parse(await obj.text()) as T;
    } catch {
      // Corrupt/foreign payload: reseed rather than wedge the endpoint.
      doc = initial();
    }
    const updated = mutate(doc);
    if (updated === null) return doc; // mutation declined
    const stored = await bucket.put(key, JSON.stringify(updated), {
      onlyIf: { etagMatches: obj.etag },
    });
    if (stored !== null) return updated;
  }
  throw new StorageBusyError();
}

export class StorageBusyError extends Error {
  constructor() {
    super('storage is busy, retry shortly');
    this.name = 'StorageBusyError';
  }
}

// ─── Stored document shapes ─────────────────────────────────────────────────

interface MapVotesDoc {
  updated_at: string;
  votes: Record<string, Record<string, 1 | -1>>; // map_id -> voter_id -> vote
}

interface ReplayFeedbackEntry {
  feedback_id: string;
  match_id: string;
  turn: number;
  type: string;
  body: string;
  author: string;
  upvotes: number;
  created_at: string;
  voters: Record<string, true>;
}

interface ReplayFeedbackDoc {
  updated_at: string;
  feedback: ReplayFeedbackEntry[];
}

interface SiteFeedbackEntry {
  feedback_id: string;
  markdown: string;
  annotations: unknown[];
  submitted_at: string;
}

interface SiteFeedbackDoc {
  updated_at: string;
  feedback: SiteFeedbackEntry[];
}

function nowIso(): string {
  return new Date().toISOString();
}

function netVotesFor(votesDoc: MapVotesDoc, mapId: string): number {
  const voters = votesDoc.votes[mapId];
  if (!voters) return 0;
  let net = 0;
  for (const v of Object.values(voters)) net += v;
  return net;
}

/** Strip voter sets — they are internal dedupe state, never served. */
function toPublicFeedback(entries: ReplayFeedbackEntry[]): Omit<ReplayFeedbackEntry, 'voters'>[] {
  return entries.map(({ voters: _voters, ...entry }) => entry);
}

// ─── Route handlers ─────────────────────────────────────────────────────────

async function handleHealth(env: ApiEnv): Promise<Response> {
  const capabilities: ApiCapabilities = { ...MATCH_TIER_CAPABILITIES };
  // Reflect real storage health: an unreachable bucket flips the live
  // capabilities off so the SPA degrades instead of erroring mid-submit.
  try {
    await env.ACB_BUCKET.head(VOTES_KEY);
  } catch {
    capabilities.feedback = false;
    capabilities.map_votes = false;
  }
  return json({ status: 'ok', capabilities });
}

async function handleGetMapVotes(env: ApiEnv, mapId: string, voterId: string | null): Promise<Response> {
  if (!isId(mapId)) return writeError(400, 'invalid map ID');
  const votesDoc = await readDoc<MapVotesDoc>(env.ACB_BUCKET, VOTES_KEY,
    () => ({ updated_at: nowIso(), votes: {} }));
  const response: { map_id: string; net_votes: number; my_vote?: number } = {
    map_id: mapId,
    net_votes: netVotesFor(votesDoc, mapId),
  };
  if (voterId !== null && voterId !== '') {
    const mine = votesDoc.votes[mapId]?.[voterId];
    if (mine !== undefined) response.my_vote = mine;
  }
  return json(response);
}

async function handleMapVote(request: Request, env: ApiEnv): Promise<Response> {
  let body: unknown;
  try {
    body = JSON.parse(await cappedBody(request));
  } catch (err) {
    // An oversized body is its own answer (413 via the router), not a
    // malformed-JSON 400.
    if (err instanceof BodyTooLargeError) throw err;
    return writeError(400, 'invalid request body');
  }
  const req = body as { map_id?: unknown; voter_id?: unknown; vote?: unknown };
  if (!isId(req.map_id)) return writeError(400, 'map_id and voter_id are required');
  if (!isId(req.voter_id)) return writeError(400, 'map_id and voter_id are required');
  if (req.vote !== 1 && req.vote !== -1) return writeError(400, 'vote must be +1 or -1');
  if (!rateLimit('map-vote', request)) return writeError(429, 'too many votes, try later');
  const mapId = req.map_id;
  const voterId = req.voter_id;
  const vote = req.vote as 1 | -1;

  let votesDoc: MapVotesDoc;
  try {
    votesDoc = await updateDoc<MapVotesDoc>(env.ACB_BUCKET, VOTES_KEY,
      () => ({ updated_at: nowIso(), votes: {} }),
      (doc) => {
        const voters = doc.votes[mapId] ?? {};
        if (Object.keys(voters).length >= MAX_VOTERS_PER_MAP && voters[voterId] === undefined) {
          return null; // declined: dedupe set full
        }
        voters[voterId] = vote;
        doc.votes[mapId] = voters;
        doc.updated_at = nowIso();
        return doc;
      });
  } catch (err) {
    if (err instanceof StorageBusyError) return writeError(503, err.message, 'storage_busy');
    throw err;
  }
  return json({ map_id: mapId, vote, net_votes: netVotesFor(votesDoc, mapId) });
}

async function handleGetFeedback(env: ApiEnv, matchId: string): Promise<Response> {
  if (!isId(matchId)) return writeError(400, 'invalid match ID');
  const doc = await readDoc<ReplayFeedbackDoc>(env.ACB_BUCKET, REPLAY_FEEDBACK_KEY,
    () => ({ updated_at: nowIso(), feedback: [] }));
  const forMatch = doc.feedback.filter((f) => f.match_id === matchId);
  return json({ match_id: matchId, feedback: toPublicFeedback(forMatch) });
}

async function handleFeedbackUpvote(request: Request, env: ApiEnv, feedbackId: string): Promise<Response> {
  if (!isId(feedbackId)) return writeError(400, 'invalid feedback ID');
  let body: unknown;
  try {
    body = JSON.parse(await cappedBody(request));
  } catch (err) {
    // An oversized body is its own answer (413 via the router), not a
    // malformed-JSON 400.
    if (err instanceof BodyTooLargeError) throw err;
    return writeError(400, 'invalid request body');
  }
  const voterId = (body as { voter_id?: unknown }).voter_id;
  if (!isId(voterId)) return writeError(400, 'voter_id is required');
  if (!rateLimit('upvote', request)) return writeError(429, 'too many upvotes, try later');

  // Signals how the LAST mutate invocation (the one that was stored) saw the
  // entry, so 'already_upvoted' reflects the committed state, not a retried
  // attempt against a stale document.
  let alreadyUpvoted = false;
  let doc: ReplayFeedbackDoc;
  try {
    doc = await updateDoc<ReplayFeedbackDoc>(env.ACB_BUCKET, REPLAY_FEEDBACK_KEY,
      () => ({ updated_at: nowIso(), feedback: [] }),
      (d) => {
        const entry = d.feedback.find((f) => f.feedback_id === feedbackId);
        if (!entry) return null; // unknown feedback id → 404
        if (entry.voters[voterId] !== undefined) {
          alreadyUpvoted = true;
          return d; // idempotent: one upvote per voter
        }
        if (Object.keys(entry.voters).length >= MAX_VOTERS_PER_ENTRY) return d; // full: no-op
        entry.voters[voterId] = true;
        entry.upvotes += 1;
        d.updated_at = nowIso();
        alreadyUpvoted = false;
        return d;
      });
  } catch (err) {
    if (err instanceof StorageBusyError) return writeError(503, err.message, 'storage_busy');
    throw err;
  }
  const entry = doc.feedback.find((f) => f.feedback_id === feedbackId);
  if (!entry) return writeError(404, 'feedback not found');
  return json(alreadyUpvoted ? { status: 'already_upvoted' } : { status: 'recorded' });
}

async function handleCreateFeedback(request: Request, env: ApiEnv): Promise<Response> {
  let body: unknown;
  try {
    body = JSON.parse(await cappedBody(request));
  } catch (err) {
    // An oversized body is its own answer (413 via the router), not a
    // malformed-JSON 400.
    if (err instanceof BodyTooLargeError) throw err;
    return writeError(400, 'invalid request body');
  }
  const req = body as Record<string, unknown>;

  // Agentation overlay site feedback: {markdown, annotations, submitted_at}
  if (typeof req.markdown === 'string') {
    return createSiteFeedback(request, env, req);
  }

  // Replay feedback: {match_id, turn, type, body, author} (+ client-local
  // extras like id/position, which are ignored — the served shape is the
  // FeedbackAPIEntry contract in components/annotation.ts).
  if (!isId(req.match_id) || !isNonEmptyString(req.type, 32) || !isNonEmptyString(req.body, 2000)) {
    return writeError(400, 'match_id, type, and body are required');
  }
  const validTypes = ['insight', 'mistake', 'idea', 'highlight'];
  if (!validTypes.includes(req.type)) {
    return writeError(400, 'type must be one of: insight, mistake, idea, highlight');
  }
  if (!isInt(req.turn, 0, 1_000_000)) return writeError(400, 'invalid turn');
  const author = req.author === undefined || req.author === '' ? 'Anonymous' : req.author;
  if (!isNonEmptyString(author, 64)) return writeError(400, 'invalid author');
  const spam = checkSpam(req.body);
  if (spam !== null) return writeError(422, spam);
  if (!rateLimit('feedback', request)) return writeError(429, 'too much feedback, try later');

  const feedbackId = generateId('fb_');
  try {
    await updateDoc<ReplayFeedbackDoc>(env.ACB_BUCKET, REPLAY_FEEDBACK_KEY,
      () => ({ updated_at: nowIso(), feedback: [] }),
      (doc) => {
        doc.feedback.unshift({
          feedback_id: feedbackId,
          match_id: req.match_id as string,
          turn: req.turn as number,
          type: req.type as string,
          body: req.body as string,
          author: author as string,
          upvotes: 0,
          created_at: nowIso(),
          voters: {},
        });
        if (doc.feedback.length > MAX_FEEDBACK_ENTRIES) doc.feedback.length = MAX_FEEDBACK_ENTRIES;
        doc.updated_at = nowIso();
        return doc;
      });
  } catch (err) {
    if (err instanceof StorageBusyError) return writeError(503, err.message, 'storage_busy');
    throw err;
  }
  return json({ status: 'recorded', feedback_id: feedbackId }, 201);
}

async function createSiteFeedback(request: Request, env: ApiEnv, req: Record<string, unknown>): Promise<Response> {
  const markdown = req.markdown;
  if (!isNonEmptyString(markdown, 8000)) return writeError(400, 'markdown is required');
  if (req.annotations !== undefined && !Array.isArray(req.annotations)) {
    return writeError(400, 'annotations must be an array');
  }
  const annotations = (req.annotations as unknown[] | undefined)?.slice(0, 50) ?? [];
  const submittedAt = typeof req.submitted_at === 'string' && req.submitted_at.length <= 40
    ? req.submitted_at
    : nowIso();
  const spam = checkSpam(markdown);
  if (spam !== null) return writeError(422, spam);
  if (!rateLimit('feedback', request)) return writeError(429, 'too much feedback, try later');

  const feedbackId = generateId('site_');
  try {
    await updateDoc<SiteFeedbackDoc>(env.ACB_BUCKET, SITE_FEEDBACK_KEY,
      () => ({ updated_at: nowIso(), feedback: [] }),
      (doc) => {
        doc.feedback.unshift({
          feedback_id: feedbackId,
          markdown,
          annotations,
          submitted_at: submittedAt,
        });
        if (doc.feedback.length > MAX_SITE_FEEDBACK_ENTRIES) doc.feedback.length = MAX_SITE_FEEDBACK_ENTRIES;
        doc.updated_at = nowIso();
        return doc;
      });
  } catch (err) {
    if (err instanceof StorageBusyError) return writeError(503, err.message, 'storage_busy');
    throw err;
  }
  return json({ status: 'recorded', feedback_id: feedbackId }, 201);
}

// ─── Match-tier placeholder ─────────────────────────────────────────────────

function matchTierOffline(action: string): Response {
  return writeError(
    503,
    action + ' is offline: the compute tier that runs matches is not deployed ' +
    '(see docs/notes/api-transport.md).',
    'match_tier_offline',
  );
}

// ─── Request body helper ────────────────────────────────────────────────────

async function cappedBody(request: Request): Promise<string> {
  const raw = await request.text();
  if (raw.length > MAX_BODY_BYTES) throw new BodyTooLargeError();
  return raw;
}

class BodyTooLargeError extends Error {}

// ─── Router ─────────────────────────────────────────────────────────────────

/**
 * Entry point for every `/api/*` request. `path` is the request pathname
 * with the `/api` prefix already stripped (so "/api/vote/map" → "/vote/map");
 * a trailing slash is ignored.
 */
export async function handleApiRequest(request: Request, env: ApiEnv, path: string): Promise<Response> {
  const route = path.replace(/\/+$/, '') || '/';
  const method = request.method.toUpperCase();

  try {
    if (route === '/health' && method === 'GET') return await handleHealth(env);

    if (route === '/vote/map' && method === 'POST') return await handleMapVote(request, env);
    if (route.startsWith('/vote/map/') && method === 'GET') {
      const mapId = route.slice('/vote/map/'.length);
      const voterId = new URL(request.url).searchParams.get('voter_id');
      return await handleGetMapVotes(env, mapId, voterId);
    }

    if (route === '/feedback' && method === 'POST') return await handleCreateFeedback(request, env);
    if (route.startsWith('/feedback/') && route.endsWith('/upvote') && method === 'POST') {
      const feedbackId = route.slice('/feedback/'.length, -'/upvote'.length);
      return await handleFeedbackUpvote(request, env, feedbackId);
    }
    if (route.startsWith('/feedback/') && method === 'GET') {
      return await handleGetFeedback(env, route.slice('/feedback/'.length));
    }

    // Match-tier routes: honest 503 JSON until acb-api is deployed and exposed.
    if ((route === '/register' || route === '/rotate-key' || route === '/predict') && method === 'POST') {
      return matchTierOffline(route === '/register' ? 'Bot registration'
        : route === '/rotate-key' ? 'API key rotation' : 'Match predictions');
    }
    if ((route === '/predictions/open' || route === '/predictions/history') && method === 'GET') {
      return matchTierOffline('Predictions');
    }

    return writeError(404, 'not found');
  } catch (err) {
    if (err instanceof BodyTooLargeError) return writeError(413, 'request body too large');
    if (err instanceof StorageBusyError) return writeError(503, err.message, 'storage_busy');
    const msg = err instanceof Error ? err.message : String(err);
    return writeError(500, 'internal error: ' + msg);
  }
}
