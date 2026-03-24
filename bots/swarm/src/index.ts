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
  const matchId = req.headers['x-acb-match-id'] as string;
  const turnStr = req.headers['x-acb-turn'] as string;
  const timestamp = req.headers['x-acb-timestamp'] as string;
  const signature = req.headers['x-acb-signature'] as string;

  if (!matchId || !turnStr || !timestamp || !signature) {
    res.writeHead(401, { 'Content-Type': 'text/plain' });
    res.end('Missing auth headers');
    return;
  }

  // Read body
  let body = '';
  for await (const chunk of req) {
    body += chunk;
  }

  // Verify signature
  if (!verifySignature(SECRET, matchId, turnStr, timestamp, body, signature)) {
    res.writeHead(401, { 'Content-Type': 'text/plain' });
    res.end('Invalid signature');
    return;
  }

  // Parse game state
  let state: GameState;
  try {
    state = JSON.parse(body);
  } catch (e) {
    res.writeHead(400, { 'Content-Type': 'text/plain' });
    res.end('Invalid JSON');
    return;
  }

  // Compute moves
  const moves = strategy.computeMoves(state);
  const turn = parseInt(turnStr, 10);

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

/**
 * Verify HMAC signature of incoming request
 */
function verifySignature(
  secret: string,
  matchId: string,
  turn: string,
  timestamp: string,
  body: string,
  signature: string
): boolean {
  const bodyHash = crypto.createHash('sha256').update(body).digest('hex');
  const signingString = `${matchId}.${turn}.${timestamp}.${bodyHash}`;
  const expected = crypto.createHmac('sha256', secret).update(signingString).digest('hex');
  return crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(expected));
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
