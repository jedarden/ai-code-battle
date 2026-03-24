// Bot Management Endpoints

import type { Env, Bot, CreateBotRequest, ApiResponse } from './types';

/**
 * Generate a random API key (256-bit, hex-encoded)
 */
function generateApiKey(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

/**
 * Hash an API key for storage
 */
async function hashApiKey(key: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(key);
  const hash = await crypto.subtle.digest('SHA-256', data);
  return Array.from(new Uint8Array(hash))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

/**
 * POST /api/register - Register a new bot
 */
export async function registerBot(
  env: Env,
  request: CreateBotRequest
): Promise<ApiResponse<{ id: string; api_key: string }>> {
  // Validate request
  if (!request.name || !request.owner_id || !request.endpoint_url) {
    return { success: false, error: 'Missing required fields' };
  }

  // Validate endpoint URL
  try {
    new URL(request.endpoint_url);
  } catch {
    return { success: false, error: 'Invalid endpoint URL' };
  }

  const botId = crypto.randomUUID();
  const apiKey = generateApiKey();
  const apiKeyHash = await hashApiKey(apiKey);
  const now = new Date().toISOString();

  // Check if owner already has a bot with this name
  const existing = await env.DB.prepare(
    'SELECT id FROM bots WHERE owner_id = ? AND name = ?'
  )
    .bind(request.owner_id, request.name)
    .first();

  if (existing) {
    return { success: false, error: 'Bot with this name already exists for this owner' };
  }

  // Create bot
  await env.DB.prepare(
    `INSERT INTO bots (id, name, owner_id, endpoint_url, api_key_hash, created_at, updated_at)
     VALUES (?, ?, ?, ?, ?, ?, ?)`
  )
    .bind(
      botId,
      request.name,
      request.owner_id,
      request.endpoint_url,
      apiKeyHash,
      now,
      now
    )
    .run();

  // Store API key hash separately
  await env.DB.prepare(
    `INSERT INTO bot_secrets (bot_id, api_key_hash, created_at)
     VALUES (?, ?, ?)`
  )
    .bind(botId, apiKeyHash, now)
    .run();

  return {
    success: true,
    data: {
      id: botId,
      api_key: apiKey, // Return the plain key only on creation
    },
  };
}

/**
 * GET /api/bots - List all bots
 */
export async function listBots(env: Env): Promise<ApiResponse<Bot[]>> {
  const result = await env.DB.prepare(
    `SELECT
      id, name, owner_id, endpoint_url, rating, rating_deviation, rating_volatility,
      created_at, updated_at, last_health_check, health_status, matches_played, matches_won
    FROM bots
    ORDER BY rating DESC`
  ).all<Bot>();

  // Remove sensitive fields
  const bots = (result.results || []).map((bot) => ({
    ...bot,
    api_key_hash: '',
  }));

  return { success: true, data: bots };
}

/**
 * GET /api/bots/:id - Get bot details
 */
export async function getBot(env: Env, botId: string): Promise<ApiResponse<Bot>> {
  const bot = await env.DB.prepare(
    `SELECT
      id, name, owner_id, endpoint_url, rating, rating_deviation, rating_volatility,
      created_at, updated_at, last_health_check, health_status, matches_played, matches_won
    FROM bots
    WHERE id = ?`
  )
    .bind(botId)
    .first<Bot>();

  if (!bot) {
    return { success: false, error: 'Bot not found' };
  }

  return { success: true, data: { ...bot, api_key_hash: '' } };
}

/**
 * PUT /api/bots/:id - Update bot details
 */
export async function updateBot(
  env: Env,
  botId: string,
  updates: { name?: string; endpoint_url?: string }
): Promise<ApiResponse<void>> {
  const now = new Date().toISOString();

  const setClauses: string[] = [];
  const values: unknown[] = [];

  if (updates.name) {
    setClauses.push('name = ?');
    values.push(updates.name);
  }

  if (updates.endpoint_url) {
    try {
      new URL(updates.endpoint_url);
      setClauses.push('endpoint_url = ?');
      values.push(updates.endpoint_url);
    } catch {
      return { success: false, error: 'Invalid endpoint URL' };
    }
  }

  if (setClauses.length === 0) {
    return { success: false, error: 'No valid updates provided' };
  }

  setClauses.push('updated_at = ?');
  values.push(now);
  values.push(botId);

  const result = await env.DB.prepare(
    `UPDATE bots SET ${setClauses.join(', ')} WHERE id = ?`
  )
    .bind(...values)
    .run();

  if (result.meta.changes === 0) {
    return { success: false, error: 'Bot not found' };
  }

  return { success: true };
}

/**
 * POST /api/rotate-key - Rotate bot API key
 */
export async function rotateApiKey(
  env: Env,
  botId: string,
  ownerId: string
): Promise<ApiResponse<{ api_key: string }>> {
  // Verify ownership
  const bot = await env.DB.prepare('SELECT owner_id FROM bots WHERE id = ?')
    .bind(botId)
    .first<{ owner_id: string }>();

  if (!bot) {
    return { success: false, error: 'Bot not found' };
  }

  if (bot.owner_id !== ownerId) {
    return { success: false, error: 'Not authorized' };
  }

  const newApiKey = generateApiKey();
  const apiKeyHash = await hashApiKey(newApiKey);
  const now = new Date().toISOString();

  // Update bot
  await env.DB.prepare('UPDATE bots SET api_key_hash = ?, updated_at = ? WHERE id = ?')
    .bind(apiKeyHash, now, botId)
    .run();

  // Update secret
  await env.DB.prepare('UPDATE bot_secrets SET api_key_hash = ? WHERE bot_id = ?')
    .bind(apiKeyHash, botId)
    .run();

  return { success: true, data: { api_key: newApiKey } };
}

/**
 * GET /api/leaderboard - Get current leaderboard
 */
export async function getLeaderboard(env: Env): Promise<ApiResponse<Bot[]>> {
  const result = await env.DB.prepare(
    `SELECT
      id, name, owner_id, rating, rating_deviation, matches_played, matches_won,
      created_at, updated_at, health_status
    FROM bots
    WHERE matches_played > 0
    ORDER BY rating DESC
    LIMIT 100`
  ).all<Bot>();

  return { success: true, data: result.results || [] };
}
