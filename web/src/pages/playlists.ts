// Playlists Page - Browse curated replay collections
// §16.15: lazy-rendered playlist grid, expandable match details,
// keyboard-accessible "Show more" affordances.
import { fetchPlaylistIndex, fetchPlaylist, type Playlist, type PlaylistIndex, type PlaylistMatch } from '../api-types';
import { initLazySections, lazySection } from '../lib/lazy-section';

function isMobile(): boolean {
  return window.innerWidth < 768;
}

const MATCH_BATCH = 10;

export async function renderPlaylistsPage(params?: Record<string, string>): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  const slug = params?.slug;

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
        background-color: var(--bg-secondary);
        border-radius: 8px;
        overflow: hidden;
        transition: background-color 0.2s;
      }

      .playlist-match-toggle {
        display: flex;
        align-items: center;
        gap: 16px;
        width: 100%;
        background: none;
        border: none;
        color: inherit;
        padding: 12px 16px;
        cursor: pointer;
        text-align: left;
      }

      .playlist-match-toggle:hover {
        background-color: var(--bg-tertiary);
      }

      .playlist-match-toggle:focus-visible {
        outline: 2px solid var(--accent);
        outline-offset: -2px;
        border-radius: 8px;
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

      .match-expand-icon {
        color: var(--text-muted);
        font-size: 0.75rem;
        transition: transform var(--transition-fast, 0.15s);
      }

      .playlist-match-details {
        max-height: 0;
        overflow: hidden;
        transition: max-height 200ms ease-out, padding 200ms ease-out;
        padding: 0 16px;
      }

      .playlist-match-details.expanded {
        max-height: 120px;
        padding: 0 16px 12px;
      }

      .playlist-match-actions {
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

      .watch-btn:focus-visible {
        outline: 2px solid var(--accent);
        outline-offset: 2px;
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

      .embed-btn:focus-visible {
        outline: 2px solid var(--accent);
        outline-offset: 2px;
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
      .category-badge.rivalry { background-color: #f97316; color: white; }
      .category-badge.season { background-color: #8b5cf6; color: white; }
      .category-badge.tutorial { background-color: #64748b; color: white; }

      .curation-tag {
        display: inline-block;
        font-size: 0.7rem;
        color: var(--text-muted);
        font-style: italic;
        margin-top: 2px;
      }

      @media (prefers-reduced-motion: reduce) {
        .playlist-match-details {
          transition: none;
        }
      }
    </style>
  `;

  if (slug) {
    await showPlaylistDetail(slug);
    document.getElementById('playlists-grid')!.style.display = 'none';
    (document.getElementById('playlist-detail') as HTMLElement).style.display = 'block';
    const backBtn = document.getElementById('back-btn');
    if (backBtn) {
      backBtn.onclick = () => {
        window.location.hash = '/watch/playlists';
      };
    }
  } else {
    await loadPlaylists();
  }
}

async function loadPlaylists(): Promise<void> {
  const grid = document.getElementById('playlists-grid');
  if (!grid) return;

  try {
    const index: PlaylistIndex = await fetchPlaylistIndex();

    if (index.playlists.length === 0) {
      grid.innerHTML = '<div class="empty-message">No playlists available yet</div>';
      return;
    }

    // Show first 6 immediately, lazy-load the rest
    const immediateCount = 6;
    const immediate = index.playlists.slice(0, immediateCount);
    const rest = index.playlists.slice(immediateCount);

    const immediateHtml = immediate.map(p => renderPlaylistCardHtml(p)).join('');

    if (rest.length === 0) {
      grid.innerHTML = immediateHtml;
    } else {
      const lazyContent = rest.map(p => renderPlaylistCardHtml(p)).join('');
      grid.innerHTML = immediateHtml + lazySection(
        'playlists-below-fold',
        lazyContent,
        { placeholder: '<div class="lazy-placeholder" style="min-height:200px"></div>' }
      );
      initLazySections(grid);
    }

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

function renderPlaylistCardHtml(p: PlaylistIndex['playlists'][number]): string {
  return `
    <div class="playlist-card" data-slug="${p.slug}">
      <h3>${escapeHtml(p.title)}<span class="category-badge ${p.category}">${formatCategory(p.category)}</span></h3>
      <p>${escapeHtml(p.description)}</p>
      <div class="meta">
        <span class="match-count">${p.match_count} matches</span>
        <span>Updated ${formatRelativeTime(p.updated_at)}</span>
      </div>
    </div>
  `;
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
    const playlist: Playlist = await fetchPlaylist(slug);

    // Mobile: defer to carousel component if available
    if (isMobile() && playlist.matches.length > 0) {
      try {
        const { PlaylistCarousel } = await import('../components/playlist-carousel');
        new PlaylistCarousel({
          playlist,
          onClose: () => {
            window.location.hash = '/watch/playlists';
          },
        });
        return;
      } catch {
        // Fallback to desktop view
      }
    }

    titleEl.textContent = playlist.title;
    descEl.textContent = playlist.description;

    // Render first batch of matches immediately
    const visibleMatches = playlist.matches.slice(0, MATCH_BATCH);
    const remainingMatches = playlist.matches.slice(MATCH_BATCH);

    matchesEl.innerHTML = visibleMatches.map(m => renderPlaylistMatchHtml(m)).join('');

    // Wire expand toggles
    initMatchExpandToggles(matchesEl);

    // Wire watch/embed buttons
    initMatchActions(matchesEl);

    // Add "Show more" for remaining matches
    if (remainingMatches.length > 0) {
      addMatchShowMore(matchesEl, remainingMatches);
    }

    grid.style.display = 'none';
    detail.style.display = 'block';

    backBtn!.onclick = () => {
      detail.style.display = 'none';
      grid.style.display = 'grid';
    };
  } catch (err) {
    console.error('Failed to load playlist:', err);
    alert('Failed to load playlist');
  }
}

function renderPlaylistMatchHtml(m: PlaylistMatch): string {
  const metaParts: string[] = [];
  if (m.turns) metaParts.push(`${m.turns} turns`);
  if (m.end_reason) metaParts.push(m.end_reason);
  if (m.completed_at) metaParts.push(formatRelativeTime(m.completed_at));
  const tag = m.curation_tag ? `<span class="curation-tag">${escapeHtml(m.curation_tag)}</span>` : '';

  return `
    <div class="playlist-match" data-match-id="${m.match_id}">
      <button class="playlist-match-toggle" type="button"
              aria-label="Expand details for ${escapeHtml(m.title || `Match ${m.order + 1}`)}"
              aria-expanded="false">
        <span class="match-order">${m.order + 1}</span>
        <div class="match-info">
          <div class="match-title">${escapeHtml(m.title || `Match ${m.order + 1}`)}</div>
          ${tag}
          <div class="match-meta">${metaParts.join(' · ')}</div>
        </div>
        <span class="match-expand-icon" aria-hidden="true">▸</span>
      </button>
      <div class="playlist-match-details">
        <div class="playlist-match-actions">
          <button class="watch-btn" data-match-id="${m.match_id}">Watch</button>
          <button class="embed-btn" data-match-id="${m.match_id}">Embed</button>
        </div>
      </div>
    </div>
  `;
}

function initMatchExpandToggles(root: HTMLElement): void {
  root.querySelectorAll<HTMLElement>('.playlist-match').forEach(match => {
    const toggle = match.querySelector<HTMLButtonElement>('.playlist-match-toggle');
    if (!toggle || toggle.dataset.wired) return;
    toggle.dataset.wired = '1';

    toggle.addEventListener('click', () => {
      const details = match.querySelector<HTMLElement>('.playlist-match-details');
      if (!details) return;
      const expanded = details.classList.toggle('expanded');
      toggle.setAttribute('aria-expanded', String(expanded));
      const icon = match.querySelector('.match-expand-icon');
      if (icon) icon.textContent = expanded ? '▾' : '▸';
    });
  });
}

function initMatchActions(root: HTMLElement): void {
  root.querySelectorAll<HTMLElement>('.watch-btn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      const matchId = btn.dataset.matchId;
      if (matchId) watchMatch(matchId);
    });
  });

  root.querySelectorAll<HTMLElement>('.embed-btn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      const matchId = btn.dataset.matchId;
      if (matchId) copyEmbedCode(matchId);
    });
  });
}

function addMatchShowMore(container: HTMLElement, remaining: PlaylistMatch[]): void {
  const btn = document.createElement('button');
  btn.className = 'btn secondary show-more-btn';
  btn.type = 'button';
  let offset = 0;
  const total = remaining.length;

  function updateBtn(): void {
    const left = total - offset;
    if (left <= 0) { btn.remove(); return; }
    const next = Math.min(MATCH_BATCH, left);
    btn.textContent = `Show ${next} more matches (${left} remaining)`;
    btn.setAttribute('aria-label', `Show ${next} more matches, ${left} remaining`);
  }

  btn.addEventListener('click', () => {
    const batch = remaining.slice(offset, offset + MATCH_BATCH);
    if (batch.length === 0) return;

    const temp = document.createElement('div');
    temp.innerHTML = batch.map(m => renderPlaylistMatchHtml(m)).join('');
    while (temp.firstChild) {
      container.appendChild(temp.firstChild);
    }

    initMatchExpandToggles(container);
    initMatchActions(container);

    offset += batch.length;
    updateBtn();
  });

  updateBtn();
  container.after(btn);
}

function watchMatch(matchId: string): void {
  window.location.hash = `/watch/replay?url=/replays/${matchId}.json.gz`;
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

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}
