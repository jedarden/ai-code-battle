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
import type { GameState, MoveResponse } from "./types.js";
import {
  verifySignature,
  signResponse,
  verifyTimestamp,
  getAuthHeaders,
} from "./auth.js";
import { computeMoves } from "./strategy.js";

const PORT = parseInt(process.env.BOT_PORT || "8080", 10);
const SECRET = process.env.BOT_SECRET || "";

if (!SECRET) {
  console.error("ERROR: BOT_SECRET environment variable is required");
  process.exit(1);
}

// Create Fastify instance with a custom content parser to capture raw body
const app = Fastify({
  logger: false, // Set to true for HTTP request logging
});

// Add a custom parser to store raw body string for signature verification
app.addContentTypeParser(
  "application/json",
  { parseAs: "string" },
  async (
    request: FastifyRequest,
    body: string
  ) => {
    // Store raw body for signature verification
    (request as any).rawBody = body;
    // Also return parsed JSON for normal use
    return JSON.parse(body);
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
  // Get raw body as string for signature verification
  const rawBody = (request as any).rawBody;
  if (typeof rawBody !== "string") {
    reply.type("text/plain").code(400);
    return "Invalid request body";
  }
  const bodyBuffer = Buffer.from(rawBody, "utf-8");

  // Extract auth headers
  const headers = request.headers as Record<string, string>;
  const { matchId, turn, timestamp, signature } = getAuthHeaders(headers);

  // Verify HMAC signature
  if (
    !signature ||
    !verifySignature(bodyBuffer, matchId, turn, timestamp, signature, SECRET)
  ) {
    reply.type("text/plain").code(401);
    return "Invalid signature";
  }

  // Verify timestamp (prevent replay attacks)
  if (!verifyTimestamp(timestamp)) {
    reply.type("text/plain").code(401);
    return "Invalid timestamp";
  }

  // Parse game state JSON (already parsed by our custom parser)
  const state: GameState = request.body as GameState;

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
  const responseBody: MoveResponse = { moves };
  const responseJson = JSON.stringify(responseBody);

  // Sign response
  const responseSig = signResponse(
    responseJson,
    matchId,
    parseInt(turn, 10),
    SECRET
  );

  // Send response with signature header
  reply
    .code(200)
    .header("Content-Type", "application/json")
    .header("X-ACB-Signature", responseSig);
  return responseJson;
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
