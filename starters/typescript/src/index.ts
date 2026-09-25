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

// Any other content type must reach the turn handler, which rejects it as
// an authentication failure, instead of Fastify's generic 415.
app.addContentTypeParser(
  "*",
  { parseAs: "buffer" },
  async (_request: FastifyRequest, body: Buffer) => body
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

// --- Request schema (docs/bot-protocol.md) ---
const REQUIRED_TOP = ["match_id", "turn", "config", "you", "bots", "energy", "cores", "walls", "dead"];
const REQUIRED_CONFIG = ["rows", "cols", "max_turns", "vision_radius2", "attack_radius2", "spawn_cost", "energy_interval", "cores_per_player", "zone_enabled", "zone_start_turn", "zone_shrink_interval", "zone_shrink_step", "zone_min_radius", "kill_score"];
const OPTIONAL_CONFIG = ["map_id", "season_id", "rules_version", "turn_timeout"];

// Returns null when the parsed body is schema-conformant, otherwise a
// short reason. Runs after signature verification: a failure here is
// authenticated malformed input (400), never an authentication failure.
// Unknown fields are rejected everywhere the contract closes an object
// (top level, config, you, zone, zone.center, and every array element),
// and required fields must carry the documented JSON types.
function isJsonObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function positionError(pos: unknown, where: string): string | null {
  if (!isJsonObject(pos)) return `${where} must be an object`;
  for (const key of Object.keys(pos)) {
    if (key !== "row" && key !== "col") return `unknown ${where} field: ${key}`;
  }
  if (!Number.isInteger(pos.row) || !Number.isInteger(pos.col)) {
    return `${where} requires integer row/col`;
  }
  return null;
}

function elementError(el: unknown, shape: "bot" | "core" | "point", where: string): string | null {
  if (!isJsonObject(el)) return `${where} elements must be objects`;
  if (shape === "point") {
    for (const key of Object.keys(el)) {
      if (key !== "row" && key !== "col") return `unknown ${where} element field: ${key}`;
    }
    if (!Number.isInteger(el.row) || !Number.isInteger(el.col)) {
      return `${where} elements require integer row/col`;
    }
    return null;
  }
  const allowed = shape === "core" ? ["position", "owner", "active"] : ["position", "owner"];
  for (const key of Object.keys(el)) {
    if (!allowed.includes(key)) return `unknown ${where} element field: ${key}`;
  }
  const posError = positionError(el.position, `${where} element position`);
  if (posError !== null) return posError;
  if (!Number.isInteger(el.owner)) return `${where} elements require integer owner`;
  if (shape === "core" && typeof el.active !== "boolean") {
    return `${where} elements require boolean active`;
  }
  return null;
}

function validateRequestSchema(state: unknown): string | null {
  if (!isJsonObject(state)) return "request must be one JSON object";
  const s = state as Record<string, unknown>;
  const knownTop = new Set([...REQUIRED_TOP, "zone"]);
  for (const key of Object.keys(s)) {
    if (!knownTop.has(key)) return `unknown field: ${key}`;
  }
  for (const key of REQUIRED_TOP) {
    if (!(key in s)) return `missing required field: ${key}`;
  }
  if (typeof s.match_id !== "string" || s.match_id === "") {
    return "match_id must be a non-empty string";
  }
  if (!Number.isInteger(s.turn)) return "turn must be an integer";
  const config = s.config;
  if (!isJsonObject(config)) return "config must be an object";
  const knownConfig = new Set([...REQUIRED_CONFIG, ...OPTIONAL_CONFIG]);
  for (const key of Object.keys(config)) {
    if (!knownConfig.has(key)) return `unknown config field: ${key}`;
  }
  for (const key of REQUIRED_CONFIG) {
    if (!(key in config)) return `config missing required field: ${key}`;
    if (key === "zone_enabled") {
      if (typeof config[key] !== "boolean") return "config.zone_enabled must be a boolean";
    } else if (!Number.isInteger(config[key])) {
      return `config.${key} must be an integer`;
    }
  }
  for (const key of ["map_id", "season_id", "rules_version"]) {
    if (key in config && typeof config[key] !== "string") {
      return `config.${key} must be a string`;
    }
  }
  if ("turn_timeout" in config && !Number.isInteger(config.turn_timeout)) {
    return "config.turn_timeout must be an integer";
  }
  const you = s.you;
  if (!isJsonObject(you)) return "you must be an object";
  for (const key of Object.keys(you)) {
    if (key !== "id" && key !== "energy" && key !== "score") {
      return `unknown you field: ${key}`;
    }
  }
  for (const key of ["id", "energy", "score"]) {
    if (!(key in you)) return `you missing required field: ${key}`;
    if (!Number.isInteger(you[key])) return `you.${key} must be an integer`;
  }
  for (const key of ["bots", "energy", "cores", "walls", "dead"]) {
    if (!Array.isArray(s[key])) return `${key} must be an array`;
  }
  for (const el of s.bots as unknown[]) {
    const err = elementError(el, "bot", "bots");
    if (err !== null) return err;
  }
  for (const el of s.dead as unknown[]) {
    const err = elementError(el, "bot", "dead");
    if (err !== null) return err;
  }
  for (const el of s.energy as unknown[]) {
    const err = elementError(el, "point", "energy");
    if (err !== null) return err;
  }
  for (const el of s.walls as unknown[]) {
    const err = elementError(el, "point", "walls");
    if (err !== null) return err;
  }
  for (const el of s.cores as unknown[]) {
    const err = elementError(el, "core", "cores");
    if (err !== null) return err;
  }
  const zone = s.zone;
  if (zone !== undefined && zone !== null) {
    if (!isJsonObject(zone)) return "zone must be an object";
    for (const key of Object.keys(zone)) {
      if (key !== "center" && key !== "radius" && key !== "active") {
        return `unknown zone field: ${key}`;
      }
    }
    for (const key of ["center", "radius", "active"]) {
      if (!(key in zone)) return `zone missing required field: ${key}`;
    }
    const centerError = positionError(zone.center, "zone.center");
    if (centerError !== null) return centerError;
    if (!Number.isInteger(zone.radius)) return "zone.radius must be an integer";
    if (typeof zone.active !== "boolean") return "zone.active must be a boolean";
  }
  return null;
}

app.post("/turn", async (request: FastifyRequest, reply: FastifyReply) => {
  // Content-Type is part of the transport contract (step 1 of the
  // documented verification order).
  if (request.headers["content-type"] !== "application/json") {
    reply.type("text/plain").code(401);
    return "Invalid content type";
  }

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

  const schemaError = validateRequestSchema(state);
  if (schemaError !== null) {
    reply.type("text/plain").code(400);
    return `Invalid request schema: ${schemaError}`;
  }

  if (state.match_id !== matchId || state.turn !== turnNumber) {
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
