// Playlists Page - Browse curated replay collections
import type { Playlist, PlaylistIndex } from '../api-types';

const PAGES_BASE = '';

export async function renderPlaylistsPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="playlists-page">
      <h1 class="page-title">Replay Playlists</h1>
      <p class="page-subtitle">Curated collections of the best matches</p>

      <div class="playlists-grid" id="playlists-grid">
        <div class="loading">Loading playlists...</div>
      </div>

      <div class="playlist-detail" id="playlist-detail" style="display: none;">
        <button class="back-btn" id="back-btn">← Back to Playlists</button>
        <div class="playlist-header">
          <h2 id="playlist-title"></h2>
          <p id="playlist-description"></p>
        </div>
        <div class="playlist-matches" id="playlist-matches"></div>
      </div>
    </div>

    <style>
      .playlists-page {
        max-width: 1200px;
        margin: 0 auto;
      }

      .page-title {
        margin-bottom: 8px;
      }

      .page-subtitle {
        color: var(--text-muted);
        margin-bottom: 24px;
      }

      .playlists-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
        gap: 20px;
      }

      .playlist-card {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 20px;
        cursor: pointer;
        transition: transform 0.2s, box-shadow 0.2s;
      }

      .playlist-card:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
      }

      .playlist-card h3 {
        color: var(--text-primary);
        margin-bottom: 8px;
        font-size: 1.1rem;
      }

      .playlist-card p {
        color: var(--text-muted);
        font-size: 0.875rem;
        margin-bottom: 12px;
      }

      .playlist-card .meta {
        display: flex;
        justify-content: space-between;
        color: var(--text-muted);
        font-size: 0.75rem;
      }

      .playlist-card .match-count {
        background-color: var(--accent);
        color: white;
        padding: 2px 8px;
        border-radius: 12px;
        font-weight: 500;
      }

      .loading {
        color: var(--text-muted);
        text-align: center;
        padding: 40px;
        grid-column: 1 / -1;
      }

      .playlist-detail {
        margin-top: 20px;
      }

      .back-btn {
        background-color: transparent;
        color: var(--accent);
        border: none;
        padding: 8px 0;
        cursor: pointer;
        font-size: 14px;
        margin-bottom: 16px;
      }

      .back-btn:hover {
        text-decoration: underline;
      }

      .playlist-header {
        margin-bottom: 24px;
      }

      .playlist-header h2 {
        margin-bottom: 8px;
      }

      .playlist-header p {
        color: var(--text-muted);
      }

      .playlist-matches {
        display: flex;
        flex-direction: column;
        gap: 12px;
      }

      .playlist-match {
        display: flex;
        align-items: center;
        gap: 16px;
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 12px 16px;
        cursor: pointer;
        transition: background-color 0.2s;
      }

      .playlist-match:hover {
        background-color: var(--bg-tertiary);
      }

      .match-order {
        font-size: 1.25rem;
        font-weight: 600;
        color: var(--text-muted);
        min-width: 30px;
      }

      .match-info {
        flex: 1;
      }

      .match-title {
        color: var(--text-primary);
        font-weight: 500;
        margin-bottom: 4px;
      }

      .match-meta {
        color: var(--text-muted);
        font-size: 0.75rem;
      }

      .match-actions {
        display: flex;
        gap: 8px;
      }

      .watch-btn {
        background-color: var(--accent);
        color: white;
        border: none;
        padding: 6px 12px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 12px;
      }

      .watch-btn:hover {
        opacity: 0.9;
      }

      .embed-btn {
        background-color: transparent;
        color: var(--text-muted);
        border: 1px solid var(--border);
        padding: 6px 12px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 12px;
      }

      .embed-btn:hover {
        border-color: var(--accent);
        color: var(--accent);
      }

      .empty-message {
        color: var(--text-muted);
        text-align: center;
        padding: 40px;
      }

      .category-badge {
        display: inline-block;
        padding: 2px 8px;
        border-radius: 4px;
        font-size: 0.7rem;
        text-transform: uppercase;
        font-weight: 600;
        margin-left: 8px;
      }

      .category-badge.featured { background-color: #3b82f6; color: white; }
      .category-badge.upsets { background-color: #ef4444; color: white; }
      .category-badge.comebacks { background-color: #f59e0b; color: white; }
      .category-badge.domination { background-color: #8b5cf6; color: white; }
      .category-badge.close_games { background-color: #22c55e; color: white; }
      .category-badge.long_games { background-color: #06b6d4; color: white; }
      .category-badge.weekly { background-color: #ec4899; color: white; }
    </style>
  `;

  // Load playlists
  await loadPlaylists();
}

async function loadPlaylists(): Promise<void> {
  const grid = document.getElementById('playlists-grid');
  if (!grid) return;

  try {
    const response = await fetch(`${PAGES_BASE}/data/playlists/index.json`);
    if (!response.ok) throw new Error('Failed to load playlists');
    const index: PlaylistIndex = await response.json();

    if (index.playlists.length === 0) {
      grid.innerHTML = '<div class="empty-message">No playlists available yet</div>';
      return;
    }

    grid.innerHTML = index.playlists.map(p => `
      <div class="playlist-card" data-slug="${p.slug}">
        <h3>${p.title}<span class="category-badge ${p.category}">${formatCategory(p.category)}</span></h3>
        <p>${p.description}</p>
        <div class="meta">
          <span class="match-count">${p.match_count} matches</span>
          <span>Updated ${formatRelativeTime(p.updated_at)}</span>
        </div>
      </div>
    `).join('');

    // Wire up click handlers
    grid.querySelectorAll('.playlist-card').forEach(card => {
      card.addEventListener('click', () => {
        const slug = (card as HTMLElement).dataset.slug;
        if (slug) showPlaylistDetail(slug);
      });
    });
  } catch (err) {
    console.error('Failed to load playlists:', err);
    grid.innerHTML = '<div class="empty-message">Failed to load playlists. Please try again later.</div>';
  }
}

async function showPlaylistDetail(slug: string): Promise<void> {
  const grid = document.getElementById('playlists-grid');
  const detail = document.getElementById('playlist-detail');
  const backBtn = document.getElementById('back-btn');
  const titleEl = document.getElementById('playlist-title');
  const descEl = document.getElementById('playlist-description');
  const matchesEl = document.getElementById('playlist-matches');

  if (!grid || !detail || !titleEl || !descEl || !matchesEl) return;

  try {
    const response = await fetch(`${PAGES_BASE}/data/playlists/${slug}.json`);
    if (!response.ok) throw new Error('Playlist not found');
    const playlist: Playlist = await response.json();

    titleEl.textContent = playlist.title;
    descEl.textContent = playlist.description;

    matchesEl.innerHTML = playlist.matches.map(m => `
      <div class="playlist-match" data-match-id="${m.match_id}">
        <span class="match-order">${m.order + 1}</span>
        <div class="match-info">
          <div class="match-title">${m.title || `Match ${m.order + 1}`}</div>
          <div class="match-meta">ID: ${m.match_id}</div>
        </div>
        <div class="match-actions">
          <button class="watch-btn" data-match-id="${m.match_id}">Watch</button>
          <button class="embed-btn" data-match-id="${m.match_id}">Embed</button>
        </div>
      </div>
    `).join('');

    // Wire up buttons
    matchesEl.querySelectorAll('.watch-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const matchId = (btn as HTMLElement).dataset.matchId;
        if (matchId) watchMatch(matchId);
      });
    });

    matchesEl.querySelectorAll('.embed-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const matchId = (btn as HTMLElement).dataset.matchId;
        if (matchId) copyEmbedCode(matchId);
      });
    });

    // Show detail, hide grid
    grid.style.display = 'none';
    detail.style.display = 'block';

    // Back button handler
    backBtn!.onclick = () => {
      detail.style.display = 'none';
      grid.style.display = 'grid';
    };
  } catch (err) {
    console.error('Failed to load playlist:', err);
    alert('Failed to load playlist');
  }
}

function watchMatch(matchId: string): void {
  window.location.hash = `/watch/replay?url=/replays/${matchId}.json`;
}

function copyEmbedCode(matchId: string): void {
  const embedUrl = `${window.location.origin}/embed/${matchId}`;
  const code = `<iframe src="${embedUrl}" width="640" height="480" frameborder="0" allowfullscreen></iframe>`;
  navigator.clipboard.writeText(code).then(() => {
    alert('Embed code copied to clipboard!');
  });
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
