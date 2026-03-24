// Job Coordination Endpoints

import type { Env, Job, Match, MatchParticipant, JobClaimResponse, ApiResponse, SubmitResultRequest } from './types';
import { updateMatchRatings } from './glicko2';

/**
 * GET /api/jobs/next - Get next available job for worker
 */
export async function getNextJob(env: Env): Promise<ApiResponse<Job | null>> {
  // Find a pending job, ordered by creation time
  const result = await env.DB.prepare(
    `SELECT * FROM jobs
     WHERE status = 'pending'
     ORDER BY created_at ASC
     LIMIT 1`
  ).first<Job>();

  return { success: true, data: result || null };
}

/**
 * POST /api/jobs/:id/claim - Claim a job for execution
 */
export async function claimJob(
  env: Env,
  jobId: string,
  workerId: string
): Promise<ApiResponse<JobClaimResponse>> {
  const now = new Date().toISOString();

  // Try to claim the job atomically
  const result = await env.DB.prepare(
    `UPDATE jobs SET
      status = 'claimed',
      worker_id = ?,
      claimed_at = ?,
      heartbeat_at = ?
    WHERE id = ? AND status = 'pending'`
  )
    .bind(workerId, now, now, jobId)
    .run();

  if (result.meta.changes === 0) {
    return { success: false, error: 'Job not found or already claimed' };
  }

  // Get the job details
  const job = await env.DB.prepare('SELECT * FROM jobs WHERE id = ?')
    .bind(jobId)
    .first<Job>();

  if (!job) {
    return { success: false, error: 'Job not found' };
  }

  // Get match details
  const match = await env.DB.prepare('SELECT * FROM matches WHERE id = ?')
    .bind(job.match_id)
    .first<Match>();

  if (!match) {
    return { success: false, error: 'Match not found' };
  }

  // Update match status to running
  await env.DB.prepare(
    `UPDATE matches SET status = 'running', started_at = ? WHERE id = ?`
  )
    .bind(now, match.id)
    .run();

  // Get participants with their ratings
  const participants = await env.DB.prepare(
    `SELECT * FROM match_participants WHERE match_id = ?`
  )
    .bind(match.id)
    .all<MatchParticipant>();

  // Get bot details (endpoint URLs)
  const botIds = participants.results.map((p) => p.bot_id);
  const placeholders = botIds.map(() => '?').join(',');
  const bots = await env.DB.prepare(
    `SELECT id, endpoint_url FROM bots WHERE id IN (${placeholders})`
  )
    .bind(...botIds)
    .all<{ id: string; endpoint_url: string }>();

  // Get bot secrets (API keys for HMAC auth)
  const secrets = await env.DB.prepare(
    `SELECT bot_id, api_key_hash as secret FROM bot_secrets WHERE bot_id IN (${placeholders})`
  )
    .bind(...botIds)
    .all<{ bot_id: string; secret: string }>();

  // Get map details
  const map = await env.DB.prepare('SELECT * FROM maps WHERE id = ?')
    .bind(match.map_id)
    .first<{ id: string; width: number; height: number; walls: string; spawns: string; cores: string }>();

  if (!map) {
    return { success: false, error: 'Map not found' };
  }

  return {
    success: true,
    data: {
      job: job,
      match: match,
      participants: participants.results,
      map: map,
      bots: bots.results,
      bot_secrets: secrets.results,
    },
  };
}

/**
 * POST /api/jobs/:id/heartbeat - Update job heartbeat
 */
export async function heartbeatJob(
  env: Env,
  jobId: string,
  workerId: string
): Promise<ApiResponse<void>> {
  const now = new Date().toISOString();

  const result = await env.DB.prepare(
    `UPDATE jobs SET heartbeat_at = ? WHERE id = ? AND worker_id = ?`
  )
    .bind(now, jobId, workerId)
    .run();

  if (result.meta.changes === 0) {
    return { success: false, error: 'Job not found or not owned by worker' };
  }

  return { success: true };
}

/**
 * POST /api/jobs/:id/result - Submit job result
 */
export async function submitResult(
  env: Env,
  jobId: string,
  result: SubmitResultRequest
): Promise<ApiResponse<void>> {
  const now = new Date().toISOString();

  // Get the job
  const job = await env.DB.prepare('SELECT * FROM jobs WHERE id = ?')
    .bind(jobId)
    .first<Job>();

  if (!job) {
    return { success: false, error: 'Job not found' };
  }

  if (job.status !== 'claimed' && job.status !== 'running') {
    return { success: false, error: 'Job not in a valid state for result submission' };
  }

  // Get participants
  const participants = await env.DB.prepare(
    'SELECT * FROM match_participants WHERE match_id = ?'
  )
    .bind(job.match_id)
    .all<MatchParticipant>();

  // Update scores
  for (const [botId, score] of Object.entries(result.scores)) {
    await env.DB.prepare(
      `UPDATE match_participants SET score = ? WHERE match_id = ? AND bot_id = ?`
    )
      .bind(score, job.match_id, botId)
      .run();
  }

  // Update ratings using Glicko-2
  await updateMatchRatings(env, job.match_id, participants.results, result.winner_id);

  // Update job status
  await env.DB.prepare(
    `UPDATE jobs SET status = 'completed', completed_at = ? WHERE id = ?`
  )
    .bind(now, jobId)
    .run();

  // Update match status
  await env.DB.prepare(
    `UPDATE matches SET
      status = 'completed',
      winner_id = ?,
      turns = ?,
      end_reason = ?,
      completed_at = ?
    WHERE id = ?`
  )
    .bind(result.winner_id, result.turns, result.end_reason, now, job.match_id)
    .run();

  return { success: true };
}

/**
 * POST /api/jobs/:id/fail - Mark job as failed
 */
export async function failJob(
  env: Env,
  jobId: string,
  workerId: string,
  errorMessage: string
): Promise<ApiResponse<void>> {
  const now = new Date().toISOString();

  const result = await env.DB.prepare(
    `UPDATE jobs SET
      status = 'failed',
      completed_at = ?,
      error_message = ?
    WHERE id = ? AND worker_id = ?`
  )
    .bind(now, errorMessage, jobId, workerId)
    .run();

  if (result.meta.changes === 0) {
    return { success: false, error: 'Job not found or not owned by worker' };
  }

  // Also update match status
  const job = await env.DB.prepare('SELECT match_id FROM jobs WHERE id = ?')
    .bind(jobId)
    .first<{ match_id: string }>();

  if (job) {
    await env.DB.prepare(
      `UPDATE matches SET status = 'failed', completed_at = ? WHERE id = ?`
    )
      .bind(now, job.match_id)
      .run();
  }

  return { success: true };
}
