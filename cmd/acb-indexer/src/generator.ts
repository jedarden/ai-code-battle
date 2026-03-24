// Index Generator - Creates static JSON index files

import type {
  ExportData,
  ExportBot,
  ExportMatch,
  LeaderboardIndex,
  LeaderboardEntry,
  BotDirectory,
  BotDirectoryEntry,
  BotProfile,
  MatchIndex,
  MatchSummary,
} from './types.js';

export class IndexGenerator {
  private data: ExportData;
  private botNameMap: Map<string, string>;

  constructor(data: ExportData) {
    this.data = data;
    this.botNameMap = new Map(data.bots.map(b => [b.id, b.name]));
  }

  /**
   * Generate leaderboard.json
   */
  generateLeaderboard(): LeaderboardIndex {
    const entries: LeaderboardEntry[] = this.data.bots
      .filter(bot => bot.matches_played > 0)
      .map((bot, index) => ({
        rank: index + 1,
        bot_id: bot.id,
        name: bot.name,
        owner_id: bot.owner_id,
        rating: Math.round(bot.rating),
        rating_deviation: Math.round(bot.rating_deviation * 10) / 10,
        matches_played: bot.matches_played,
        matches_won: bot.matches_won,
        win_rate: bot.matches_played > 0
          ? Math.round((bot.matches_won / bot.matches_played) * 1000) / 10
          : 0,
        health_status: bot.health_status,
      }));

    return {
      updated_at: this.data.generated_at,
      entries,
    };
  }

  /**
   * Generate bots/index.json - bot directory
   */
  generateBotDirectory(): BotDirectory {
    const bots: BotDirectoryEntry[] = this.data.bots.map(bot => ({
      id: bot.id,
      name: bot.name,
      rating: Math.round(bot.rating),
      matches_played: bot.matches_played,
      win_rate: bot.matches_played > 0
        ? Math.round((bot.matches_won / bot.matches_played) * 1000) / 10
        : 0,
    }));

    return {
      updated_at: this.data.generated_at,
      bots,
    };
  }

  /**
   * Generate individual bot profile
   */
  generateBotProfile(botId: string): BotProfile | null {
    const bot = this.data.bots.find(b => b.id === botId);
    if (!bot) return null;

    // Get rating history for this bot
    const ratingHistory = this.data.rating_history
      .filter(h => h.bot_id === botId)
      .sort((a, b) => a.recorded_at.localeCompare(b.recorded_at));

    // Get recent matches for this bot (last 20)
    const recentMatches = this.data.matches
      .filter(m => m.participants.some(p => p.bot_id === botId))
      .slice(0, 20)
      .map(m => this.generateMatchSummary(m));

    return {
      id: bot.id,
      name: bot.name,
      owner_id: bot.owner_id,
      rating: Math.round(bot.rating),
      rating_deviation: Math.round(bot.rating_deviation * 10) / 10,
      rating_volatility: Math.round(bot.rating_volatility * 10000) / 10000,
      matches_played: bot.matches_played,
      matches_won: bot.matches_won,
      win_rate: bot.matches_played > 0
        ? Math.round((bot.matches_won / bot.matches_played) * 1000) / 10
        : 0,
      health_status: bot.health_status,
      created_at: bot.created_at,
      updated_at: bot.updated_at,
      rating_history: ratingHistory,
      recent_matches: recentMatches,
    };
  }

  /**
   * Generate matches/index.json - recent match list
   */
  generateMatchIndex(): MatchIndex {
    const matches = this.data.matches.map(m => this.generateMatchSummary(m));

    return {
      updated_at: this.data.generated_at,
      matches,
    };
  }

  /**
   * Generate match summary for a single match
   */
  private generateMatchSummary(match: ExportMatch): MatchSummary {
    return {
      id: match.id,
      completed_at: match.completed_at,
      participants: match.participants.map(p => ({
        bot_id: p.bot_id,
        name: this.botNameMap.get(p.bot_id) || 'Unknown',
        score: p.score,
        won: p.bot_id === match.winner_id,
      })),
      winner_id: match.winner_id,
      turns: match.turns,
      end_reason: match.end_reason,
    };
  }

  /**
   * Generate all index files
   */
  generateAll(): {
    leaderboard: LeaderboardIndex;
    botDirectory: BotDirectory;
    botProfiles: Map<string, BotProfile>;
    matchIndex: MatchIndex;
  } {
    const botProfiles = new Map<string, BotProfile>();

    for (const bot of this.data.bots) {
      const profile = this.generateBotProfile(bot.id);
      if (profile) {
        botProfiles.set(bot.id, profile);
      }
    }

    return {
      leaderboard: this.generateLeaderboard(),
      botDirectory: this.generateBotDirectory(),
      botProfiles,
      matchIndex: this.generateMatchIndex(),
    };
  }
}
