// AI Code Battle Worker Types

export interface Env {
  DB: D1Database;
  API_KEY: string;
  ENVIRONMENT: string;
}

// Bot types
export interface Bot {
  id: string;
  name: string;
  owner_id: string;
  endpoint_url: string;
  api_key_hash: string;
  rating: number;
  rating_deviation: number;
  rating_volatility: number;
  created_at: string;
  updated_at: string;
  last_health_check: string | null;
  health_status: 'healthy' | 'unhealthy' | 'unknown';
  matches_played: number;
  matches_won: number;
}

export interface CreateBotRequest {
  name: string;
  owner_id: string;
  endpoint_url: string;
}

// Match types
export type MatchStatus = 'pending' | 'running' | 'completed' | 'failed';

export interface Match {
  id: string;
  status: MatchStatus;
  winner_id: string | null;
  turns: number | null;
  end_reason: string | null;
  map_id: string;
  created_at: string;
  started_at: string | null;
  completed_at: string | null;
}

export interface MatchParticipant {
  id: string;
  match_id: string;
  bot_id: string;
  player_index: number;
  score: number;
  rating_before: number;
  rating_after: number | null;
  rating_deviation_before: number;
  rating_deviation_after: number | null;
}

// Job types
export type JobStatus = 'pending' | 'claimed' | 'running' | 'completed' | 'failed' | 'timeout';

export interface Job {
  id: string;
  match_id: string;
  status: JobStatus;
  worker_id: string | null;
  claimed_at: string | null;
  heartbeat_at: string | null;
  created_at: string;
  completed_at: string | null;
  error_message: string | null;
}

export interface ClaimJobRequest {
  worker_id: string;
}

export interface SubmitResultRequest {
  winner_id: string;
  turns: number;
  end_reason: string;
  replay_url: string;
  scores: Record<string, number>;
}

// Rating types
export interface RatingChange {
  bot_id: string;
  rating_before: number;
  rating_after: number;
  rating_deviation: number;
}

// API Response types
export interface ApiResponse<T = unknown> {
  success: boolean;
  data?: T;
  error?: string;
}

export interface JobClaimResponse {
  job: Job;
  match: Match;
  participants: MatchParticipant[];
  map: {
    id: string;
    width: number;
    height: number;
    walls: string;
    spawns: string;
    cores: string;
  };
  bots: Array<{
    id: string;
    endpoint_url: string;
  }>;
  bot_secrets: Array<{
    bot_id: string;
    secret: string;
  }>;
}
