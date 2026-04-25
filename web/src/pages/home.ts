// Home page — dynamic landing page per plan §16.3
// §16.15: below-the-fold sections (playlists, season, evolution) deferred
// via IntersectionObserver lazy sections to reduce initial DOM weight.
import {
  fetchLeaderboard,
  fetchBlogIndex,
  fetchPlaylistIndex,
  fetchEvolutionMeta,
  fetchSeasonIndex,
  fetchMatchIndex,
  fetchEnrichedIndex,
  type Season,
  type MatchSummary,
} from '../api-types';
import { initLazySections, lazySection } from '../lib/lazy-section';
// Featured replay selection: prefer enriched/AI-commentary matches, then most recent
async function findFeaturedReplay(
  matches: MatchSummary[],
): Promise<{ match: MatchSummary | null; enriched: boolean }> {
  const completed = matches.filter(
    (m) => m.completed_at && m.participants.length >= 2,
  );
  if (completed.length === 0) return { match: null, enriched: false };

  const sorted = [...completed].sort(
    (a, b) =>
      new Date(b.completed_at!).getTime() -
      new Date(a.completed_at!).getTime(),
  );

  try {
    const enrichedIndex = await fetchEnrichedIndex();
    const enrichedIDs = new Set(enrichedIndex.entries.map((e) => e.match_id));
    const enrichedMatch = sorted.find((m) => enrichedIDs.has(m.id));
    if (enrichedMatch) return { match: enrichedMatch, enriched: true };
  } catch {
    // enriched index not available
  }

  return { match: sorted[0], enriched: false };
}

function formatTimeRemaining(endDate: string | null): string {
  if (!endDate) return '';
  const diff = new Date(endDate).getTime() - Date.now();
  if (diff <= 0) return 'Ending soon';
  const days = Math.floor(diff / (86400000));
  const hours = Math.floor((diff % 86400000) / 3600000);
  if (days > 0) return `${days} day${days === 1 ? '' : 's'} remaining`;
  if (hours > 0) return `${hours} hour${hours === 1 ? '' : 's'} remaining`;
  return 'Less than an hour';
}

function getSeasonProgress(season: Season | null): {
  week: number;
  totalWeeks: number;
  timeRemaining: string;
} | null {
  if (!season || season.status !== 'active') return null;
  const start = new Date(season.starts_at).getTime();
  const now = Date.now();
  const totalWeeks = 4;
  const week = Math.min(
    Math.floor((now - start) / 604800000) + 1,
    totalWeeks,
  );
  return {
    week,
    totalWeeks,
    timeRemaining: formatTimeRemaining(season.ends_at),
  };
}

function esc(text: string): string {
  const d = document.createElement('div');
  d.textContent = text;
  return d.innerHTML;
}

function renderPlaylistCards(playlists: any[]): string {
  return playlists.map((pl: any) => `
      <a href="#/watch/playlists/${pl.slug}" class="home-pl-card">
        <div class="home-pl-thumb">
          ${pl.thumbnail_match_id
            ? `<img src="/replays/${pl.thumbnail_match_id}.jpg" alt="${esc(pl.title)}" loading="lazy">`
            : '<div class="home-pl-placeholder">&#9876;</div>'}
        </div>
        <div class="home-pl-info">
          <span class="home-pl-title">${esc(pl.title)}</span>
          <span class="home-pl-count">${pl.match_count} matches</span>
        </div>
      </a>`).join('');
}

export async function renderHomePage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  // Fetch all data sources in parallel (fetchers use shared SWR cache)
  const [
    leaderboardData,
    blogData,
    playlistsData,
    evolutionMeta,
    seasonData,
    matchesData,
  ] = await Promise.all([
    fetchLeaderboard().catch(() => ({ updated_at: '', entries: [] })),
    fetchBlogIndex().catch(() => ({ updated_at: '', posts: [] })),
    fetchPlaylistIndex().catch(() => ({
      updated_at: '',
      playlists: [],
    })),
    fetchEvolutionMeta().catch(() => ({
      generation: 0,
      promoted_today: 0,
      top_10_count: 0,
      updated_at: '',
    })),
    fetchSeasonIndex().catch(() => ({
      updated_at: '',
      active_season: null,
      seasons: [],
    })),
    fetchMatchIndex().catch(() => ({
      updated_at: '',
      matches: [],
      pagination: { page: 1, per_page: 50, total: 0 },
    })),
  ]);

  const top5 = (leaderboardData.entries || []).slice(0, 5);
  const latestStories = (blogData.posts || []).slice(0, 3);
  const featuredPlaylists = (playlistsData.playlists || []).slice(0, 8);
  const { match: featuredReplay } = await findFeaturedReplay(
    matchesData.matches || [],
  );
  const activeSeason = seasonData.active_season;
  const seasonProgress = getSeasonProgress(activeSeason);

  // Featured replay: use demo replay as fallback when no live matches
  const hasLiveReplay = !!featuredReplay;
  const replayEmbedSrc = hasLiveReplay
    ? `/embed.html?match_id=${featuredReplay!.id}&autoplay=true&speed=150&loop=true&view=influence`
    : '/embed.html?demo=true&autoplay=true&speed=150&loop=true&view=influence';
  const replayTitle = hasLiveReplay
    ? `${featuredReplay!.participants.map((p) => `<strong>${esc(p.name)}</strong>`).join(' vs ')}${featuredReplay!.winner_id ? ` — Winner: <strong>${esc(featuredReplay!.participants.find((p) => p.bot_id === featuredReplay!.winner_id)?.name || 'Unknown')}</strong>` : ''}`
    : 'Demo Replay — Watch a sample battle';
  const replayLink = hasLiveReplay
    ? `#/watch/replay?url=/replays/${featuredReplay!.id}.json.gz`
    : '#/watch/replays';

  // Build lazy-loaded content for below-the-fold sections
  const playlistsHtml = featuredPlaylists.length > 0
    ? lazySection(
        'home-playlists',
        `<section class="home-playlists"><h2>Playlists</h2><div class="home-carousel">${renderPlaylistCards(featuredPlaylists)}</div></section>`,
        { placeholder: '<div class="lazy-placeholder" style="min-height:120px"></div>' }
      )
    : '';

  const seasonHtml = activeSeason && seasonProgress
    ? lazySection(
        'home-season',
        `<section class="home-season"><div class="home-season-info"><span class="home-season-name">${esc(activeSeason.name)}</span><span class="home-season-week">Week ${seasonProgress.week} of ${seasonProgress.totalWeeks}</span><span class="home-season-time">${seasonProgress.timeRemaining}</span></div><a href="#/watch/predictions" class="btn small primary">Predictions Open &rarr;</a></section>`,
        { placeholder: '<div class="lazy-placeholder" style="min-height:60px"></div>' }
      )
    : '';

  const evoHtml = lazySection(
    'home-evo',
    `<section class="home-evo"><div class="home-evo-info"><span class="home-evo-icon">&#129516;</span><span class="home-evo-text"><strong>Evolution Observatory</strong> &mdash; Gen #${evolutionMeta.generation}${evolutionMeta.promoted_today > 0 ? ` &middot; ${evolutionMeta.promoted_today} promoted today` : ''}${evolutionMeta.top_10_count > 0 ? ` &middot; ${evolutionMeta.top_10_count} in top 10` : ''}</span></div><a href="#/evolution" class="btn small secondary">Watch evolution live &rarr;</a></section>`,
    { placeholder: '<div class="lazy-placeholder" style="min-height:60px"></div>' }
  );

  app.innerHTML = `
<div class="home-page">

  <!-- Hero: headline + CTA -->
  <section class="home-hero">
    <h1>AI Code Battle</h1>
    <p class="home-tagline">Bots compete. Strategies evolve. You watch.</p>
    <div class="home-ctas">
      <a href="#/watch/replays" class="btn primary">Watch Battles</a>
      <a href="#/compete/register" class="btn secondary">Build a Bot</a>
    </div>
  </section>

  <!-- Featured Replay (auto-playing, territory view, muted) -->
  <section class="home-featured">
    <div class="home-replay-embed">
      <iframe
        src="${replayEmbedSrc}"
        frameborder="0"
        allowfullscreen
        loading="lazy"
        title="Featured replay"
      ></iframe>
    </div>
    <div class="home-replay-bar">
      <p class="home-replay-title">${replayTitle}</p>
      <a href="${replayLink}" class="btn small secondary">Watch Full Replay &rarr;</a>
    </div>
  </section>

  <!-- Two-column: Top 5 + Latest Stories -->
  <section class="home-grid">
    <div class="home-card">
      <h2>Top 5 Bots</h2>
      <div class="home-lb-list">
        ${top5.length > 0
          ? top5.map(
              (e: any, i: number) => `
        <div class="home-lb-row rank-${i + 1}">
          <span class="home-lb-rank">#${e.rank}</span>
          <a href="#/bot/${e.bot_id}" class="home-lb-name">${esc(e.name)}</a>
          <span class="home-lb-rating">${e.rating}</span>
        </div>`,
            ).join('')
          : '<p class="home-empty">No bots ranked yet</p>'}
      </div>
      <a href="#/leaderboard" class="btn small secondary">Full leaderboard &rarr;</a>
    </div>

    <div class="home-card">
      <h2>Latest Stories</h2>
      <div class="home-stories">
        ${latestStories.length > 0
          ? latestStories.map(
              (p: any) => `
        <a href="#/blog/${p.slug}" class="home-story">
          <span class="home-story-title">${esc(p.title)}</span>
          <span class="home-story-date">${p.published_at || p.date || ''}</span>
        </a>`,
            ).join('')
          : '<p class="home-empty">No stories yet</p>'}
      </div>
      <a href="#/blog" class="btn small secondary">All stories &rarr;</a>
    </div>
  </section>

  ${playlistsHtml}
  ${seasonHtml}
  ${evoHtml}
</div>

<style>
.home-page {
  max-width: 1200px;
  margin: 0 auto;
}

/* Hero — compact for above-the-fold on 1080p */
.home-hero {
  text-align: center;
  padding: 28px 20px 20px;
  background: linear-gradient(135deg, var(--bg-secondary) 0%, var(--bg-primary) 100%);
  border-radius: 10px;
  margin-bottom: 16px;
}
.home-hero h1 {
  font-size: 2.25rem;
  color: var(--text-primary);
  margin-bottom: 4px;
}
.home-tagline {
  font-size: 1.125rem;
  color: var(--accent);
  margin-bottom: 16px;
}
.home-ctas {
  display: flex;
  gap: 10px;
  justify-content: center;
}

/* Featured replay */
.home-featured {
  background: var(--bg-secondary);
  border-radius: 10px;
  overflow: hidden;
  margin-bottom: 16px;
}
.home-replay-embed {
  position: relative;
  width: 100%;
  aspect-ratio: 16 / 9;
  background: #000;
}
.home-replay-embed iframe {
  width: 100%;
  height: 100%;
}
.home-replay-bar {
  padding: 8px 14px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.home-replay-title {
  color: var(--text-primary);
  font-size: 0.8rem;
}

/* Two-column grid */
.home-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  margin-bottom: 16px;
}
.home-card {
  background: var(--bg-secondary);
  border-radius: 10px;
  padding: 16px;
}
.home-card h2 {
  font-size: 1rem;
  color: var(--text-primary);
  margin-bottom: 12px;
}

/* Leaderboard summary */
.home-lb-row {
  display: flex;
  gap: 10px;
  padding: 6px 0;
  border-bottom: 1px solid var(--border);
}
.home-lb-row:last-child { border-bottom: none; }
.home-lb-rank {
  width: 28px;
  font-weight: 700;
  color: var(--text-muted);
}
.rank-1 .home-lb-rank { color: #fbbf24; }
.rank-2 .home-lb-rank { color: #94a3b8; }
.rank-3 .home-lb-rank { color: #b45309; }
.home-lb-name {
  flex: 1;
  color: var(--text-secondary);
  text-decoration: none;
}
.home-lb-name:hover { color: var(--accent); }
.home-lb-rating {
  font-weight: 600;
  color: var(--text-primary);
}

/* Stories */
.home-stories {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.home-story {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  text-decoration: none;
  padding: 6px 0;
  gap: 10px;
}
.home-story-title {
  color: var(--text-secondary);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.home-story:hover .home-story-title { color: var(--accent); }
.home-story-date {
  font-size: 0.7rem;
  color: var(--text-muted);
  white-space: nowrap;
}

/* Playlists carousel */
.home-playlists h2 {
  font-size: 1rem;
  color: var(--text-primary);
  margin-bottom: 12px;
}
.home-carousel {
  display: flex;
  gap: 12px;
  overflow-x: auto;
  padding-bottom: 6px;
  scrollbar-width: thin;
  scrollbar-color: var(--border) var(--bg-secondary);
}
.home-carousel::-webkit-scrollbar { height: 5px; }
.home-carousel::-webkit-scrollbar-track {
  background: var(--bg-secondary);
  border-radius: 3px;
}
.home-carousel::-webkit-scrollbar-thumb {
  background: var(--border);
  border-radius: 3px;
}
.home-pl-card {
  flex-shrink: 0;
  width: 160px;
  background: var(--bg-secondary);
  border-radius: 8px;
  overflow: hidden;
  text-decoration: none;
  transition: transform 0.2s;
}
.home-pl-card:hover { transform: translateY(-2px); }
.home-pl-thumb {
  width: 100%;
  aspect-ratio: 16 / 9;
  background: var(--bg-tertiary);
  display: flex;
  align-items: center;
  justify-content: center;
}
.home-pl-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.home-pl-placeholder {
  font-size: 1.5rem;
  color: var(--text-muted);
}
.home-pl-info { padding: 8px; }
.home-pl-title {
  display: block;
  font-size: 0.8rem;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-bottom: 2px;
}
.home-pl-count {
  font-size: 0.7rem;
  color: var(--text-muted);
}

/* Season bar */
.home-season {
  background: linear-gradient(135deg, var(--accent) 0%, var(--accent-hover) 100%);
  border-radius: 10px;
  padding: 12px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}
.home-season-info {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}
.home-season-name {
  font-weight: 700;
  color: white;
  font-size: 1rem;
}
.home-season-week,
.home-season-time {
  color: rgba(255,255,255,0.9);
  font-size: 0.8rem;
}
.home-season .btn {
  background: white;
  color: var(--accent);
}
.home-season .btn:hover { background: #f1f5f9; }

/* Evolution mini */
.home-evo {
  background: var(--bg-secondary);
  border-radius: 10px;
  padding: 10px 14px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.home-evo-info {
  display: flex;
  align-items: center;
  gap: 8px;
}
.home-evo-icon { font-size: 1.25rem; }
.home-evo-text {
  color: var(--text-secondary);
  font-size: 0.8rem;
}
.home-evo-text strong { color: var(--text-primary); }

.home-empty {
  color: var(--text-muted);
  text-align: center;
  padding: 16px 0;
}

/* Responsive — phone (<640px) */
@media (max-width: 639px) {
  .home-grid { grid-template-columns: 1fr; }
  .home-hero h1 { font-size: 1.75rem; }
  .home-tagline { font-size: 1rem; }
  .home-hero { padding: 20px 16px; }
  .home-ctas { flex-wrap: wrap; }
  .home-season {
    flex-direction: column;
    gap: 10px;
    text-align: center;
  }
  .home-season-info {
    flex-direction: column;
    gap: 4px;
  }
  .home-evo {
    flex-direction: column;
    gap: 8px;
    text-align: center;
  }
  .home-replay-bar {
    flex-direction: column;
    gap: 8px;
    align-items: flex-start;
  }
  .home-pl-card { width: 140px; }
}
</style>`;

  // Activate lazy sections for below-the-fold content
  initLazySections(app);
}
