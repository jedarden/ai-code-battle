// Index Builder Types

export interface ApiClientConfig {
  apiUrl: string;
  apiKey: string;
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

export interface ExportData {
  bots: ExportBot[];
  matches: ExportMatch[];
  rating_history: RatingHistoryEntry[];
  generated_at: string;
}

// Generated Index Types

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

export interface MatchSummary {
  id: string;
  completed_at: string | null;
  participants: {
    bot_id: string;
    name: string;
    score: number;
    won: boolean;
  }[];
  winner_id: string | null;
  turns: number | null;
  end_reason: string | null;
}

export interface MatchIndex {
  updated_at: string;
  matches: MatchSummary[];
}

// Blog / Narrative Engine types

export interface BlogPost {
  slug: string;
  title: string;
  published_at: string; // ISO 8601 date
  week_start: string;   // ISO 8601 date (Monday of the covered week)
  summary: string;      // one-paragraph plain-text teaser
  body_html: string;    // full HTML narrative content
  stats: BlogWeekStats;
}

export interface BlogWeekStats {
  matches_played: number;
  top_bot: string;
  top_bot_rating: number;
  biggest_upset: string | null; // "BotA defeated BotB" or null
  most_active_bot: string;
  most_active_bot_matches: number;
  island_leader: string | null; // leading evolution island
}

export interface BlogIndex {
  updated_at: string;
  posts: BlogPost[];
}

// Evolution dashboard types (written by acb-evolver live-export)
export interface EvolutionIslandStat {
  count: number;
  best_fitness: number;
  avg_fitness: number;
  diversity: number;
  promoted_count: number;
}

export interface EvolutionGenerationEntry {
  generation: number;
  island: string;
  evaluated_at: string;
  count: number;
  promoted: number;
  best_fitness: number;
  avg_fitness: number;
}

export interface EvolutionLineageNode {
  id: number;
  parent_ids: number[];
  generation: number;
  island: string;
  fitness: number;
  promoted: boolean;
  language: string;
  created_at: string;
}

export interface EvolutionMetaSnapshot {
  generation: number;
  island_counts: Record<string, number>;
  island_best_fitness: Record<string, number>;
}

export interface EvolutionLiveData {
  updated_at: string;
  total_programs: number;
  promoted_count: number;
  islands: Record<string, EvolutionIslandStat>;
  generation_log: EvolutionGenerationEntry[];
  lineage: EvolutionLineageNode[];
  meta_snapshots: EvolutionMetaSnapshot[];
}
