// Home page - dynamic landing page with live data
import {
  fetchLeaderboard,
  fetchBlogIndex,
  fetchPlaylistIndex,
  fetchEvolutionMeta,
  fetchSeasonIndex,
  fetchMatchIndex,
  fetchEnrichedIndex,
  type Season,
  type MatchSummary
} from '../api-types';

const PAGES_BASE = '';

// Stale-while-revalidate cache
interface CacheEntry<T> {
  data: T;
  timestamp: number;
}

const cache = new Map<string, CacheEntry<unknown>>();
const CACHE_TTL = 5 * 60 * 1000; // 5 minutes

async function fetchWithCache<T>(
  key: string,
  fetcher: () => Promise<T>,
  defaultValue: T
): Promise<T> {
  const cached = cache.get(key) as CacheEntry<T> | undefined;
  const now = Date.now();

  if (cached && now - cached.timestamp < CACHE_TTL) {
    // Stale: return cached data immediately
    fetcher().then(data => {
      cache.set(key, { data, timestamp: now });
      // Trigger re-render with fresh data
      requestAnimationFrame(() => renderHomePage());
    }).catch(() => {
      // Silently fail on background refresh
    });
    return cached.data;
  }

  // No cache or expired: fetch fresh data
  try {
    const data = await fetcher();
    cache.set(key, { data, timestamp: now });
    return data;
  } catch {
    return defaultValue;
  }
}

// Find featured replay — prefer enriched/AI-commentary matches, then most recent
async function findFeaturedReplay(matches: MatchSummary[]): Promise<{ match: MatchSummary | null; enriched: boolean }> {
  const completed = matches.filter(m => m.completed_at && m.participants.length >= 2);
  if (completed.length === 0) return { match: null, enriched: false };

  // Sort by most recent first
  const sorted = [...completed].sort((a, b) =>
    new Date(b.completed_at!).getTime() - new Date(a.completed_at!).getTime()
  );

  // Try to find an enriched match among recent replays
  try {
    const enrichedIndex = await fetchEnrichedIndex();
    const enrichedIDs = new Set(enrichedIndex.entries.map(e => e.match_id));
    const enrichedMatch = sorted.find(m => enrichedIDs.has(m.id));
    if (enrichedMatch) {
      return { match: enrichedMatch, enriched: true };
    }
  } catch {
    // enriched index not available — fall through
  }

  return { match: sorted[0], enriched: false };
}

// Format time remaining
function formatTimeRemaining(endDate: string | null): string {
  if (!endDate) return '';
  const now = Date.now();
  const end = new Date(endDate).getTime();
  const diff = end - now;

  if (diff <= 0) return 'Ending soon';

  const days = Math.floor(diff / (1000 * 60 * 60 * 24));
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));

  if (days > 0) return `${days} day${days === 1 ? '' : 's'} remaining`;
  if (hours > 0) return `${hours} hour${hours === 1 ? '' : 's'} remaining`;
  return 'Less than an hour';
}

// Get current week of season
function getSeasonProgress(season: Season | null): { week: number; totalWeeks: number; timeRemaining: string } | null {
  if (!season || season.status !== 'active') return null;
  // Simple calculation - in production this would come from season data
  const start = new Date(season.starts_at).getTime();
  season.ends_at ? new Date(season.ends_at).getTime() : start + (4 * 7 * 24 * 60 * 60 * 1000);
  const now = Date.now();
  const totalWeeks = 4;
  const week = Math.min(Math.floor((now - start) / (7 * 24 * 60 * 60 * 1000)) + 1, totalWeeks);
  return { week, totalWeeks, timeRemaining: formatTimeRemaining(season.ends_at) };
}

export async function renderHomePage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  // Fetch all data in parallel
  const [
    leaderboardData,
    blogData,
    playlistsData,
    evolutionMeta,
    seasonData,
    matchesData
  ] = await Promise.all([
    fetchWithCache('leaderboard', fetchLeaderboard, { updated_at: '', entries: [] }),
    fetchWithCache('blog', fetchBlogIndex, { updated_at: '', posts: [] }),
    fetchWithCache('playlists', fetchPlaylistIndex, { updated_at: '', playlists: [] }),
    fetchWithCache('evolution', fetchEvolutionMeta, { generation: 0, promoted_today: 0, top_10_count: 0, updated_at: '' }),
    fetchWithCache('seasons', fetchSeasonIndex, { updated_at: '', active_season: null, seasons: [] }),
    fetchWithCache('matches', fetchMatchIndex, { updated_at: '', matches: [] })
  ]);

  const top5 = leaderboardData.entries.slice(0, 5);
  const latestStories = blogData.posts.slice(0, 3);
  const featuredPlaylists = playlistsData.playlists.slice(0, 6);
  const { match: featuredReplay } = await findFeaturedReplay(matchesData.matches);
  const activeSeason = seasonData.active_season;
  const seasonProgress = getSeasonProgress(activeSeason);

  app.innerHTML = `
    <div class="home-page">
      <!-- Hero Section -->
      <section class="hero">
        <h1>AI Code Battle</h1>
        <p class="tagline">Bots compete. Strategies evolve. You watch.</p>
        <div class="cta-buttons">
          <a href="#/watch/replays" class="btn primary">Watch Battles</a>
          <a href="#/compete/register" class="btn secondary">Build a Bot</a>
        </div>
      </section>

      <!-- Featured Replay -->
      ${featuredReplay ? `
      <section class="featured-replay">
        <div class="replay-embed" id="featured-replay-embed">
          <iframe
            src="${PAGES_BASE}/embed.html?match_id=${featuredReplay.id}&autoplay=true&speed=150&loop=true"
            frameborder="0"
            allowfullscreen
            loading="lazy"
          ></iframe>
        </div>
        <div class="replay-info">
          <p class="replay-title">
            ${featuredReplay.participants.map(p => `<strong>${escapeHtml(p.name)}</strong>`).join(' vs ')}
            ${featuredReplay.winner_id ? ` — Winner: <strong>${escapeHtml(featuredReplay.participants.find(p => p.bot_id === featuredReplay.winner_id)?.name || 'Unknown')}</strong>` : ''}
          </p>
          <a href="#/watch/replay?url=/replays/${featuredReplay.id}.json" class="btn small secondary">Watch Full Replay →</a>
        </div>
      </section>
      ` : ''}

      <!-- Two-column: Top 5 + Latest Stories -->
      <section class="home-grid">
        <!-- Top 5 Leaderboard -->
        <div class="card leaderboard-summary">
          <h2>Top 5 Bots</h2>
          <div class="leaderboard-list">
            ${top5.length > 0 ? top5.map((entry: any, i: number) => `
              <div class="leaderboard-row rank-${i + 1}">
                <span class="rank">#${entry.rank}</span>
                <a href="#/bot/${entry.bot_id}" class="bot-name">${escapeHtml(entry.name)}</a>
                <span class="rating">${entry.rating}</span>
              </div>
            `).join('') : '<p class="empty">No bots yet</p>'}
          </div>
          <a href="#/leaderboard" class="btn small secondary">Full leaderboard →</a>
        </div>

        <!-- Latest Stories -->
        <div class="card stories-summary">
          <h2>Latest Stories</h2>
          <div class="stories-list">
            ${latestStories.length > 0 ? latestStories.map((post: any) => `
              <a href="#/blog/${post.slug}" class="story-link">
                <div class="story-title">${escapeHtml(post.title)}</div>
                <div class="story-meta">${post.published_at || post.date || ''}</div>
              </a>
            `).join('') : '<p class="empty">No stories yet</p>'}
          </div>
          <a href="#/blog" class="btn small secondary">All stories →</a>
        </div>
      </section>

      <!-- Playlists Carousel -->
      ${featuredPlaylists.length > 0 ? `
      <section class="playlists-section">
        <h2>Playlists</h2>
        <div class="playlists-carousel">
          ${featuredPlaylists.map((playlist: any) => `
            <a href="#/watch/playlists/${playlist.slug}" class="playlist-card">
              <div class="playlist-thumbnail">
                ${playlist.thumbnail_match_id
                  ? `<img src="/replays/${playlist.thumbnail_match_id}.jpg" alt="${escapeHtml(playlist.title)}" loading="lazy">`
                  : `<div class="thumbnail-placeholder">⚔️</div>`
                }
              </div>
              <div class="playlist-info">
                <div class="playlist-title">${escapeHtml(playlist.title)}</div>
                <div class="playlist-count">${playlist.match_count} matches</div>
              </div>
            </a>
          `).join('')}
        </div>
      </section>
      ` : ''}

      <!-- Season Status Bar -->
      ${activeSeason && seasonProgress ? `
      <section class="season-bar">
        <div class="season-info">
          <span class="season-name">${escapeHtml(activeSeason.name)}</span>
          <span class="season-progress">Week ${seasonProgress.week} of ${seasonProgress.totalWeeks}</span>
          <span class="season-time">${seasonProgress.timeRemaining}</span>
        </div>
        <a href="#/watch/predictions" class="btn small primary">Predictions Open →</a>
      </section>
      ` : ''}

      <!-- Evolution Observatory Mini -->
      <section class="evolution-mini">
        <div class="evolution-info">
          <span class="evolution-icon">🧬</span>
          <span class="evolution-text">
            <strong>Evolution Observatory</strong> — Gen #${evolutionMeta.generation}
            ${evolutionMeta.promoted_today > 0 ? ` · ${evolutionMeta.promoted_today} promoted today` : ''}
            ${evolutionMeta.top_10_count > 0 ? ` · ${evolutionMeta.top_10_count} in top 10` : ''}
          </span>
        </div>
        <a href="#/evolution" class="btn small secondary">Watch evolution live →</a>
      </section>
    </div>

    <style>
      .home-page {
        max-width: 1200px;
        margin: 0 auto;
      }

      /* Hero */
      .hero {
        text-align: center;
        padding: 40px 20px;
        background: linear-gradient(135deg, var(--bg-secondary) 0%, var(--bg-primary) 100%);
        border-radius: 12px;
        margin-bottom: 24px;
      }

      .hero h1 {
        font-size: 2.5rem;
        color: var(--text-primary);
        margin-bottom: 8px;
      }

      .hero .tagline {
        font-size: 1.25rem;
        color: var(--accent);
        margin-bottom: 24px;
      }

      .cta-buttons {
        display: flex;
        gap: 12px;
        justify-content: center;
      }

      /* Featured Replay */
      .featured-replay {
        background-color: var(--bg-secondary);
        border-radius: 12px;
        overflow: hidden;
        margin-bottom: 24px;
      }

      .replay-embed {
        position: relative;
        width: 100%;
        aspect-ratio: 16 / 9;
        background-color: #000;
      }

      .replay-embed iframe {
        width: 100%;
        height: 100%;
      }

      .replay-info {
        padding: 12px 16px;
        display: flex;
        justify-content: space-between;
        align-items: center;
      }

      .replay-title {
        color: var(--text-primary);
        font-size: 0.875rem;
      }

      /* Two-column grid */
      .home-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 20px;
        margin-bottom: 24px;
      }

      .card {
        background-color: var(--bg-secondary);
        border-radius: 12px;
        padding: 20px;
      }

      .card h2 {
        font-size: 1.125rem;
        color: var(--text-primary);
        margin-bottom: 16px;
      }

      /* Leaderboard summary */
      .leaderboard-row {
        display: flex;
        gap: 12px;
        padding: 8px 0;
        border-bottom: 1px solid var(--border);
      }

      .leaderboard-row:last-child {
        border-bottom: none;
      }

      .leaderboard-row .rank {
        width: 32px;
        font-weight: 700;
        color: var(--text-muted);
      }

      .leaderboard-row.rank-1 .rank { color: #fbbf24; }
      .leaderboard-row.rank-2 .rank { color: #94a3b8; }
      .leaderboard-row.rank-3 .rank { color: #b45309; }

      .leaderboard-row .bot-name {
        flex: 1;
        color: var(--text-secondary);
        text-decoration: none;
      }

      .leaderboard-row .bot-name:hover {
        color: var(--accent);
      }

      .leaderboard-row .rating {
        font-weight: 600;
        color: var(--text-primary);
      }

      /* Stories */
      .stories-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
      }

      .story-link {
        display: block;
        text-decoration: none;
        padding: 8px 0;
      }

      .story-title {
        color: var(--text-secondary);
        margin-bottom: 4px;
      }

      .story-link:hover .story-title {
        color: var(--accent);
      }

      .story-meta {
        font-size: 0.75rem;
        color: var(--text-muted);
      }

      /* Playlists carousel */
      .playlists-section h2 {
        font-size: 1.125rem;
        color: var(--text-primary);
        margin-bottom: 16px;
      }

      .playlists-carousel {
        display: flex;
        gap: 16px;
        overflow-x: auto;
        padding-bottom: 8px;
        scrollbar-width: thin;
        scrollbar-color: var(--border) var(--bg-secondary);
      }

      .playlists-carousel::-webkit-scrollbar {
        height: 6px;
      }

      .playlists-carousel::-webkit-scrollbar-track {
        background: var(--bg-secondary);
        border-radius: 3px;
      }

      .playlists-carousel::-webkit-scrollbar-thumb {
        background: var(--border);
        border-radius: 3px;
      }

      .playlist-card {
        flex-shrink: 0;
        width: 180px;
        background-color: var(--bg-secondary);
        border-radius: 8px;
        overflow: hidden;
        text-decoration: none;
        transition: transform 0.2s;
      }

      .playlist-card:hover {
        transform: translateY(-2px);
      }

      .playlist-thumbnail {
        width: 100%;
        aspect-ratio: 16 / 9;
        background-color: var(--bg-tertiary);
        display: flex;
        align-items: center;
        justify-content: center;
      }

      .playlist-thumbnail img {
        width: 100%;
        height: 100%;
        object-fit: cover;
      }

      .thumbnail-placeholder {
        font-size: 2rem;
        color: var(--text-muted);
      }

      .playlist-info {
        padding: 10px;
      }

      .playlist-title {
        font-size: 0.875rem;
        color: var(--text-secondary);
        margin-bottom: 4px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      .playlist-count {
        font-size: 0.75rem;
        color: var(--text-muted);
      }

      /* Season bar */
      .season-bar {
        background: linear-gradient(135deg, var(--accent) 0%, var(--accent-hover) 100%);
        border-radius: 12px;
        padding: 16px 20px;
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 16px;
      }

      .season-info {
        display: flex;
        flex-wrap: wrap;
        gap: 12px;
        align-items: center;
      }

      .season-name {
        font-weight: 700;
        color: white;
        font-size: 1.125rem;
      }

      .season-progress,
      .season-time {
        color: rgba(255, 255, 255, 0.9);
        font-size: 0.875rem;
      }

      .season-bar .btn {
        background-color: white;
        color: var(--accent);
      }

      .season-bar .btn:hover {
        background-color: #f1f5f9;
      }

      /* Evolution mini */
      .evolution-mini {
        background-color: var(--bg-secondary);
        border-radius: 12px;
        padding: 12px 16px;
        display: flex;
        justify-content: space-between;
        align-items: center;
      }

      .evolution-info {
        display: flex;
        align-items: center;
        gap: 10px;
      }

      .evolution-icon {
        font-size: 1.5rem;
      }

      .evolution-text {
        color: var(--text-secondary);
        font-size: 0.875rem;
      }

      .evolution-text strong {
        color: var(--text-primary);
      }

      .empty {
        color: var(--text-muted);
        text-align: center;
        padding: 20px 0;
      }

      /* Responsive */
      @media (max-width: 768px) {
        .home-grid {
          grid-template-columns: 1fr;
        }

        .hero h1 {
          font-size: 1.75rem;
        }

        .hero .tagline {
          font-size: 1rem;
        }

        .season-bar {
          flex-direction: column;
          gap: 12px;
          text-align: center;
        }

        .season-info {
          flex-direction: column;
          gap: 4px;
        }
      }
    </style>
  `;
}

function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}
