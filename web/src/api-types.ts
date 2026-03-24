// API response types matching the Worker API and index builder

// Leaderboard types
export interface LeaderboardEntry {
  rank: number;
  bot_id: string;
  name: string;
  owner_id: string;
  rating: number;
  rating_deviation: number;
  matches_played: number;
  matches_won: number;
  win_rate: number;
  health_status: string;
}

export interface LeaderboardIndex {
  updated_at: string;
  entries: LeaderboardEntry[];
}

// Bot profile types
export interface RatingHistoryEntry {
  bot_id: string;
  rating: number;
  rating_deviation: number;
  recorded_at: string;
}

export interface MatchSummaryParticipant {
  bot_id: string;
  name: string;
  score: number;
  won: boolean;
}

export interface MatchSummary {
  id: string;
  completed_at: string | null;
  participants: MatchSummaryParticipant[];
  winner_id: string | null;
  turns: number | null;
  end_reason: string | null;
}

export interface BotProfile {
  id: string;
  name: string;
  owner_id: string;
  rating: number;
  rating_deviation: number;
  rating_volatility: number;
  matches_played: number;
  matches_won: number;
  win_rate: number;
  health_status: string;
  created_at: string;
  updated_at: string;
  rating_history: RatingHistoryEntry[];
  recent_matches: MatchSummary[];
}

export interface BotDirectoryEntry {
  id: string;
  name: string;
  rating: number;
  matches_played: number;
  win_rate: number;
}

export interface BotDirectory {
  updated_at: string;
  bots: BotDirectoryEntry[];
}

// Match index types
export interface MatchIndex {
  updated_at: string;
  matches: MatchSummary[];
}

// Registration types
export interface RegisterRequest {
  name: string;
  endpoint_url: string;
  owner_id: string;
}

export interface RegisterResponse {
  success: boolean;
  bot_id?: string;
  api_key?: string;
  error?: string;
}

// API configuration
export const API_BASE = '/api';

// API client functions
export async function fetchLeaderboard(): Promise<LeaderboardIndex> {
  const response = await fetch('/data/leaderboard.json');
  if (!response.ok) throw new Error(`Failed to fetch leaderboard: ${response.status}`);
  return response.json();
}

export async function fetchBotDirectory(): Promise<BotDirectory> {
  const response = await fetch('/data/bots/index.json');
  if (!response.ok) throw new Error(`Failed to fetch bot directory: ${response.status}`);
  return response.json();
}

export async function fetchBotProfile(botId: string): Promise<BotProfile> {
  const response = await fetch(`/data/bots/${botId}.json`);
  if (!response.ok) throw new Error(`Failed to fetch bot profile: ${response.status}`);
  return response.json();
}

export async function fetchMatchIndex(): Promise<MatchIndex> {
  const response = await fetch('/data/matches/index.json');
  if (!response.ok) throw new Error(`Failed to fetch match index: ${response.status}`);
  return response.json();
}

export async function registerBot(request: RegisterRequest): Promise<RegisterResponse> {
  const response = await fetch(`${API_BASE}/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  return response.json();
}

export async function rotateApiKey(botId: string, currentKey: string): Promise<RegisterResponse> {
  const response = await fetch(`${API_BASE}/rotate-key`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${currentKey}`,
    },
    body: JSON.stringify({ bot_id: botId }),
  });
  return response.json();
}
