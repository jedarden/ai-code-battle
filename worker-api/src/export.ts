// Data Export Endpoint for Index Builder

import type { Env, Bot, Match, MatchParticipant, ApiResponse } from './types';

/**
 * Export data for index building.
 * This endpoint is called by the Rackspace index builder every ~90 minutes.
 * It returns all data needed to generate the index JSON files.
 */
export interface ExportData {
  bots: ExportBot[];
  matches: ExportMatch[];
  rating_history: RatingHistoryEntry[];
  generated_at: string;
}

export interface ExportBot {
  id: string;
  name: string;
  owner_id: string;
  rating: number;
  rating_deviation: number;
  rating_volatility: number;
  matches_played: number;
  matches_won: number;
  created_at: string;
  updated_at: string;
  health_status: string;
}

export interface ExportMatch {
  id: string;
  status: string;
  winner_id: string | null;
  turns: number | null;
  end_reason: string | null;
  map_id: string;
  created_at: string;
  completed_at: string | null;
  participants: ExportMatchParticipant[];
}

export interface ExportMatchParticipant {
  bot_id: string;
  player_index: number;
  score: number;
  rating_before: number;
  rating_after: number | null;
}

export interface RatingHistoryEntry {
  bot_id: string;
  rating: number;
  rating_deviation: number;
  recorded_at: string;
}

/**
 * GET /api/data/export - Export all data for index building
 */
export async function exportData(env: Env): Promise<ApiResponse<ExportData>> {
  const now = new Date().toISOString();

  // Fetch all bots
  const botsResult = await env.DB.prepare(
    `SELECT
      id, name, owner_id, rating, rating_deviation, rating_volatility,
      matches_played, matches_won, created_at, updated_at, health_status
    FROM bots
    ORDER BY rating DESC`
  ).all<ExportBot>();

  // Fetch recent matches (last 1000 completed)
  const matchesResult = await env.DB.prepare(
    `SELECT id, status, winner_id, turns, end_reason, map_id, created_at, completed_at
    FROM matches
    WHERE status = 'completed'
    ORDER BY completed_at DESC
    LIMIT 1000`
  ).all<Match>();

  // Fetch match participants for all matches
  const matchIds = matchesResult.results.map(m => m.id);
  let participants: MatchParticipant[] = [];

  if (matchIds.length > 0) {
    // Build query with proper parameter binding
    const placeholders = matchIds.map(() => '?').join(',');
    const participantsResult = await env.DB.prepare(
      `SELECT bot_id, match_id, player_index, score, rating_before, rating_after
      FROM match_participants
      WHERE match_id IN (${placeholders})`
    ).bind(...matchIds).all<MatchParticipant>();

    participants = participantsResult.results || [];
  }

  // Group participants by match_id
  const participantsByMatch = new Map<string, MatchParticipant[]>();
  for (const p of participants) {
    if (!participantsByMatch.has(p.match_id)) {
      participantsByMatch.set(p.match_id, []);
    }
    participantsByMatch.get(p.match_id)!.push(p);
  }

  // Build export matches with embedded participants
  const exportMatches: ExportMatch[] = matchesResult.results.map(m => ({
    id: m.id,
    status: m.status,
    winner_id: m.winner_id,
    turns: m.turns,
    end_reason: m.end_reason,
    map_id: m.map_id,
    created_at: m.created_at,
    completed_at: m.completed_at,
    participants: (participantsByMatch.get(m.id) || []).map(p => ({
      bot_id: p.bot_id,
      player_index: p.player_index,
      score: p.score,
      rating_before: p.rating_before,
      rating_after: p.rating_after,
    })),
  }));

  // Fetch rating history (last 30 days)
  const thirtyDaysAgo = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString();
  const ratingHistoryResult = await env.DB.prepare(
    `SELECT bot_id, rating, rating_deviation, recorded_at
    FROM rating_history
    WHERE recorded_at >= ?
    ORDER BY bot_id, recorded_at ASC`
  )
    .bind(thirtyDaysAgo)
    .all<RatingHistoryEntry>();

  return {
    success: true,
    data: {
      bots: botsResult.results || [],
      matches: exportMatches,
      rating_history: ratingHistoryResult.results || [],
      generated_at: now,
    },
  };
}
