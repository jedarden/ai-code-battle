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

  if (
    state === null ||
    typeof state !== 'object' ||
    state.match_id !== matchId ||
    state.turn !== turn
  ) {
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
  if (!/^[0-9a-fA-F]{64}$/.test(signature)) return false;
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
