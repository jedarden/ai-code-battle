/**
 * SwarmBot - Formation-based combat strategy for AI Code Battle.
 *
 * HTTP server that handles game engine requests with HMAC authentication.
 */

import * as crypto from 'crypto';
import * as http from 'http';
import { GameState, MoveResponse } from './game.js';
import { SwarmStrategy } from './strategy.js';

const PORT = parseInt(process.env.BOT_PORT || '8084', 10);
const SECRET = process.env.BOT_SECRET || '';

if (!SECRET) {
  console.error('ERROR: BOT_SECRET environment variable is required');
  process.exit(1);
}

const strategy = new SwarmStrategy();

// --- Request schema (docs/bot-protocol.md) ---
const REQUIRED_TOP = ['match_id', 'turn', 'config', 'you', 'bots', 'energy', 'cores', 'walls', 'dead'];
const REQUIRED_CONFIG = ['rows', 'cols', 'max_turns', 'vision_radius2', 'attack_radius2', 'spawn_cost', 'energy_interval', 'cores_per_player', 'zone_enabled', 'zone_start_turn', 'zone_shrink_interval', 'zone_shrink_step', 'zone_min_radius', 'kill_score'];
const OPTIONAL_CONFIG = ['map_id', 'season_id', 'rules_version', 'turn_timeout'];

// Returns null when the parsed body is schema-conformant, otherwise a
// short reason. Runs after signature verification: a failure here is
// authenticated malformed input (400), never an authentication failure.
// Unknown fields are rejected everywhere the contract closes an object
// (top level, config, you, zone, zone.center, and every array element),
// and required fields must carry the documented JSON types.
function isJsonObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function positionError(pos: unknown, where: string): string | null {
  if (!isJsonObject(pos)) return `${where} must be an object`;
  for (const key of Object.keys(pos)) {
    if (key !== 'row' && key !== 'col') return `unknown ${where} field: ${key}`;
  }
  if (!Number.isInteger(pos.row) || !Number.isInteger(pos.col)) {
    return `${where} requires integer row/col`;
  }
  return null;
}

function elementError(el: unknown, shape: 'bot' | 'core' | 'point', where: string): string | null {
  if (!isJsonObject(el)) return `${where} elements must be objects`;
  if (shape === 'point') {
    for (const key of Object.keys(el)) {
      if (key !== 'row' && key !== 'col') return `unknown ${where} element field: ${key}`;
    }
    if (!Number.isInteger(el.row) || !Number.isInteger(el.col)) {
      return `${where} elements require integer row/col`;
    }
    return null;
  }
  const allowed = shape === 'core' ? ['position', 'owner', 'active'] : ['position', 'owner'];
  for (const key of Object.keys(el)) {
    if (!allowed.includes(key)) return `unknown ${where} element field: ${key}`;
  }
  const posError = positionError(el.position, `${where} element position`);
  if (posError !== null) return posError;
  if (!Number.isInteger(el.owner)) return `${where} elements require integer owner`;
  if (shape === 'core' && typeof el.active !== 'boolean') {
    return `${where} elements require boolean active`;
  }
  return null;
}

function validateRequestSchema(state: unknown): string | null {
  if (!isJsonObject(state)) return 'request must be one JSON object';
  const s = state as Record<string, unknown>;
  const knownTop = new Set([...REQUIRED_TOP, 'zone']);
  for (const key of Object.keys(s)) {
    if (!knownTop.has(key)) return `unknown field: ${key}`;
  }
  for (const key of REQUIRED_TOP) {
    if (!(key in s)) return `missing required field: ${key}`;
  }
  if (typeof s.match_id !== 'string' || s.match_id === '') {
    return 'match_id must be a non-empty string';
  }
  if (!Number.isInteger(s.turn)) return 'turn must be an integer';
  const config = s.config;
  if (!isJsonObject(config)) return 'config must be an object';
  const knownConfig = new Set([...REQUIRED_CONFIG, ...OPTIONAL_CONFIG]);
  for (const key of Object.keys(config)) {
    if (!knownConfig.has(key)) return `unknown config field: ${key}`;
  }
  for (const key of REQUIRED_CONFIG) {
    if (!(key in config)) return `config missing required field: ${key}`;
    if (key === 'zone_enabled') {
      if (typeof config[key] !== 'boolean') return 'config.zone_enabled must be a boolean';
    } else if (!Number.isInteger(config[key])) {
      return `config.${key} must be an integer`;
    }
  }
  for (const key of ['map_id', 'season_id', 'rules_version']) {
    if (key in config && typeof config[key] !== 'string') {
      return `config.${key} must be a string`;
    }
  }
  if ('turn_timeout' in config && !Number.isInteger(config.turn_timeout)) {
    return 'config.turn_timeout must be an integer';
  }
  const you = s.you;
  if (!isJsonObject(you)) return 'you must be an object';
  for (const key of Object.keys(you)) {
    if (key !== 'id' && key !== 'energy' && key !== 'score') {
      return `unknown you field: ${key}`;
    }
  }
  for (const key of ['id', 'energy', 'score']) {
    if (!(key in you)) return `you missing required field: ${key}`;
    if (!Number.isInteger(you[key])) return `you.${key} must be an integer`;
  }
  for (const key of ['bots', 'energy', 'cores', 'walls', 'dead']) {
    if (!Array.isArray(s[key])) return `${key} must be an array`;
  }
  for (const el of s.bots as unknown[]) {
    const err = elementError(el, 'bot', 'bots');
    if (err !== null) return err;
  }
  for (const el of s.dead as unknown[]) {
    const err = elementError(el, 'bot', 'dead');
    if (err !== null) return err;
  }
  for (const el of s.energy as unknown[]) {
    const err = elementError(el, 'point', 'energy');
    if (err !== null) return err;
  }
  for (const el of s.walls as unknown[]) {
    const err = elementError(el, 'point', 'walls');
    if (err !== null) return err;
  }
  for (const el of s.cores as unknown[]) {
    const err = elementError(el, 'core', 'cores');
    if (err !== null) return err;
  }
  const zone = s.zone;
  if (zone !== undefined && zone !== null) {
    if (!isJsonObject(zone)) return 'zone must be an object';
    for (const key of Object.keys(zone)) {
      if (key !== 'center' && key !== 'radius' && key !== 'active') {
        return `unknown zone field: ${key}`;
      }
    }
    for (const key of ['center', 'radius', 'active']) {
      if (!(key in zone)) return `zone missing required field: ${key}`;
    }
    const centerError = positionError(zone.center, 'zone.center');
    if (centerError !== null) return centerError;
    if (!Number.isInteger(zone.radius)) return 'zone.radius must be an integer';
    if (typeof zone.active !== 'boolean') return 'zone.active must be a boolean';
  }
  return null;
}

const server = http.createServer((req, res) => {
  if (req.method === 'GET' && req.url === '/health') {
    res.writeHead(200, { 'Content-Type': 'text/plain' });
    res.end('OK');
    return;
  }

  if (req.method === 'POST' && req.url === '/turn') {
    handleTurn(req, res);
    return;
  }

  res.writeHead(404, { 'Content-Type': 'text/plain' });
  res.end('Not Found');
});

async function handleTurn(req: http.IncomingMessage, res: http.ServerResponse): Promise<void> {
  // Content-Type is part of the transport contract (step 1 of the
  // documented verification order).
  if (getHeader(req, 'content-type') !== 'application/json') {
    res.writeHead(401, { 'Content-Type': 'text/plain' });
    res.end('Invalid content type');
    return;
  }

  // Extract auth headers
  const matchId = getHeader(req, 'x-acb-match-id');
  const turnStr = getHeader(req, 'x-acb-turn');
  const timestamp = getHeader(req, 'x-acb-timestamp');
  const botId = getHeader(req, 'x-acb-bot-id');
  const signature = getHeader(req, 'x-acb-signature');
  const turn = parseTurn(turnStr);

  if (!matchId || turn === null || !timestamp || !botId || !signature) {
    res.writeHead(401, { 'Content-Type': 'text/plain' });
    res.end('Missing auth headers');
    return;
  }

  // Read body
  const chunks: Buffer[] = [];
  for await (const chunk of req) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  }
  const body = Buffer.concat(chunks);

  // Verify signature
  if (!verifySignature(SECRET, matchId, turnStr, timestamp, body, signature)) {
    res.writeHead(401, { 'Content-Type': 'text/plain' });
    res.end('Invalid signature');
    return;
  }

  if (!verifyTimestamp(timestamp)) {
    res.writeHead(401, { 'Content-Type': 'text/plain' });
    res.end('Invalid timestamp');
    return;
  }

  // Parse game state
  let state: GameState;
  try {
    state = JSON.parse(body.toString('utf8'));
  } catch (e) {
    res.writeHead(400, { 'Content-Type': 'text/plain' });
    res.end('Invalid JSON');
    return;
  }

  const schemaError = validateRequestSchema(state);
  if (schemaError !== null) {
    res.writeHead(400, { 'Content-Type': 'text/plain' });
    res.end(`Invalid request schema: ${schemaError}`);
    return;
  }

  if (state.match_id !== matchId || state.turn !== turn) {
    res.writeHead(401, { 'Content-Type': 'text/plain' });
    res.end('Invalid request identity');
    return;
  }

  // Compute moves
  const moves = strategy.computeMoves(state);

  console.log(`Turn ${turn}: ${moves.length} moves computed`);

  // Build response
  const response: MoveResponse = { moves };
  const responseBody = JSON.stringify(response);

  // Sign response
  const responseSig = signResponse(SECRET, matchId, turn, responseBody);

  res.writeHead(200, {
    'Content-Type': 'application/json',
    'X-ACB-Signature': responseSig,
  });
  res.end(responseBody);
}

function getHeader(req: http.IncomingMessage, name: string): string {
  const value = req.headers[name];
  return typeof value === 'string' ? value : '';
}

function parseTurn(turn: string): number | null {
  if (!/^(0|[1-9]\d*)$/.test(turn)) return null;
  const value = Number(turn);
  return Number.isSafeInteger(value) ? value : null;
}

/**
 * Verify HMAC signature of incoming request
 */
function verifySignature(
  secret: string,
  matchId: string,
  turn: string,
  timestamp: string,
  body: Buffer,
  signature: string
): boolean {
  // The contract requires exactly 64 lowercase hex characters; a
  // case-insensitive test would also accept uppercase.
  if (!/^[0-9a-f]{64}$/.test(signature)) return false;
  const bodyHash = crypto.createHash('sha256').update(body).digest('hex');
  const signingString = `${matchId}.${turn}.${timestamp}.${bodyHash}`;
  const expected = crypto.createHmac('sha256', secret).update(signingString).digest('hex');
  const provided = Buffer.from(signature, 'hex');
  const expectedBytes = Buffer.from(expected, 'hex');
  return provided.length === expectedBytes.length && crypto.timingSafeEqual(provided, expectedBytes);
}

function verifyTimestamp(timestamp: string): boolean {
  const seconds = Number(timestamp);
  return (
    /^(0|[1-9]\d*)$/.test(timestamp) &&
    Number.isSafeInteger(seconds) &&
    Math.abs(Date.now() / 1000 - seconds) <= 30
  );
}

/**
 * Sign response body
 */
function signResponse(secret: string, matchId: string, turn: number, body: string): string {
  const bodyHash = crypto.createHash('sha256').update(body).digest('hex');
  const signingString = `${matchId}.${turn}.${bodyHash}`;
  return crypto.createHmac('sha256', secret).update(signingString).digest('hex');
}

server.listen(PORT, '0.0.0.0', () => {
  console.log(`SwarmBot starting on port ${PORT}`);
});
