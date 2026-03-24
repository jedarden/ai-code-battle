// Cron Job Handlers

import type { Env, Bot } from './types';

/**
 * Matchmaker cron: Create match jobs for bots that need games
 * Runs every minute
 */
export async function runMatchmaker(env: Env): Promise<{ created: number }> {
  const now = new Date().toISOString();

  // Get bots that are healthy and have played fewer than 10 matches today
  // For simplicity, we'll just pair bots randomly for now
  // A more sophisticated system would consider rating proximity

  // Get active bots (healthy, played at least one match or registered recently)
  const bots = await env.DB.prepare(
    `SELECT id, rating, matches_played FROM bots
     WHERE health_status = 'healthy'
     ORDER BY RANDOM()
     LIMIT 10`
  ).all<Bot>();

  if (!bots.results || bots.results.length < 2) {
    return { created: 0 };
  }

  // Get a random map
  const map = await env.DB.prepare(
    'SELECT id FROM maps ORDER BY RANDOM() LIMIT 1'
  ).first<{ id: string }>();

  if (!map) {
    return { created: 0 };
  }

  let created = 0;

  // Create matches in pairs
  for (let i = 0; i < bots.results.length - 1; i += 2) {
    const bot1 = bots.results[i];
    const bot2 = bots.results[i + 1];

    // Check if these bots already have a pending match together
    const existingMatch = await env.DB.prepare(
      `SELECT m.id FROM matches m
       JOIN match_participants mp1 ON m.id = mp1.match_id
       JOIN match_participants mp2 ON m.id = mp2.match_id
       WHERE m.status = 'pending'
       AND mp1.bot_id = ? AND mp2.bot_id = ?`
    )
      .bind(bot1.id, bot2.id)
      .first();

    if (existingMatch) {
      continue; // Skip this pair
    }

    // Create match
    const matchId = crypto.randomUUID();
    await env.DB.prepare(
      `INSERT INTO matches (id, status, map_id, created_at)
       VALUES (?, 'pending', ?, ?)`
    )
      .bind(matchId, map.id, now)
      .run();

    // Get bot ratings for participants
    const bot1Data = await env.DB.prepare(
      'SELECT rating, rating_deviation FROM bots WHERE id = ?'
    )
      .bind(bot1.id)
      .first<{ rating: number; rating_deviation: number }>();

    const bot2Data = await env.DB.prepare(
      'SELECT rating, rating_deviation FROM bots WHERE id = ?'
    )
      .bind(bot2.id)
      .first<{ rating: number; rating_deviation: number }>();

    if (!bot1Data || !bot2Data) continue;

    // Create participants (player_index 0 and 1)
    await env.DB.prepare(
      `INSERT INTO match_participants (id, match_id, bot_id, player_index, score, rating_before, rating_deviation_before)
       VALUES (?, ?, ?, 0, 0, ?, ?)`
    )
      .bind(crypto.randomUUID(), matchId, bot1.id, bot1Data.rating, bot1Data.rating_deviation)
      .run();

    await env.DB.prepare(
      `INSERT INTO match_participants (id, match_id, bot_id, player_index, score, rating_before, rating_deviation_before)
       VALUES (?, ?, ?, 1, 0, ?, ?)`
    )
      .bind(crypto.randomUUID(), matchId, bot2.id, bot2Data.rating, bot2Data.rating_deviation)
      .run();

    // Create job
    await env.DB.prepare(
      `INSERT INTO jobs (id, match_id, status, created_at)
       VALUES (?, ?, 'pending', ?)`
    )
      .bind(crypto.randomUUID(), matchId, now)
      .run();

    created++;
  }

  return { created };
}

/**
 * Health checker cron: Ping bot endpoints to check health
 * Runs every 15 minutes
 */
export async function runHealthChecker(env: Env): Promise<{ checked: number }> {
  const bots = await env.DB.prepare(
    `SELECT id, endpoint_url FROM bots WHERE health_status != 'unhealthy' OR last_health_check IS NULL`
  ).all<{ id: string; endpoint_url: string }>();

  let checked = 0;
  const now = new Date().toISOString();

  for (const bot of bots.results || []) {
    try {
      // Simple health check - just try to connect
      const response = await fetch(bot.endpoint_url, {
        method: 'GET',
        signal: AbortSignal.timeout(5000), // 5 second timeout
      });

      const status = response.ok ? 'healthy' : 'unhealthy';

      await env.DB.prepare(
        `UPDATE bots SET health_status = ?, last_health_check = ? WHERE id = ?`
      )
        .bind(status, now, bot.id)
        .run();

      checked++;
    } catch {
      // Connection failed
      await env.DB.prepare(
        `UPDATE bots SET health_status = 'unhealthy', last_health_check = ? WHERE id = ?`
      )
        .bind(now, bot.id)
        .run();

      checked++;
    }
  }

  return { checked };
}

/**
 * Stale job reaper: Reclaim jobs that have timed out
 * Runs every 5 minutes
 */
export async function runStaleJobReaper(env: Env): Promise<{ reclaimed: number }> {
  const now = new Date();
  const staleThreshold = new Date(now.getTime() - 5 * 60 * 1000); // 5 minutes ago
  const staleThresholdStr = staleThreshold.toISOString();

  // Find jobs that have been claimed but haven't had a heartbeat in 5 minutes
  const staleJobs = await env.DB.prepare(
    `SELECT id, match_id FROM jobs
     WHERE status = 'claimed'
     AND heartbeat_at < ?`
  )
    .bind(staleThresholdStr)
    .all<{ id: string; match_id: string }>();

  let reclaimed = 0;

  for (const job of staleJobs.results || []) {
    // Reset the job to pending so another worker can claim it
    await env.DB.prepare(
      `UPDATE jobs SET
        status = 'pending',
        worker_id = NULL,
        claimed_at = NULL,
        heartbeat_at = NULL
      WHERE id = ?`
    )
      .bind(job.id)
      .run();

    // Reset match status to pending
    await env.DB.prepare(
      `UPDATE matches SET status = 'pending', started_at = NULL WHERE id = ?`
    )
      .bind(job.match_id)
      .run();

    reclaimed++;
  }

  return { reclaimed };
}

/**
 * Dispatch cron handler based on event type
 */
export async function handleCron(
  env: Env,
  cron: string
): Promise<{ success: boolean; result: unknown }> {
  // Parse cron expression to determine which handler to run
  // */1 * * * * = matchmaker (every minute)
  // */5 * * * * = stale job reaper (every 5 minutes)
  // */15 * * * * = health checker (every 15 minutes)

  // The cron expression is passed, but we need to determine the type
  // For simplicity, we'll check the pattern
  if (cron === '*/1 * * * *' || cron.includes('*/1')) {
    const result = await runMatchmaker(env);
    return { success: true, result };
  } else if (cron === '*/5 * * * *' || cron.includes('*/5')) {
    const result = await runStaleJobReaper(env);
    return { success: true, result };
  } else if (cron === '*/15 * * * *' || cron.includes('*/15')) {
    const result = await runHealthChecker(env);
    return { success: true, result };
  }

  return { success: false, result: 'Unknown cron pattern' };
}
