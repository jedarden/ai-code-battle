// AI Code Battle Worker API
// Phase 4: Match Orchestration

import type { Env, ApiResponse, ClaimJobRequest, SubmitResultRequest, CreateBotRequest } from './types';
import { handleCron } from './cron';
import {
  getNextJob,
  claimJob,
  heartbeatJob,
  submitResult,
  failJob,
} from './jobs';
import {
  registerBot,
  listBots,
  getBot,
  updateBot,
  rotateApiKey,
  getLeaderboard,
} from './bots';

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    const url = new URL(request.url);
    const path = url.pathname;
    const method = request.method;

    // CORS headers
    const corsHeaders = {
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type, Authorization, X-API-Key',
    };

    // Handle preflight
    if (method === 'OPTIONS') {
      return new Response(null, { headers: corsHeaders });
    }

    // Helper for JSON responses
    const json = <T>(data: ApiResponse<T>, status = 200): Response => {
      return new Response(JSON.stringify(data), {
        status,
        headers: {
          'Content-Type': 'application/json',
          ...corsHeaders,
        },
      });
    };

    // Helper to verify API key
    const verifyApiKey = async (): Promise<boolean> => {
      const apiKey = request.headers.get('X-API-Key');
      if (!apiKey) return false;
      return apiKey === env.API_KEY;
    };

    // Helper to parse JSON body
    const parseBody = async <T>(): Promise<T | null> => {
      try {
        return await request.json<T>();
      } catch {
        return null;
      }
    };

    try {
      // Health check
      if (path === '/health' || path === '/api/health') {
        return json({ success: true, data: { status: 'healthy' } });
      }

      // ============ Job Endpoints (require API key) ============

      if (path === '/api/jobs/next' && method === 'GET') {
        if (!(await verifyApiKey())) {
          return json({ success: false, error: 'Unauthorized' }, 401);
        }
        const result = await getNextJob(env);
        return json(result);
      }

      if (path.match(/^\/api\/jobs\/[^/]+\/claim$/) && method === 'POST') {
        if (!(await verifyApiKey())) {
          return json({ success: false, error: 'Unauthorized' }, 401);
        }
        const jobId = path.split('/')[3];
        const body = await parseBody<ClaimJobRequest>();
        if (!body?.worker_id) {
          return json({ success: false, error: 'Missing worker_id' }, 400);
        }
        const result = await claimJob(env, jobId, body.worker_id);
        return json(result, result.success ? 200 : 400);
      }

      if (path.match(/^\/api\/jobs\/[^/]+\/heartbeat$/) && method === 'POST') {
        if (!(await verifyApiKey())) {
          return json({ success: false, error: 'Unauthorized' }, 401);
        }
        const jobId = path.split('/')[3];
        const body = await parseBody<{ worker_id: string }>();
        if (!body?.worker_id) {
          return json({ success: false, error: 'Missing worker_id' }, 400);
        }
        const result = await heartbeatJob(env, jobId, body.worker_id);
        return json(result, result.success ? 200 : 400);
      }

      if (path.match(/^\/api\/jobs\/[^/]+\/result$/) && method === 'POST') {
        if (!(await verifyApiKey())) {
          return json({ success: false, error: 'Unauthorized' }, 401);
        }
        const jobId = path.split('/')[3];
        const body = await parseBody<SubmitResultRequest>();
        if (!body) {
          return json({ success: false, error: 'Invalid request body' }, 400);
        }
        const result = await submitResult(env, jobId, body);
        return json(result, result.success ? 200 : 400);
      }

      if (path.match(/^\/api\/jobs\/[^/]+\/fail$/) && method === 'POST') {
        if (!(await verifyApiKey())) {
          return json({ success: false, error: 'Unauthorized' }, 401);
        }
        const jobId = path.split('/')[3];
        const body = await parseBody<{ worker_id: string; error_message: string }>();
        if (!body?.worker_id || !body?.error_message) {
          return json({ success: false, error: 'Missing required fields' }, 400);
        }
        const result = await failJob(env, jobId, body.worker_id, body.error_message);
        return json(result, result.success ? 200 : 400);
      }

      // ============ Bot Endpoints (public or owner-verified) ============

      if (path === '/api/register' && method === 'POST') {
        const body = await parseBody<CreateBotRequest>();
        if (!body) {
          return json({ success: false, error: 'Invalid request body' }, 400);
        }
        const result = await registerBot(env, body);
        return json(result, result.success ? 201 : 400);
      }

      if (path === '/api/bots' && method === 'GET') {
        const result = await listBots(env);
        return json(result);
      }

      if (path.match(/^\/api\/bots\/[^/]+$/) && method === 'GET') {
        const botId = path.split('/')[3];
        const result = await getBot(env, botId);
        return json(result, result.success ? 200 : 404);
      }

      if (path.match(/^\/api\/bots\/[^/]+$/) && method === 'PUT') {
        const botId = path.split('/')[3];
        const body = await parseBody<{ name?: string; endpoint_url?: string }>();
        if (!body) {
          return json({ success: false, error: 'Invalid request body' }, 400);
        }
        const result = await updateBot(env, botId, body);
        return json(result, result.success ? 200 : 400);
      }

      if (path === '/api/rotate-key' && method === 'POST') {
        const body = await parseBody<{ bot_id: string; owner_id: string }>();
        if (!body?.bot_id || !body?.owner_id) {
          return json({ success: false, error: 'Missing required fields' }, 400);
        }
        const result = await rotateApiKey(env, body.bot_id, body.owner_id);
        return json(result, result.success ? 200 : 400);
      }

      if (path === '/api/leaderboard' && method === 'GET') {
        const result = await getLeaderboard(env);
        return json(result);
      }

      // 404 for unmatched routes
      return json({ success: false, error: 'Not found' }, 404);
    } catch (error) {
      console.error('Worker error:', error);
      return json(
        { success: false, error: 'Internal server error' },
        500
      );
    }
  },

  async scheduled(event: ScheduledEvent, env: Env, ctx: ExecutionContext): Promise<void> {
    const cron = event.cron;
    console.log(`Running scheduled task: ${cron}`);

    try {
      const result = await handleCron(env, cron);
      console.log(`Cron result:`, result);
    } catch (error) {
      console.error(`Cron error:`, error);
    }
  },
};
