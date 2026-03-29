// Playlist Generator - Auto-curated replay collections
import type {
  ExportMatch,
  ExportBot,
  Playlist,
  PlaylistCategory,
  PlaylistMatch,
  PlaylistSummary,
  PlaylistIndex,
} from './types.js';

export class PlaylistGenerator {
  private matches: ExportMatch[];
  private bots: ExportBot[];
  private botNameMap: Map<string, string>;
  private now: string;

  constructor(matches: ExportMatch[], bots: ExportBot[]) {
    this.matches = matches.filter(m => m.status === 'completed');
    this.bots = bots;
    this.botNameMap = new Map(bots.map(b => [b.id, b.name]));
    this.now = new Date().toISOString();
  }

  /**
   * Generate all playlists
   */
  generateAll(): Playlist[] {
    return [
      this.generateFeaturedPlaylist(),
      this.generateUpsetsPlaylist(),
      this.generateComebacksPlaylist(),
      this.generateDominationPlaylist(),
      this.generateCloseGamesPlaylist(),
      this.generateLongGamesPlaylist(),
      this.generateWeeklyBestPlaylist(),
    ].filter((p): p is Playlist => p !== null && p.matches.length > 0);
  }

  /**
   * Generate playlist index
   */
  generateIndex(playlists: Playlist[]): PlaylistIndex {
    return {
      updated_at: this.now,
      playlists: playlists.map(p => ({
        slug: p.slug,
        title: p.title,
        description: p.description,
        category: p.category,
        match_count: p.match_count,
        thumbnail_match_id: p.matches[0]?.match_id,
      })),
    };
  }

  /**
   * Featured matches - high-rated bot confrontations
   */
  private generateFeaturedPlaylist(): Playlist {
    const botRatingMap = new Map(this.bots.map(b => [b.id, b.rating]));
    const featured = this.matches
      .filter(m => {
        // Only 2-player matches between high-rated bots
        if (m.participants.length !== 2) return false;
        const ratings = m.participants.map(p => botRatingMap.get(p.bot_id) || 0);
        return ratings.every(r => r > 1600);
      })
      .sort((a, b) => (b.completed_at || '').localeCompare(a.completed_at || ''))
      .slice(0, 10)
      .map((m, i) => this.matchToPlaylistEntry(m, i));

    return {
      slug: 'featured',
      title: 'Featured Matches',
      description: 'High-level confrontations between top-rated bots',
      category: 'featured',
      match_count: featured.length,
      created_at: this.now,
      updated_at: this.now,
      matches: featured,
    };
  }

  /**
   * Upsets - lower-rated bot beats higher-rated opponent
   */
  private generateUpsetsPlaylist(): Playlist {
    const botRatingMap = new Map(this.bots.map(b => [b.id, b.rating]));
    const upsets = this.matches
      .filter(m => {
        if (m.participants.length !== 2 || !m.winner_id) return false;
        const winnerRating = botRatingMap.get(m.winner_id) || 1500;
        const loserId = m.participants.find(p => p.bot_id !== m.winner_id)?.bot_id;
        if (!loserId) return false;
        const loserRating = botRatingMap.get(loserId) || 1500;
        // Upset: winner was at least 100 points lower rated
        return winnerRating < loserRating - 100;
      })
      .sort((a, b) => {
        // Sort by upset magnitude (largest first)
        const aMag = this.getUpsetMagnitude(a, botRatingMap);
        const bMag = this.getUpsetMagnitude(b, botRatingMap);
        return bMag - aMag;
      })
      .slice(0, 10)
      .map((m, i) => this.matchToPlaylistEntry(m, i));

    return {
      slug: 'upsets',
      title: 'Epic Upsets',
      description: 'Unexpected victories where underdogs triumphed',
      category: 'upsets',
      match_count: upsets.length,
      created_at: this.now,
      updated_at: this.now,
      matches: upsets,
    };
  }

  private getUpsetMagnitude(match: ExportMatch, ratingMap: Map<string, number>): number {
    if (!match.winner_id) return 0;
    const winnerRating = ratingMap.get(match.winner_id) || 1500;
    const loserId = match.participants.find(p => p.bot_id !== match.winner_id)?.bot_id;
    if (!loserId) return 0;
    const loserRating = ratingMap.get(loserId) || 1500;
    return loserRating - winnerRating;
  }

  /**
   * Comebacks - matches with large score swings
   */
  private generateComebacksPlaylist(): Playlist {
    // Comebacks are hard to detect without turn-by-turn data
    // For now, use close final scores as a proxy
    const closeMatches = this.matches
      .filter(m => m.participants.length === 2)
      .filter(m => {
        const scores = m.participants.map(p => p.score);
        const diff = Math.abs(scores[0] - scores[1]);
        // Close game: score difference <= 2
        return diff <= 2 && diff > 0;
      })
      .sort((a, b) => (b.completed_at || '').localeCompare(a.completed_at || ''))
      .slice(0, 10)
      .map((m, i) => this.matchToPlaylistEntry(m, i));

    return {
      slug: 'comebacks',
      title: 'Epic Comebacks',
      description: 'Matches where fortunes shifted dramatically',
      category: 'comebacks',
      match_count: closeMatches.length,
      created_at: this.now,
      updated_at: this.now,
      matches: closeMatches,
    };
  }

  /**
   * Domination - massive score differences
   */
  private generateDominationPlaylist(): Playlist {
    const dominated = this.matches
      .filter(m => m.participants.length === 2)
      .filter(m => {
        const scores = m.participants.map(p => p.score);
        const diff = Math.abs(scores[0] - scores[1]);
        // Domination: score difference >= 5
        return diff >= 5;
      })
      .sort((a, b) => {
        // Sort by domination magnitude
        const aDiff = Math.abs(a.participants[0].score - a.participants[1].score);
        const bDiff = Math.abs(b.participants[0].score - b.participants[1].score);
        return bDiff - aDiff;
      })
      .slice(0, 10)
      .map((m, i) => this.matchToPlaylistEntry(m, i));

    return {
      slug: 'domination',
      title: 'Total Domination',
      description: 'One-sided victories with massive score differences',
      category: 'domination',
      match_count: dominated.length,
      created_at: this.now,
      updated_at: this.now,
      matches: dominated,
    };
  }

  /**
   * Close games - decided by a single point
   */
  private generateCloseGamesPlaylist(): Playlist {
    const close = this.matches
      .filter(m => m.participants.length === 2)
      .filter(m => {
        const scores = m.participants.map(p => p.score);
        const diff = Math.abs(scores[0] - scores[1]);
        return diff === 1;
      })
      .sort((a, b) => (b.completed_at || '').localeCompare(a.completed_at || ''))
      .slice(0, 10)
      .map((m, i) => this.matchToPlaylistEntry(m, i));

    return {
      slug: 'close-games',
      title: 'Photo Finishes',
      description: 'Matches decided by the thinnest of margins',
      category: 'close_games',
      match_count: close.length,
      created_at: this.now,
      updated_at: this.now,
      matches: close,
    };
  }

  /**
   * Long games - high turn counts
   */
  private generateLongGamesPlaylist(): Playlist {
    const longGames = this.matches
      .filter(m => (m.turns || 0) >= 300)
      .sort((a, b) => (b.turns || 0) - (a.turns || 0))
      .slice(0, 10)
      .map((m, i) => this.matchToPlaylistEntry(m, i));

    return {
      slug: 'long-games',
      title: 'Marathon Matches',
      description: 'Extended battles that went the distance',
      category: 'long_games',
      match_count: longGames.length,
      created_at: this.now,
      updated_at: this.now,
      matches: longGames,
    };
  }

  /**
   * Weekly best - most recent week's top matches
   */
  private generateWeeklyBestPlaylist(): Playlist {
    const oneWeekAgo = new Date();
    oneWeekAgo.setDate(oneWeekAgo.getDate() - 7);
    const weekStart = oneWeekAgo.toISOString().split('T')[0];

    const weeklyMatches = this.matches
      .filter(m => (m.completed_at || '') >= weekStart)
      .sort((a, b) => (b.completed_at || '').localeCompare(a.completed_at || ''))
      .slice(0, 15)
      .map((m, i) => this.matchToPlaylistEntry(m, i));

    // Generate title with date range
    const now = new Date();
    const weekEndStr = now.toISOString().split('T')[0];

    return {
      slug: 'weekly-best',
      title: `Best of the Week (${weekStart} to ${weekEndStr})`,
      description: 'Top matches from the past 7 days',
      category: 'weekly',
      match_count: weeklyMatches.length,
      created_at: this.now,
      updated_at: this.now,
      matches: weeklyMatches,
    };
  }

  /**
   * Convert a match to a playlist entry
   */
  private matchToPlaylistEntry(match: ExportMatch, order: number): PlaylistMatch {
    const winnerName = match.winner_id ? this.botNameMap.get(match.winner_id) : 'Draw';
    const participants = match.participants
      .map(p => this.botNameMap.get(p.bot_id) || 'Unknown')
      .join(' vs ');

    return {
      match_id: match.id,
      order,
      title: `${participants} - ${winnerName} wins`,
      thumbnail_url: `https://r2.aicodebattle.com/thumbnails/${match.id}.png`,
    };
  }
}
