/**
 * AI Code Battle - TypeScript Starter Bot
 *
 * Fastify HTTP server with HMAC authentication for the AI Code Battle platform.
 *
 * Environment variables:
 *   BOT_SECRET - Your bot's shared secret (required)
 *   BOT_PORT   - Port to listen on (default: 8080)
 */

import Fastify, { FastifyRequest, FastifyReply } from "fastify";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import type { VisibleState, TurnResponse } from "./types.js";
import {
  verifySignature,
  signResponse,
  verifyTimestamp,
  getAuthHeaders,
} from "./auth.js";
import { computeMoves } from "./strategy.js";

const PORT = parseInt(process.env.BOT_PORT || "8080", 10);
const SECRET = process.env.BOT_SECRET || "";

function parseTurn(turn: string): number | null {
  if (!/^(0|[1-9]\d*)$/.test(turn)) return null;
  const value = Number(turn);
  return Number.isSafeInteger(value) ? value : null;
}

if (!SECRET) {
  console.error("ERROR: BOT_SECRET environment variable is required");
  process.exit(1);
}

// Create Fastify instance with a custom content parser to capture raw body
const app = Fastify({
  logger: false, // Set to true for HTTP request logging
});

// Add a custom parser to store raw body bytes for signature verification
app.addContentTypeParser(
  "application/json",
  { parseAs: "buffer" },
  async (
    request: FastifyRequest,
    body: Buffer
  ) => {
    // Store raw body for signature verification
    (request as any).rawBody = body;
    return body;
  }
);

/**
 * Health check endpoint - used during bot registration.
 */
app.get("/health", async (_request: FastifyRequest, reply: FastifyReply) => {
  reply.type("text/plain").code(200);
  return "OK";
});

/**
 * Main game turn endpoint.
 * Receives game state JSON, computes moves, returns moves JSON.
 */
app.post("/turn", async (request: FastifyRequest, reply: FastifyReply) => {
  // Get raw body as bytes for signature verification
  const rawBody = (request as any).rawBody;
  if (!Buffer.isBuffer(rawBody)) {
    reply.type("text/plain").code(400);
    return "Invalid request body";
  }

  // Extract auth headers
  const { matchId, turn, timestamp, botId, signature } = getAuthHeaders(
    request.headers
  );
  const turnNumber = parseTurn(turn);

  if (
    !matchId ||
    turnNumber === null ||
    !timestamp ||
    !botId ||
    !signature
  ) {
    reply.type("text/plain").code(401);
    return "Missing auth headers";
  }

  // Verify HMAC signature over the exact request bytes
  if (
    !verifySignature(rawBody, matchId, turn, timestamp, signature, SECRET)
  ) {
    reply.type("text/plain").code(401);
    return "Invalid signature";
  }

  // Verify timestamp (prevent replay attacks)
  if (!verifyTimestamp(timestamp)) {
    reply.type("text/plain").code(401);
    return "Invalid timestamp";
  }

  // Parse game state JSON after authentication
  let state: VisibleState;
  try {
    state = JSON.parse(rawBody.toString("utf-8"));
  } catch {
    reply.type("text/plain").code(400);
    return "Invalid JSON";
  }

  if (
    state === null ||
    typeof state !== "object" ||
    state.match_id !== matchId ||
    state.turn !== turnNumber
  ) {
    reply.type("text/plain").code(401);
    return "Invalid request identity";
  }

  // Log match start (turn 0)
  if (state.turn === 0) {
    console.log(
      `match=${state.match_id} ` +
        `season_id=${state.config.season_id || "none"} ` +
        `rules_version=${state.config.rules_version || "none"} ` +
        `rows=${state.config.rows} cols=${state.config.cols}`
    );
  }

  // Compute moves
  const moves = computeMoves(state);

  // Build response
  const responseBody: TurnResponse = { moves };
  const responseBuffer = Buffer.from(JSON.stringify(responseBody));

  // Sign the exact serialized response body without a timestamp
  const responseSig = signResponse(
    responseBuffer,
    matchId,
    turnNumber,
    SECRET
  );

  // Send response with signature header
  reply
    .code(200)
    .header("Content-Type", "application/json")
    .header("X-ACB-Signature", responseSig);
  return responseBuffer;
});

/**
 * Read package.json for version info
 */
const __dirname = fileURLToPath(new URL(".", import.meta.url));
let version = "unknown";
try {
  const pkg = JSON.parse(
    await readFile(join(__dirname, "..", "package.json"), "utf-8")
  );
  version = pkg.version;
} catch {
  // Ignore
}

/**
 * Start the server
 */
const start = async () => {
  try {
    await app.listen({ port: PORT, host: "0.0.0.0" });
    console.log(`acb-starter-typescript v${version} listening on port ${PORT}`);
  } catch (err) {
    app.log.error(err);
    process.exit(1);
  }
};

start();
