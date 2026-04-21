// Match history page - displays recent matches with featured playlists

import { fetchMatchIndex, fetchPlaylistIndex, type MatchSummary, type PlaylistIndex } from '../api-types';

export async function renderMatchesPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="matches-page">
      <h1>Match History</h1>
      <div id="playlists-section"></div>
      <div id="matches-content" class="loading">Loading...</div>
    </div>

    <style>
      .curated-playlists { margin-bottom: 40px; }
      .curated-playlists-header { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 16px; }
      .curated-playlists-header h2 { color: var(--text-primary); font-size: 1.25rem; margin: 0; }
      .curated-playlists-header a { color: var(--accent, #3b82f6); font-size: 0.875rem; text-decoration: none; }
      .curated-playlists-header a:hover { text-decoration: underline; }
      .curated-sections { display: grid; grid-template-columns: 2fr 1fr 1fr; gap: 16px; margin-bottom: 20px; }
      .curated-section {
        background-color: var(--bg-secondary);
        border-radius: 12px;
        padding: 20px;
        text-decoration: none;
        display: flex;
        flex-direction: column;
        gap: 8px;
        transition: transform 0.2s, box-shadow 0.2s;
        border: 1px solid transparent;
      }
      .curated-section:hover { transform: translateY(-2px); box-shadow: 0 4px 16px rgba(0,0,0,0.3); }
      .curated-section.primary { border-color: rgba(236,72,153,0.25); background: linear-gradient(135deg, var(--bg-secondary) 0%, rgba(236,72,153,0.07) 100%); }
      .curated-section h3 { color: var(--text-primary); font-size: 1rem; margin: 0; }
      .curated-section.primary h3 { font-size: 1.1rem; }
      .curated-section p { color: var(--text-muted); font-size: 0.8rem; line-height: 1.5; flex: 1; margin: 0; }
      .section-meta { display: flex; justify-content: space-between; align-items: center; font-size: 0.75rem; color: var(--text-muted); margin-top: 4px; }
      .section-browse { color: var(--accent, #3b82f6); }
      .more-playlists-label { color: var(--text-muted); font-size: 0.8rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 10px; }
      .playlists-row {
        display: flex;
        gap: 12px;
        overflow-x: auto;
        padding-bottom: 8px;
        scroll-snap-type: x mandatory;
      }
      .playlists-row::-webkit-scrollbar { height: 4px; }
      .playlists-row::-webkit-scrollbar-thumb { background: var(--border, #333); border-radius: 2px; }
      .playlist-card {
        flex: 0 0 200px;
        scroll-snap-align: start;
        background-color: var(--bg-secondary);
        border-radius: 10px;
        padding: 14px;
        text-decoration: none;
        display: flex;
        flex-direction: column;
        gap: 6px;
        transition: transform 0.2s;
      }
      .playlist-card:hover { transform: translateY(-2px); }
      .playlist-card h3 {
        color: var(--text-primary);
        font-size: 0.85rem;
        margin: 0;
        display: flex;
        align-items: center;
        gap: 6px;
        flex-wrap: wrap;
      }
      .playlist-card p {
        color: var(--text-muted);
        font-size: 0.75rem;
        margin: 0;
        line-height: 1.4;
        flex: 1;
      }
      .playlist-card .meta {
        font-size: 0.7rem;
        color: var(--text-muted);
      }
      .category-badge {
        display: inline-block;
        padding: 2px 6px;
        border-radius: 4px;
        font-size: 0.6rem;
        text-transform: uppercase;
        font-weight: 600;
        letter-spacing: 0.5px;
        white-space: nowrap;
      }
      .category-badge.featured { background-color: #3b82f6; color: white; }
      .category-badge.upsets { background-color: #ef4444; color: white; }
      .category-badge.comebacks { background-color: #f59e0b; color: white; }
      .category-badge.domination { background-color: #8b5cf6; color: white; }
      .category-badge.close_games { background-color: #22c55e; color: white; }
      .category-badge.long_games { background-color: #06b6d4; color: white; }
      .category-badge.weekly { background-color: #ec4899; color: white; }
      .category-badge.rivalry { background-color: #f97316; color: white; }
      .category-badge.season { background-color: #8b5cf6; color: white; }
      .category-badge.tutorial { background-color: #64748b; color: white; }
      .playlist-empty { color: var(--text-muted); font-size: 0.875rem; }
      @media (max-width: 768px) {
        .curated-sections { grid-template-columns: 1fr; }
        .curated-section.primary { grid-column: 1; }
      }
      @media (max-width: 480px) {
        .playlist-card { flex: 0 0 170px; padding: 10px; }
      }
    </style>
  `;

  const content = document.getElementById('matches-content');
  const playlistsSection = document.getElementById('playlists-section');
  if (!content || !playlistsSection) return;

  // Load playlists in parallel with matches
  const [matchResult, playlistResult] = await Promise.allSettled([
    fetchMatchIndex(),
    fetchPlaylistIndex(),
  ]);

  // Render playlists section
  if (playlistResult.status === 'fulfilled' && playlistResult.value.playlists.length > 0) {
    renderPlaylistCards(playlistsSection, playlistResult.value);
  }

  // Render matches
  if (matchResult.status === 'fulfilled') {
    renderMatchesList(content, matchResult.value.matches, matchResult.value.updated_at);
  } else {
    content.innerHTML = `
      <div class="error">
        <p>Failed to load match history: ${matchResult.reason}</p>
        <p class="hint">Match data may not be available yet.</p>
      </div>
    `;
  }
}

function renderPlaylistCards(container: HTMLElement, index: PlaylistIndex): void {
  // Priority slugs for the curated sections at the top
  const curatedSlugs = ['best-of-week', 'biggest-upsets', 'closest-finishes'];
  const curatedSections = curatedSlugs
    .map(slug => index.playlists.find(p => p.slug === slug))
    .filter((p): p is NonNullable<typeof p> => p !== undefined);

  // Remaining playlists shown as a scrollable row
  const curatedSlugSet = new Set(curatedSlugs);
  const rest = index.playlists.filter(p => !curatedSlugSet.has(p.slug));

  const curatedHtml = curatedSections.map((p, i) => `
    <a href="#/watch/playlists/${p.slug}" class="curated-section ${i === 0 ? 'primary' : ''}">
      <span class="category-badge ${p.category}">${formatCategory(p.category)}</span>
      <h3>${escapeHtml(p.title)}</h3>
      <p>${escapeHtml(p.description)}</p>
      <div class="section-meta">
        <span>${p.match_count} matches</span>
        <span class="section-browse">Browse →</span>
      </div>
    </a>
  `).join('');

  const restHtml = rest.map(p => `
    <a href="#/watch/playlists/${p.slug}" class="playlist-card">
      <h3>
        ${escapeHtml(p.title)}
        <span class="category-badge ${p.category}">${formatCategory(p.category)}</span>
      </h3>
      <p>${escapeHtml(p.description)}</p>
      <div class="meta">${p.match_count} matches · ${formatRelativeTime(p.updated_at)}</div>
    </a>
  `).join('');

  container.innerHTML = `
    <div class="curated-playlists">
      <div class="curated-playlists-header">
        <h2>Featured Playlists</h2>
        <a href="#/watch/playlists">Browse all →</a>
      </div>
      ${curatedSections.length > 0 ? `<div class="curated-sections">${curatedHtml}</div>` : ''}
      ${rest.length > 0 ? `
        <p class="more-playlists-label">More Collections</p>
        <div class="playlists-row">${restHtml}</div>
      ` : ''}
    </div>
  `;
}

function renderMatchesList(
  container: HTMLElement,
  matches: MatchSummary[],
  updatedAt: string
): void {
  if (matches.length === 0) {
    container.innerHTML = `
      <div class="empty-state">
        <p>No matches have been played yet.</p>
        <p>Matches will appear here once bots are registered and competing.</p>
        <a href="#/leaderboard" class="btn primary">View Leaderboard</a>
      </div>
    `;
    return;
  }

  container.innerHTML = `
    <p class="updated-at">Last updated: ${formatTimestamp(updatedAt)}</p>
    <div class="matches-list">
      ${matches.map(match => renderMatchCard(match)).join('')}
    </div>
  `;
}

function renderMatchCard(match: MatchSummary): string {
  const completedAt = match.completed_at ? formatTimestamp(match.completed_at) : 'In progress';

  return `
    <div class="match-card" data-match-id="${escapeHtml(match.id)}">
      <div class="match-header">
        <span class="match-id">${escapeHtml(match.id.slice(0, 8))}</span>
        <span class="match-time">${completedAt}</span>
      </div>
      <div class="match-participants">
        ${match.participants.map(p => `
          <div class="participant ${p.won ? 'winner' : ''}">
            <a href="#/bot/${encodeURIComponent(p.bot_id)}" class="participant-name">
              ${escapeHtml(p.name)}
            </a>
            <span class="participant-score">${p.score}</span>
            ${p.won ? '<span class="winner-badge">Winner</span>' : ''}
          </div>
        `).join('')}
      </div>
      <div class="match-footer">
        <span class="match-turns">${match.turns ?? '-'} turns</span>
        <span class="match-reason">${match.end_reason ?? '-'}</span>
        <a href="#/watch/replay?url=/replays/${match.id}.json" class="btn small">Watch</a>
      </div>
    </div>
  `;
}

function formatCategory(category: string): string {
  const labels: Record<string, string> = {
    featured: 'Featured',
    rivalry: 'Rivalry',
    upsets: 'Upsets',
    comebacks: 'Comebacks',
    domination: 'Domination',
    close_games: 'Close',
    long_games: 'Marathon',
    tutorial: 'Tutorial',
    season: 'Season',
    weekly: 'Weekly',
  };
  return labels[category] || category;
}

function formatRelativeTime(isoDate: string): string {
  const date = new Date(isoDate);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

function formatTimestamp(iso: string): string {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}
