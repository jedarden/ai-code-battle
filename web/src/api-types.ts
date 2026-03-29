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
  // Evolution fields (optional - only present for evolved bots)
  evolved?: boolean;
  island?: string;
  generation?: number;
  parent_ids?: string[];
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

// Evolution dashboard types
export interface IslandStat {
  count: number;
  best_fitness: number;
  avg_fitness: number;
  diversity: number;
  promoted_count: number;
}

export interface GenerationEntry {
  generation: number;
  island: string;
  evaluated_at: string;
  count: number;
  promoted: number;
  best_fitness: number;
  avg_fitness: number;
}

export interface LineageNode {
  id: number;
  parent_ids: number[];
  generation: number;
  island: string;
  fitness: number;
  promoted: boolean;
  language: string;
  created_at: string;
}

export interface MetaSnapshot {
  generation: number;
  island_counts: Record<string, number>;
  island_best_fitness: Record<string, number>;
}

export interface EvolutionLiveData {
  updated_at: string;
  total_programs: number;
  promoted_count: number;
  islands: Record<string, IslandStat>;
  generation_log: GenerationEntry[];
  lineage: LineageNode[];
  meta_snapshots: MetaSnapshot[];
}

// Blog / Narrative Engine types

export interface BlogWeekStats {
  matches_played: number;
  top_bot: string;
  top_bot_rating: number;
  biggest_upset: string | null;
  most_active_bot: string;
  most_active_bot_matches: number;
  island_leader: string | null;
}

export interface BlogPost {
  slug: string;
  title: string;
  published_at: string;
  week_start: string;
  summary: string;
  body_html: string;
  stats: BlogWeekStats;
}

export interface BlogIndex {
  updated_at: string;
  posts: BlogPost[];
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

export async function fetchEvolutionData(): Promise<EvolutionLiveData> {
  const response = await fetch('/data/evolution/live.json');
  if (!response.ok) throw new Error(`Failed to fetch evolution data: ${response.status}`);
  return response.json();
}

export async function fetchBlogIndex(): Promise<BlogIndex> {
  const response = await fetch('/data/blog/index.json');
  if (!response.ok) throw new Error(`Failed to fetch blog index: ${response.status}`);
  return response.json();
}

export async function fetchBlogPost(slug: string): Promise<BlogPost> {
  const response = await fetch(`/data/blog/${slug}.json`);
  if (!response.ok) throw new Error(`Failed to fetch blog post: ${response.status}`);
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

// Playlist types

export type PlaylistCategory =
  | 'featured'
  | 'rivalry'
  | 'upsets'
  | 'comebacks'
  | 'domination'
  | 'close_games'
  | 'long_games'
  | 'tutorial'
  | 'season'
  | 'weekly';

export interface PlaylistMatch {
  match_id: string;
  order: number;
  title?: string;
  thumbnail_url?: string;
}

export interface Playlist {
  slug: string;
  title: string;
  description: string;
  category: PlaylistCategory;
  match_count: number;
  created_at: string;
  updated_at: string;
  matches: PlaylistMatch[];
}

export interface PlaylistSummary {
  slug: string;
  title: string;
  description: string;
  category: PlaylistCategory;
  match_count: number;
  updated_at: string;
  thumbnail_match_id?: string;
}

export interface PlaylistIndex {
  updated_at: string;
  playlists: PlaylistSummary[];
}

export async function fetchPlaylistIndex(): Promise<PlaylistIndex> {
  const response = await fetch('/data/playlists/index.json');
  if (!response.ok) throw new Error(`Failed to fetch playlist index: ${response.status}`);
  return response.json();
}

export async function fetchPlaylist(slug: string): Promise<Playlist> {
  const response = await fetch(`/data/playlists/${slug}.json`);
  if (!response.ok) throw new Error(`Failed to fetch playlist: ${response.status}`);
  return response.json();
}
