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

// Evolution dashboard types (re-exported from types.ts for convenience)
export type {
  LiveJSON,
  EvolutionIslandStat as IslandStat,
  EvolutionParentInfo as ParentInfo,
  EvolutionStageResult as StageResult,
  EvolutionValidationStatus as ValidationStatus,
  EvaluationMatchResult as MatchResult,
  EvolutionEvaluationStatus as EvaluationStatus,
  EvolutionCandidate as Candidate,
  EvolutionCycleInfo as CycleInfo,
  EvolutionActivityEntry as ActivityEntry,
  EvolutionTotals as Totals,
  EvolutionGenerationEntry as GenerationEntry,
  EvolutionLineageNode as LineageNode,
  EvolutionMetaSnapshot as MetaSnapshot,
} from './types';

// Convenience alias: the full live.json document
import type { LiveJSON } from './types';
export type EvolutionLiveData = LiveJSON;

// Full island stat (legacy format, not in live.json schema)
export interface IslandStatFull {
  count: number;
  best_fitness: number;
  avg_fitness: number;
  diversity: number;
  promoted_count: number;
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
  type: 'meta-report' | 'chronicle';
  summary: string;
  body_markdown: string;
  tags: string[];
}

export interface BlogIndex {
  updated_at: string;
  posts: BlogPost[];
}

// API configuration
export const API_BASE = '/api';

// ─── Stale-while-revalidate cache ─────────────────────────────────────────────────
// Returns cached data instantly (if available) while fetching fresh data in the
// background.  On the next call the fresh data is already in cache.  This gives
// sub-ms render times for repeat visits while keeping data reasonably current.

interface CacheEntry<T> { data: T; ts: number }

const swrCache = new Map<string, CacheEntry<unknown>>();
const SWR_MAX_AGE = 5 * 60 * 1000; // 5 min — data is served from cache without re-fetch

function swr<T>(key: string, fetcher: () => Promise<T>): Promise<T> {
  const cached = swrCache.get(key) as CacheEntry<T> | undefined;

  if (cached) {
    // Serve stale immediately, revalidate in background if older than max-age
    if (Date.now() - cached.ts > SWR_MAX_AGE) {
      fetcher().then(data => swrCache.set(key, { data, ts: Date.now() })).catch(() => {});
    }
    return Promise.resolve(cached.data);
  }

  // No cache — fetch and cache
  return fetcher().then(data => {
    swrCache.set(key, { data, ts: Date.now() });
    return data;
  });
}

// API client functions
export async function fetchLeaderboard(): Promise<LeaderboardIndex> {
  return swr('leaderboard', async () => {
    const response = await fetch('/data/leaderboard.json');
    if (!response.ok) throw new Error(`Failed to fetch leaderboard: ${response.status}`);
    return response.json();
  });
}

export async function fetchBotDirectory(): Promise<BotDirectory> {
  return swr('bot-directory', async () => {
    const response = await fetch('/data/bots/index.json');
    if (!response.ok) throw new Error(`Failed to fetch bot directory: ${response.status}`);
    return response.json();
  });
}

export async function fetchBotProfile(botId: string): Promise<BotProfile> {
  return swr(`bot-${botId}`, async () => {
    const response = await fetch(`/data/bots/${botId}.json`);
    if (!response.ok) throw new Error(`Failed to fetch bot profile: ${response.status}`);
    return response.json();
  });
}

export async function fetchMatchIndex(): Promise<MatchIndex> {
  return swr('match-index', async () => {
    const response = await fetch('/data/matches/index.json');
    if (!response.ok) throw new Error(`Failed to fetch match index: ${response.status}`);
    return response.json();
  });
}

export async function registerBot(request: RegisterRequest): Promise<RegisterResponse> {
  const response = await fetch(`${API_BASE}/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  return response.json();
}

// R2_BASE_URL is the Cloudflare R2 bucket custom domain for live data.
// The evolver writes live.json here every cycle with Cache-Control: max-age=10.
const R2_BASE_URL = 'https://r2.aicodebattle.com';

export async function fetchEvolutionData(): Promise<EvolutionLiveData> {
  // Evolution data changes every ~10s — bypass SWR, always fetch fresh
  const response = await fetch(`${R2_BASE_URL}/evolution/live.json`);
  if (!response.ok) throw new Error(`Failed to fetch evolution data: ${response.status}`);
  return response.json();
}

export async function fetchBlogIndex(): Promise<BlogIndex> {
  return swr('blog-index', async () => {
    const response = await fetch('/data/blog/index.json');
    if (!response.ok) throw new Error(`Failed to fetch blog index: ${response.status}`);
    return response.json();
  });
}

export async function fetchBlogPost(slug: string): Promise<BlogPost> {
  return swr(`blog-${slug}`, async () => {
    const response = await fetch(`/data/blog/${slug}.json`);
    if (!response.ok) throw new Error(`Failed to fetch blog post: ${response.status}`);
    return response.json();
  });
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
  curation_tag?: string;
  participants?: { bot_id: string; name: string; score: number; won: boolean }[];
  score?: string;
  turns?: number;
  end_reason?: string;
  completed_at?: string;
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
  return swr('playlist-index', async () => {
    const response = await fetch('/data/playlists/index.json');
    if (!response.ok) throw new Error(`Failed to fetch playlist index: ${response.status}`);
    return response.json();
  });
}

export async function fetchPlaylist(slug: string): Promise<Playlist> {
  return swr(`playlist-${slug}`, async () => {
    const response = await fetch(`/data/playlists/${slug}.json`);
    if (!response.ok) throw new Error(`Failed to fetch playlist: ${response.status}`);
    return response.json();
  });
}

// Prediction types

export interface PredictionData {
  id: number;
  match_id: string;
  predictor_id: string;
  predicted_bot: string;
  correct?: boolean;
  created_at: string;
  resolved_at?: string;
}

export interface PredictionHistoryEntry {
  id: number;
  match_id: string;
  predicted_bot: string;
  predicted_name: string;
  correct: boolean | null;
  confidence?: number;
  created_at: string;
  resolved_at?: string;
  match_status: string;
  winner_name?: string;
}

export async function fetchPredictionHistory(predictorId: string, limit?: number): Promise<{ predictions: PredictionHistoryEntry[] }> {
  const params = new URLSearchParams({ predictor_id: predictorId });
  if (limit) params.set('limit', String(limit));
  const response = await fetch(`/api/predictions/history?${params}`);
  if (!response.ok) throw new Error(`Failed to fetch prediction history: ${response.status}`);
  return response.json();
}

export interface PredictorStats {
  predictor_id: string;
  correct: number;
  incorrect: number;
  streak: number;
  best_streak: number;
}

export interface PredictionsLeaderboard {
  updated_at: string;
  entries: PredictorStats[];
}

export async function fetchPredictionsLeaderboard(): Promise<PredictionsLeaderboard> {
  return swr('predictions-leaderboard', async () => {
    const response = await fetch('/data/predictions/leaderboard.json');
    if (!response.ok) throw new Error(`Failed to fetch predictions leaderboard: ${response.status}`);
    return response.json();
  });
}

export interface OpenMatch {
  match_id: string;
  created_at: string;
  participants: { bot_id: string; name: string; rating: number }[];
  your_pick?: string;
}

export interface OpenPredictionsResponse {
  matches: OpenMatch[];
}

export async function fetchOpenPredictions(predictorId?: string): Promise<OpenPredictionsResponse> {
  const params = predictorId ? `?predictor_id=${encodeURIComponent(predictorId)}` : '';
  const response = await fetch(`/api/predictions/open${params}`);
  if (!response.ok) throw new Error(`Failed to fetch open predictions: ${response.status}`);
  return response.json();
}

export async function submitPrediction(matchId: string, botId: string, predictorId: string): Promise<{ id: number }> {
  const response = await fetch('/api/predict', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ match_id: matchId, bot_id: botId, predictor_id: predictorId }),
  });
  if (!response.ok) {
    const err = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(err.error || `Failed to submit prediction: ${response.status}`);
  }
  return response.json();
}

export function getOrCreatePredictorId(): string {
  let id = localStorage.getItem('acb_predictor_id');
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem('acb_predictor_id', id);
  }
  return id;
}

// Evolution meta types for homepage
export interface EvolutionMeta {
  generation: number;
  promoted_today: number;
  top_10_count: number;
  updated_at: string;
}

export async function fetchEvolutionMeta(): Promise<EvolutionMeta> {
  return swr('evolution-meta', async () => {
    const response = await fetch('/data/evolution/meta.json');
    if (!response.ok) {
      return { generation: 0, promoted_today: 0, top_10_count: 0, updated_at: '' };
    }
    return response.json();
  });
}

// Season types (re-export from types.ts for convenience)
import type { SeasonIndex } from './types';
export type { Season, SeasonIndex } from './types';

export async function fetchSeasonIndex(): Promise<SeasonIndex> {
  return swr('season-index', async () => {
    const response = await fetch('/data/seasons/index.json');
    if (!response.ok) {
      return { updated_at: '', active_season: null, seasons: [] };
    }
    return response.json();
  });
}

// Commentary / Enrichment types (§13.3)

export interface CommentaryEntry {
  turn: number;
  text: string;
  type: 'setup' | 'action' | 'reaction' | 'climax' | 'denouement';
}

export interface EnrichedCommentary {
  match_id: string;
  generated_at: string;
  criteria: string[];
  entries: CommentaryEntry[];
}

export interface EnrichedMatchEntry {
  match_id: string;
  criteria: string[];
}

export interface EnrichedIndex {
  updated_at: string;
  entries: EnrichedMatchEntry[];
}

const R2_COMMENTARY_BASE = 'https://r2.aicodebattle.com';

export async function fetchCommentary(matchId: string): Promise<EnrichedCommentary | null> {
  try {
    const response = await fetch(`${R2_COMMENTARY_BASE}/commentary/${matchId}.json`);
    if (!response.ok) return null;
    return response.json();
  } catch {
    return null;
  }
}

export async function fetchEnrichedIndex(): Promise<EnrichedIndex> {
  return swr('enriched-index', async () => {
    const response = await fetch('/data/commentary/index.json');
    if (!response.ok) return { updated_at: '', entries: [] };
    return response.json();
  });
}
