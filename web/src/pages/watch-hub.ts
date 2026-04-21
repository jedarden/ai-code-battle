// Watch hub page - spectator hub with replays, playlists, predictions

function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

export function renderWatchHubPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="watch-hub-page">
      <h1 class="page-title">Watch</h1>
      <p class="page-subtitle">Spectate matches, browse replays, and make predictions</p>

      <div class="watch-grid">
        <a href="#/watch/replays" class="watch-card">
          <div class="card-icon">📺</div>
          <h2>Match Replays</h2>
          <p>Browse all completed matches and watch replays</p>
        </a>

        <a href="#/watch/predictions" class="watch-card">
          <div class="card-icon">🎯</div>
          <h2>Predictions</h2>
          <p>Predict match winners and climb the predictor leaderboard</p>
        </a>

        <a href="#/leaderboard" class="watch-card">
          <div class="card-icon">🏆</div>
          <h2>Leaderboard</h2>
          <p>See current rankings and top bots</p>
        </a>
      </div>

      <div class="featured-section">
        <h2>Featured Playlists</h2>
        <div id="featured-playlists" class="playlists-preview">
          <div class="loading">Loading playlists...</div>
        </div>
      </div>
    </div>

    <style>
      .watch-hub-page { max-width: 1200px; margin: 0 auto; }
      .page-subtitle { color: var(--text-muted); margin-bottom: 32px; }
      .watch-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 20px; margin-bottom: 40px; }
      .watch-card { background-color: var(--bg-secondary); border-radius: 12px; padding: 32px 24px; text-decoration: none; transition: transform 0.2s, box-shadow 0.2s; display: block; }
      .watch-card:hover { transform: translateY(-4px); box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3); }
      .card-icon { font-size: 3rem; margin-bottom: 16px; }
      .watch-card h2 { color: var(--text-primary); margin-bottom: 8px; font-size: 1.25rem; }
      .watch-card p { color: var(--text-muted); font-size: 0.875rem; }
      .featured-section { margin-top: 40px; }
      .featured-section h2 { color: var(--text-primary); margin-bottom: 16px; }
      .playlists-preview { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 16px; }
      .playlist-preview-card { background-color: var(--bg-secondary); border-radius: 8px; padding: 16px; text-decoration: none; transition: transform 0.2s; }
      .playlist-preview-card:hover { transform: translateY(-2px); }
      .playlist-preview-card h3 { color: var(--text-primary); font-size: 1rem; margin-bottom: 4px; }
      .playlist-preview-card p { color: var(--text-muted); font-size: 0.75rem; }
      .loading { color: var(--text-muted); text-align: center; padding: 40px; grid-column: 1 / -1; }
    </style>
  `;

  loadFeaturedPlaylists();
}

async function loadFeaturedPlaylists(): Promise<void> {
  const container = document.getElementById('featured-playlists');
  if (!container) return;

  try {
    const response = fetch('/data/playlists/index.json');
    const data = await (await response).json();

    if (data.playlists.length === 0) {
      container.innerHTML = '<p style="color: var(--text-muted);">No playlists available yet.</p>';
      return;
    }

    const featured = data.playlists.slice(0, 6);
    container.innerHTML = featured.map((p: any) => `
      <a href="#/watch/playlists/${p.slug}" class="playlist-preview-card">
        <h3>${escapeHtml(p.title)}</h3>
        <p>${p.match_count} matches · ${escapeHtml(p.description || '').substring(0, 60)}</p>
      </a>
    `).join('');
  } catch {
    container.innerHTML = '<p style="color: var(--text-muted);">Failed to load playlists.</p>';
  }
}
