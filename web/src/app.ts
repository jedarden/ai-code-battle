// Main SPA entry point with routing
// Code splitting: pages are loaded on-demand to keep initial bundle small
import { router } from './router';
import type { RouteHandler } from './router';
import type { Replay, GameEvent } from './types';

// ─── Lazy loaders for code splitting ─────────────────────────────────────────────
// Each loader creates its own chunk, loaded only when the route is visited

// Core pages - loaded frequently
const loadHomePage = () => import('./pages/home').then(m => m.renderHomePage);
const loadLeaderboardPage = () => import('./pages/leaderboard').then(m => m.renderLeaderboardPage);

// Watch section - replay viewer and related pages
const loadMatchesPage = () => import('./pages/matches').then(m => m.renderMatchesPage);
const loadSeriesPage = () => import('./pages/series').then(m => m.renderSeriesPage);
const loadPredictionsPage = () => import('./pages/predictions').then(m => m.renderPredictionsPage);
const loadReplayViewer = () => import('./replay-viewer');

// Compete section - sandbox, register, docs
const loadSandboxPage = () => import('./pages/sandbox').then(m => m.renderSandboxPage);
const loadRegisterPage = () => import('./pages/register').then(m => m.renderRegisterPage);

// Bot-related pages
const loadBotProfilePage = () => import('./pages/bot-profile').then(m => m.renderBotProfilePage);
const loadEvolutionPage = () => import('./pages/evolution').then(m => m.renderEvolutionPage);

// Blog & seasons
const loadBlogPages = () => import('./pages/blog').then(m => ({ renderBlogPage: m.renderBlogPage, renderBlogPostPage: m.renderBlogPostPage }));
const loadSeasonsPage = () => import('./pages/seasons').then(m => m.renderSeasonsPage);

// ─── Helper: wrap async page loader in sync RouteHandler ────────────────────────
function lazyRoute(loader: () => Promise<(params: Record<string, string>) => void>): RouteHandler {
  return (params: Record<string, string>) => {
    loader().then(handler => handler(params));
  };
}

// ─── Backwards compatibility redirects ────────────────────────────────────────────
function redirect(to: string): RouteHandler {
  return (params: Record<string, string>) => {
    const fullPath = Object.entries(params).reduce(
      (path, [key, value]) => path.replace(`:${key}`, encodeURIComponent(value)),
      to
    );
    router.navigate(fullPath);
  };
}

// ─── In-page route handlers (no lazy load needed) ────────────────────────────────

// Watch hub page - spectator hub with replays, playlists, predictions
function renderWatchHubPage(): void {
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

    const featured = data.playlists.slice(0, 4);
    container.innerHTML = featured.map((p: any) => `
      <a href="#/watch/replays" class="playlist-preview-card">
        <h3>${escapeHtml(p.title)}</h3>
        <p>${p.match_count} matches</p>
      </a>
    `).join('');
  } catch {
    container.innerHTML = '<p style="color: var(--text-muted);">Failed to load playlists.</p>';
  }
}

// Compete hub page - participant hub with sandbox, register, docs
function renderCompeteHubPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="compete-hub-page">
      <h1 class="page-title">Compete</h1>
      <p class="page-subtitle">Build your bot and climb the ranks</p>

      <div class="getting-started">
        <h2>Getting Started</h2>
        <p>AI Code Battle is a competitive programming platform where you write HTTP bots that control units on a grid world.</p>
      </div>

      <div class="compete-grid">
        <a href="#/compete/sandbox" class="compete-card primary">
          <div class="card-icon">🧪</div>
          <h2>Test in Sandbox</h2>
          <p>Write code and run matches in-browser with no server needed</p>
        </a>

        <a href="#/compete/register" class="compete-card primary">
          <div class="card-icon">🤖</div>
          <h2>Register Your Bot</h2>
          <p>Sign up your HTTP bot and start competing</p>
        </a>

        <a href="#/compete/docs" class="compete-card">
          <div class="card-icon">📖</div>
          <h2>Documentation</h2>
          <p>Read the protocol spec and starter kit guides</p>
        </a>

        <a href="https://github.com/aicodebattle/acb" class="compete-card" target="_blank" rel="noopener">
          <div class="card-icon">💻</div>
          <h2>Starter Kits</h2>
          <p>Example bots in Python, Go, Rust, TypeScript, and more</p>
        </a>

        <a href="#/leaderboard" class="compete-card">
          <div class="card-icon">🏆</div>
          <h2>Leaderboard</h2>
          <p>See current standings and top performers</p>
        </a>

        <a href="#/evolution" class="compete-card">
          <div class="card-icon">🧬</div>
          <h2>Evolution</h2>
          <p>Watch bots evolve through genetic algorithms</p>
        </a>
      </div>

      <div class="how-it-works">
        <h2>How Competition Works</h2>
        <div class="steps">
          <div class="step">
            <span class="step-number">1</span>
            <h3>Build a Bot</h3>
            <p>Write an HTTP server that receives game state and returns move commands</p>
          </div>
          <div class="step">
            <span class="step-number">2</span>
            <h3>Register</h3>
            <p>Submit your bot's endpoint URL and API key to start competing</p>
          </div>
          <div class="step">
            <span class="step-number">3</span>
            <h3>Climb the Ranks</h3>
            <p>Your bot plays matches automatically and earns rating through Glicko-2</p>
          </div>
        </div>
      </div>
    </div>

    <style>
      .compete-hub-page { max-width: 1200px; margin: 0 auto; }
      .page-subtitle { color: var(--text-muted); margin-bottom: 32px; }
      .getting-started { background-color: var(--bg-secondary); border-radius: 12px; padding: 24px; margin-bottom: 32px; }
      .getting-started h2 { color: var(--text-primary); margin-bottom: 12px; }
      .getting-started p { color: var(--text-muted); }
      .compete-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 20px; margin-bottom: 40px; }
      .compete-card { background-color: var(--bg-secondary); border-radius: 12px; padding: 32px 24px; text-decoration: none; transition: transform 0.2s, box-shadow 0.2s; display: block; border: 2px solid transparent; }
      .compete-card:hover { transform: translateY(-4px); box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3); }
      .compete-card.primary { border-color: var(--accent); background-color: rgba(59, 130, 246, 0.1); }
      .card-icon { font-size: 3rem; margin-bottom: 16px; }
      .compete-card h2 { color: var(--text-primary); margin-bottom: 8px; font-size: 1.25rem; }
      .compete-card p { color: var(--text-muted); font-size: 0.875rem; }
      .how-it-works { background-color: var(--bg-secondary); border-radius: 12px; padding: 32px; }
      .how-it-works h2 { color: var(--text-primary); margin-bottom: 24px; }
      .steps { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 24px; }
      .step { display: flex; flex-direction: column; gap: 12px; }
      .step-number { display: flex; align-items: center; justify-content: center; width: 48px; height: 48px; background-color: var(--accent); color: white; border-radius: 50%; font-weight: 700; font-size: 1.25rem; }
      .step h3 { color: var(--text-primary); }
      .step p { color: var(--text-muted); font-size: 0.875rem; }
    </style>
  `;
}

// Season detail page - standalone page for viewing a specific season
function renderSeasonDetailPage(params: Record<string, string>): void {
  const seasonId = params.id;
  if (!seasonId) {
    router.navigate('/seasons');
    return;
  }

  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="season-detail-page">
      <div class="breadcrumb">
        <a href="#/seasons">Seasons</a> / <span id="season-breadcrumb">Loading...</span>
      </div>
      <div id="season-content" class="loading">Loading season...</div>
    </div>

    <style>
      .season-detail-page { max-width: 1000px; margin: 0 auto; }
      .breadcrumb { color: var(--text-muted); font-size: 0.875rem; margin-bottom: 20px; }
      .breadcrumb a { color: var(--accent); text-decoration: none; }
      .breadcrumb a:hover { text-decoration: underline; }
      .loading { color: var(--text-muted); text-align: center; padding: 40px; }
      .season-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 24px; flex-wrap: wrap; gap: 16px; }
      .season-info h1 { font-size: 2rem; color: var(--text-primary); margin-bottom: 8px; }
      .season-theme { color: var(--text-muted); font-size: 1rem; }
      .season-dates { text-align: right; color: var(--text-muted); font-size: 0.875rem; }
      .status-badge { display: inline-block; padding: 4px 12px; border-radius: 4px; font-size: 0.75rem; font-weight: 600; text-transform: uppercase; margin-bottom: 8px; }
      .status-badge.active { background-color: #22c55e; color: white; }
      .status-badge.completed { background-color: #3b82f6; color: white; }
      .status-badge.upcoming { background-color: #6b7280; color: white; }
      .champion-banner { background: linear-gradient(135deg, rgba(255, 215, 0, 0.1) 0%, rgba(255, 215, 0, 0.05) 100%); border: 1px solid rgba(255, 215, 0, 0.3); border-radius: 12px; padding: 24px; text-align: center; margin-bottom: 32px; }
      .champion-crown { font-size: 3rem; margin-bottom: 8px; }
      .champion-label { color: var(--text-muted); font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.1em; }
      .champion-name { font-size: 1.5rem; color: gold; font-weight: 700; }
      .section-title { font-size: 1.25rem; color: var(--text-primary); margin-bottom: 16px; }
      .leaderboard-table { width: 100%; border-collapse: collapse; background-color: var(--bg-secondary); border-radius: 8px; overflow: hidden; margin-bottom: 32px; }
      .leaderboard-table th, .leaderboard-table td { padding: 12px 16px; text-align: left; border-bottom: 1px solid var(--bg-tertiary); }
      .leaderboard-table th { background-color: var(--bg-tertiary); color: var(--text-muted); font-weight: 600; font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; }
      .leaderboard-table .rank { font-weight: 700; color: var(--text-muted); }
      .leaderboard-table tr.rank-1 .rank { color: #fbbf24; }
      .leaderboard-table tr.rank-2 .rank { color: #94a3b8; }
      .leaderboard-table tr.rank-3 .rank { color: #cd7f32; }
      .season-rules { background-color: var(--bg-tertiary); border-radius: 8px; padding: 20px; }
      .season-rules h4 { color: var(--text-primary); margin-bottom: 12px; }
      .season-rules ul { margin-left: 20px; color: var(--text-muted); }
      .season-rules li { margin-bottom: 6px; }
    </style>
  `;

  loadSeasonDetail(seasonId);
}

async function loadSeasonDetail(seasonId: string): Promise<void> {
  const breadcrumb = document.getElementById('season-breadcrumb');
  const content = document.getElementById('season-content');

  if (!content) return;

  try {
    const response = await fetch(`/data/seasons/${seasonId}.json`);
    if (!response.ok) throw new Error('Season not found');
    const season = await response.json();

    if (breadcrumb) {
      breadcrumb.textContent = season.name;
    }

    content.innerHTML = `
      <div class="season-header">
        <div class="season-info">
          <h1>${escapeHtml(season.name)}</h1>
          <p class="season-theme">${escapeHtml(season.theme)}</p>
        </div>
        <div class="season-dates">
          <span class="status-badge ${season.status}">${season.status}</span>
          <div>Started: ${new Date(season.starts_at).toLocaleDateString()}</div>
          ${season.ends_at ? `<div>Ended: ${new Date(season.ends_at).toLocaleDateString()}</div>` : ''}
        </div>
      </div>

      ${season.champion_name ? `
        <div class="champion-banner">
          <div class="champion-crown">👑</div>
          <div class="champion-label">Champion</div>
          <div class="champion-name">${escapeHtml(season.champion_name)}</div>
        </div>
      ` : ''}

      ${season.final_snapshot && season.final_snapshot.length > 0 ? `
        <h2 class="section-title">Final Leaderboard</h2>
        <table class="leaderboard-table">
          <thead>
            <tr>
              <th>Rank</th>
              <th>Bot</th>
              <th>Rating</th>
              <th>Wins</th>
              <th>Losses</th>
            </tr>
          </thead>
          <tbody>
            ${season.final_snapshot.map((entry: any) => `
              <tr class="rank-${entry.rank}">
                <td class="rank">#${entry.rank}</td>
                <td>${escapeHtml(entry.bot_name)}</td>
                <td>${Math.round(entry.rating)}</td>
                <td>${entry.wins}</td>
                <td>${entry.losses}</td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      ` : ''}

      <div class="season-rules">
        <h4>Rules Version: ${season.rules_version}</h4>
        <ul>
          <li>Standard 60×60 toroidal grid</li>
          <li>500 turn limit</li>
          <li>Glicko-2 rating system</li>
          <li>Best-of-1 matches</li>
        </ul>
      </div>
    `;
  } catch (err) {
    console.error('Failed to load season:', err);
    content.innerHTML = `
      <div class="error">
        <p>Failed to load season: ${seasonId}</p>
        <p class="hint">The season may not exist yet.</p>
        <a href="#/seasons" class="btn primary">Back to Seasons</a>
      </div>
    `;
  }
}

// Replay viewer page - lazy loads the ReplayViewer class
function renderReplayPage(params: Record<string, string>): void {
  const app = document.getElementById('app');
  if (!app) return;

  // Show loading state while ReplayViewer loads
  app.innerHTML = `
    <div class="replay-page">
      <h1 class="page-title">Replay Viewer</h1>
      <div id="replay-loading" style="text-align: center; padding: 60px 20px; color: var(--text-muted);">
        Loading replay viewer...
      </div>
    </div>
  `;

  // Lazy load ReplayViewer and initialize
  loadReplayViewer().then(({ ReplayViewer }) => {
    initReplayViewerWithClass(ReplayViewer, params.url);
  });
}

function initReplayViewerWithClass(ReplayViewerClass: any, initialUrl?: string): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="replay-page">
      <h1 class="page-title">Replay Viewer</h1>

      <div class="replay-layout">
        <div class="replay-main">
          <div class="canvas-wrapper">
            <canvas id="replay-canvas"></canvas>
            <div id="no-replay" class="no-replay-message">Load a replay file to view</div>
          </div>
          <div id="win-prob-section" class="win-prob-section" style="display:none">
            <div class="win-prob-header">
              <span class="win-prob-title">Win Probability</span>
              <div class="critical-moment-nav">
                <button id="prev-critical-btn" class="btn" title="Previous critical moment" disabled>&#9664; Prev</button>
                <span id="critical-moment-info" class="critical-moment-info">&#8212;</span>
                <button id="next-critical-btn" class="btn" title="Next critical moment" disabled>Next &#9654;</button>
              </div>
            </div>
            <div id="win-prob-container" class="win-prob-container"></div>
            <div class="win-prob-legend">
              <span id="wp-p0-label" class="wp-legend-p0">&#8212; Player 0</span>
              <span id="wp-p1-label" class="wp-legend-p1">-- Player 1</span>
            </div>
          </div>
        </div>

        <div class="replay-sidebar">
          <div class="panel">
            <h2>Load Replay</h2>
            <div class="load-controls">
              <div class="file-input-wrapper">
                <label class="btn secondary" for="file-input">Choose File</label>
                <input type="file" id="file-input" accept=".json" style="display: none;">
              </div>
              <div class="url-input-group">
                <input type="text" id="url-input" placeholder="Or enter URL...">
                <button id="load-url-btn" class="btn primary">Load</button>
              </div>
            </div>
          </div>

          <div class="panel">
            <h2>Playback</h2>
            <div class="playback-controls">
              <button id="play-btn" class="btn" disabled>Play</button>
              <button id="prev-btn" class="btn" disabled>Prev</button>
              <button id="next-btn" class="btn" disabled>Next</button>
              <button id="reset-btn" class="btn" disabled>Reset</button>
            </div>
            <div class="slider-group">
              <label>Turn: <span id="turn-display">0</span> / <span id="total-turns">0</span></label>
              <input type="range" id="turn-slider" min="0" max="0" value="0" disabled>
            </div>
            <div class="slider-group">
              <label>Speed: <span id="speed-display">100</span>ms/turn</label>
              <input type="range" id="speed-slider" min="20" max="1000" value="100">
            </div>
          </div>

          <div class="panel">
            <h2>View Options</h2>
            <div class="view-options">
              <label for="fog-select">Fog of War:</label>
              <select id="fog-select">
                <option value="">Disabled (full view)</option>
              </select>
              <label for="cell-size-select" style="margin-top: 10px;">Cell Size:</label>
              <select id="cell-size-select">
                <option value="6">Small (6px)</option>
                <option value="8">Medium (8px)</option>
                <option value="10" selected>Large (10px)</option>
                <option value="12">X-Large (12px)</option>
              </select>
            </div>
          </div>

          <div class="panel">
            <h2>Accessibility</h2>
            <div class="accessibility-options">
              <label class="checkbox-label">
                <input type="checkbox" id="color-blind-toggle" checked>
                Color-blind safe palette
              </label>
              <label class="checkbox-label">
                <input type="checkbox" id="shapes-toggle" checked>
                Shapes per player
              </label>
              <label class="checkbox-label">
                <input type="checkbox" id="high-contrast-toggle">
                High contrast mode
              </label>
              <label class="checkbox-label">
                <input type="checkbox" id="reduced-motion-toggle">
                Reduced motion
              </label>
            </div>
          </div>

          <div class="panel">
            <h2>Match Info</h2>
            <dl class="match-info">
              <dt>Match ID</dt>
              <dd id="info-match-id">-</dd>
              <dt>Winner</dt>
              <dd id="info-winner">-</dd>
              <dt>Turns</dt>
              <dd id="info-turns">-</dd>
              <dt>Reason</dt>
              <dd id="info-reason">-</dd>
            </dl>
          </div>

          <div class="panel">
            <h2>Events This Turn</h2>
            <div class="event-log" id="event-log">
              <div class="no-events">No events</div>
            </div>
          </div>

          <div class="keyboard-shortcuts">
            <kbd>Space</kbd> Play/Pause
            <kbd>←</kbd><kbd>→</kbd> Step
            <kbd>Home</kbd><kbd>End</kbd> First/Last
          </div>
        </div>
      </div>
    </div>

    <style>
      .replay-page .page-title { margin-bottom: 20px; }
      .replay-layout { display: flex; gap: 20px; }
      .replay-main { flex: 1; min-width: 0; }
      .canvas-wrapper { background-color: var(--bg-secondary); border-radius: 8px; padding: 10px; overflow: auto; max-height: 80vh; }
      #replay-canvas { display: block; }
      .no-replay-message { color: var(--text-muted); text-align: center; padding: 60px 20px; }
      .replay-sidebar { width: 300px; flex-shrink: 0; display: flex; flex-direction: column; gap: 15px; }
      .panel { background-color: var(--bg-secondary); border-radius: 8px; padding: 15px; }
      .panel h2 { font-size: 1rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 12px; }
      .load-controls { display: flex; flex-direction: column; gap: 10px; }
      .url-input-group { display: flex; gap: 8px; }
      .url-input-group input { flex: 1; background-color: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 8px; border-radius: 6px; font-size: 14px; }
      .playback-controls { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
      .playback-controls .btn:disabled { opacity: 0.5; cursor: not-allowed; }
      .slider-group { margin-bottom: 10px; }
      .slider-group label { display: block; color: var(--text-muted); font-size: 0.875rem; margin-bottom: 6px; }
      .slider-group input[type="range"] { width: 100%; }
      .view-options { display: flex; flex-direction: column; }
      .view-options label { color: var(--text-muted); font-size: 0.875rem; margin-bottom: 6px; }
      .view-options select { background-color: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 8px; border-radius: 6px; font-size: 14px; }
      .accessibility-options { display: flex; flex-direction: column; gap: 8px; }
      .checkbox-label { display: flex; align-items: center; gap: 8px; cursor: pointer; color: var(--text-muted); font-size: 0.875rem; }
      .checkbox-label input[type="checkbox"] { width: 16px; height: 16px; accent-color: var(--accent); cursor: pointer; }
      .checkbox-label:hover { color: var(--text-primary); }
      .match-info dt { color: var(--text-muted); font-size: 0.75rem; text-transform: uppercase; margin-top: 10px; }
      .match-info dd { color: var(--text-primary); }
      .event-log { max-height: 150px; overflow-y: auto; font-size: 0.75rem; font-family: monospace; }
      .event-log .event { padding: 4px 0; border-bottom: 1px solid var(--bg-tertiary); }
      .event-log .event:last-child { border-bottom: none; }
      .no-events { color: var(--text-muted); }
      .keyboard-shortcuts { font-size: 0.75rem; color: var(--text-muted); }
      .keyboard-shortcuts kbd { background-color: var(--bg-tertiary); padding: 2px 6px; border-radius: 4px; font-family: monospace; margin-right: 4px; }
      .win-prob-section { background-color: var(--bg-secondary); border-radius: 8px; padding: 12px; margin-top: 10px; }
      .win-prob-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; flex-wrap: wrap; gap: 8px; }
      .win-prob-title { color: var(--text-muted); font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; font-weight: 600; }
      .critical-moment-nav { display: flex; align-items: center; gap: 8px; }
      .critical-moment-nav .btn { padding: 4px 10px; font-size: 0.75rem; }
      .critical-moment-nav .btn:disabled { opacity: 0.4; cursor: not-allowed; }
      .critical-moment-info { color: var(--text-muted); font-size: 0.8rem; max-width: 280px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .win-prob-container { width: 100%; overflow: hidden; border-radius: 4px; }
      .win-prob-legend { display: flex; gap: 16px; margin-top: 6px; font-size: 0.75rem; }
      .wp-legend-p0 { color: #3b82f6; }
      .wp-legend-p1 { color: #ef4444; }
      @media (max-width: 900px) {
        .replay-layout { flex-direction: column; }
        .replay-sidebar { width: 100%; }
      }
    </style>
  `;

  initReplayViewer(ReplayViewerClass, initialUrl);
}

function initReplayViewer(ReplayViewerClass: any, initialUrl?: string): void {
  const canvas = document.getElementById('replay-canvas') as HTMLCanvasElement;
  const noReplayDiv = document.getElementById('no-replay') as HTMLDivElement;
  const fileInput = document.getElementById('file-input') as HTMLInputElement;
  const urlInput = document.getElementById('url-input') as HTMLInputElement;
  const loadUrlBtn = document.getElementById('load-url-btn') as HTMLButtonElement;
  const playBtn = document.getElementById('play-btn') as HTMLButtonElement;
  const prevBtn = document.getElementById('prev-btn') as HTMLButtonElement;
  const nextBtn = document.getElementById('next-btn') as HTMLButtonElement;
  const resetBtn = document.getElementById('reset-btn') as HTMLButtonElement;
  const turnDisplay = document.getElementById('turn-display') as HTMLSpanElement;
  const totalTurnsSpan = document.getElementById('total-turns') as HTMLSpanElement;
  const turnSlider = document.getElementById('turn-slider') as HTMLInputElement;
  const speedDisplay = document.getElementById('speed-display') as HTMLSpanElement;
  const speedSlider = document.getElementById('speed-slider') as HTMLInputElement;
  const fogSelect = document.getElementById('fog-select') as HTMLSelectElement;
  const cellSizeSelect = document.getElementById('cell-size-select') as HTMLSelectElement;
  const eventLogDiv = document.getElementById('event-log') as HTMLDivElement;
  const infoMatchId = document.getElementById('info-match-id') as HTMLElement;
  const infoWinner = document.getElementById('info-winner') as HTMLElement;
  const infoTurns = document.getElementById('info-turns') as HTMLElement;
  const infoReason = document.getElementById('info-reason') as HTMLElement;
  const winProbSection = document.getElementById('win-prob-section') as HTMLDivElement;
  const winProbContainer = document.getElementById('win-prob-container') as HTMLDivElement;
  const prevCriticalBtn = document.getElementById('prev-critical-btn') as HTMLButtonElement;
  const nextCriticalBtn = document.getElementById('next-critical-btn') as HTMLButtonElement;
  const criticalMomentInfo = document.getElementById('critical-moment-info') as HTMLSpanElement;
  const wpP0Label = document.getElementById('wp-p0-label') as HTMLSpanElement;
  const wpP1Label = document.getElementById('wp-p1-label') as HTMLSpanElement;

  let viewer = new ReplayViewerClass(canvas, { cellSize: 10 });
  let criticalMoments: Array<{turn: number; delta: number; description: string}> = [];

  function enableControls(): void {
    playBtn.disabled = false;
    prevBtn.disabled = false;
    nextBtn.disabled = false;
    resetBtn.disabled = false;
    turnSlider.disabled = false;
    noReplayDiv.style.display = 'none';
  }

  function updateUI(): void {
    turnDisplay.textContent = String(viewer.getTurn());
    totalTurnsSpan.textContent = String(viewer.getTotalTurns());
    turnSlider.value = String(viewer.getTurn());
    playBtn.textContent = 'Pause';
    if (!viewer.getReplay() || viewer.isAtEnd()) {
      playBtn.textContent = 'Play';
    }
  }

  function updateEventLog(): void {
    const events = viewer.getTurnEvents();
    if (events.length === 0) {
      eventLogDiv.innerHTML = '<div class="no-events">No events</div>';
      return;
    }
    eventLogDiv.innerHTML = events.map((e: GameEvent) => {
      const type = e.type.replace(/_/g, ' ');
      return `<div class="event"><span style="color: #fbbf24;">${type}</span></div>`;
    }).join('');
  }

  function updateMatchInfo(replay: Replay): void {
    infoMatchId.textContent = replay.match_id;
    infoTurns.textContent = String(replay.result.turns);
    infoReason.textContent = replay.result.reason;

    if (replay.result.winner >= 0 && replay.result.winner < replay.players.length) {
      infoWinner.textContent = replay.players[replay.result.winner].name;
    } else if (replay.result.winner === -1) {
      infoWinner.textContent = 'Draw';
    } else {
      infoWinner.textContent = 'Player ' + replay.result.winner;
    }

    fogSelect.innerHTML = '<option value="">Disabled (full view)</option>';
    replay.players.forEach((player, idx) => {
      const option = document.createElement('option');
      option.value = String(idx);
      option.textContent = player.name;
      fogSelect.appendChild(option);
    });
  }

  function loadReplay(replay: Replay): void {
    viewer.loadReplay(replay);
    enableControls();
    updateMatchInfo(replay);
    turnSlider.max = String(viewer.getTotalTurns() - 1);
    updateUI();
    updateEventLog();
    initWinProb(replay);
  }

  function initWinProb(replay: Replay): void {
    if (!replay.win_prob || replay.win_prob.length === 0) {
      winProbSection.style.display = 'none';
      return;
    }

    const points = replay.win_prob.map((pair: any, t: number) => ({
      turn: t,
      p0WinProb: pair[0] ?? 0.5,
      p1WinProb: pair[1] ?? 0.5,
      drawProb: Math.max(0, 1 - (pair[0] ?? 0.5) - (pair[1] ?? 0.5)),
    }));

    criticalMoments = replay.critical_moments ?? [];

    viewer.setWinProbabilityData(points);
    viewer.setCriticalMoments(criticalMoments);

    winProbSection.style.display = 'block';

    if (replay.players.length >= 1) wpP0Label.textContent = `— ${replay.players[0].name}`;
    if (replay.players.length >= 2) wpP1Label.textContent = `-- ${replay.players[1].name}`;

    winProbContainer.innerHTML = '';
    viewer.createWinProbSparkline(winProbContainer, 800, 70, (turn: number) => {
      viewer.setTurn(turn);
      updateUI();
      updateEventLog();
    });

    updateCriticalMomentNav();
  }

  function updateCriticalMomentNav(): void {
    const hasMoments = criticalMoments.length > 0;
    prevCriticalBtn.disabled = !hasMoments;
    nextCriticalBtn.disabled = !hasMoments;

    if (hasMoments) {
      const currentTurn = viewer.getTurn();
      const atMoment = criticalMoments.find((m: any) => m.turn === currentTurn);
      if (atMoment) {
        criticalMomentInfo.textContent = atMoment.description;
      } else {
        criticalMomentInfo.textContent = `${criticalMoments.length} critical moment${criticalMoments.length !== 1 ? 's' : ''}`;
      }
    } else {
      criticalMomentInfo.textContent = '—';
    }
  }

  prevCriticalBtn.addEventListener('click', () => {
    const currentTurn = viewer.getTurn();
    const prev = [...criticalMoments].reverse().find((m: any) => m.turn < currentTurn);
    if (prev) {
      viewer.setTurn(prev.turn);
      updateUI();
      updateEventLog();
      criticalMomentInfo.textContent = prev.description;
    }
  });

  nextCriticalBtn.addEventListener('click', () => {
    const currentTurn = viewer.getTurn();
    const next = criticalMoments.find((m: any) => m.turn > currentTurn);
    if (next) {
      viewer.setTurn(next.turn);
      updateUI();
      updateEventLog();
      criticalMomentInfo.textContent = next.description;
    }
  });

  fileInput.addEventListener('change', async (e) => {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      const replay = JSON.parse(text) as Replay;
      loadReplay(replay);
    } catch (err) {
      alert('Failed to load replay: ' + err);
    }
  });

  loadUrlBtn.addEventListener('click', async () => {
    const url = urlInput.value.trim();
    if (!url) return;
    try {
      const response = await fetch(url);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const replay = await response.json() as Replay;
      loadReplay(replay);
    } catch (err) {
      alert('Failed to load replay from URL: ' + err);
    }
  });

  playBtn.addEventListener('click', () => viewer.togglePlay());
  prevBtn.addEventListener('click', () => { viewer.setTurn(viewer.getTurn() - 1); updateUI(); updateEventLog(); });
  nextBtn.addEventListener('click', () => { viewer.setTurn(viewer.getTurn() + 1); updateUI(); updateEventLog(); });
  resetBtn.addEventListener('click', () => { viewer.pause(); viewer.setTurn(0); updateUI(); updateEventLog(); });

  turnSlider.addEventListener('input', () => {
    viewer.setTurn(parseInt(turnSlider.value, 10));
    updateUI();
    updateEventLog();
  });

  speedSlider.addEventListener('input', () => {
    const speed = parseInt(speedSlider.value, 10);
    viewer.setSpeed(speed);
    speedDisplay.textContent = String(speed);
  });

  fogSelect.addEventListener('change', () => {
    const value = fogSelect.value;
    viewer.setFogOfWar(value === '' ? null : parseInt(value, 10));
  });

  cellSizeSelect.addEventListener('change', () => {
    const size = parseInt(cellSizeSelect.value, 10);
    const replay = viewer.getReplay();
    if (replay) {
      const prevTurn = viewer.getTurn();
      viewer.destroy();
      viewer = new ReplayViewerClass(canvas, { cellSize: size });
      loadReplay(replay);
      viewer.setTurn(prevTurn);
      updateUI();
    }
  });

  // Accessibility toggle handlers
  const colorBlindToggle = document.getElementById('color-blind-toggle') as HTMLInputElement;
  const shapesToggle = document.getElementById('shapes-toggle') as HTMLInputElement;
  const highContrastToggle = document.getElementById('high-contrast-toggle') as HTMLInputElement;
  const reducedMotionToggle = document.getElementById('reduced-motion-toggle') as HTMLInputElement;

  function updateAccessibility(): void {
    viewer.setAccessibility({
      colorBlindSafe: colorBlindToggle.checked,
      showShapes: shapesToggle.checked,
      highContrast: highContrastToggle.checked,
      reducedMotion: reducedMotionToggle.checked,
    });
  }

  colorBlindToggle.addEventListener('change', updateAccessibility);
  shapesToggle.addEventListener('change', updateAccessibility);
  highContrastToggle.addEventListener('change', updateAccessibility);
  reducedMotionToggle.addEventListener('change', updateAccessibility);

  // Initialize accessibility from system preferences
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    reducedMotionToggle.checked = true;
    updateAccessibility();
  }

  viewer.onTurnChange = () => {
    updateUI();
    updateEventLog();
    if (criticalMoments.length > 0) updateCriticalMomentNav();
  };
  viewer.onPlayStateChange = (playing: boolean) => { playBtn.textContent = playing ? 'Pause' : 'Play'; };

  document.addEventListener('keydown', (e) => {
    if (!viewer.getReplay()) return;
    switch (e.code) {
      case 'Space':
        e.preventDefault();
        viewer.togglePlay();
        break;
      case 'ArrowLeft':
        e.preventDefault();
        viewer.setTurn(viewer.getTurn() - 1);
        updateUI();
        updateEventLog();
        break;
      case 'ArrowRight':
        e.preventDefault();
        viewer.setTurn(viewer.getTurn() + 1);
        updateUI();
        updateEventLog();
        break;
      case 'Home':
        e.preventDefault();
        viewer.setTurn(0);
        updateUI();
        updateEventLog();
        break;
      case 'End':
        e.preventDefault();
        viewer.setTurn(viewer.getTotalTurns() - 1);
        updateUI();
        updateEventLog();
        break;
    }
  });

  // Load from URL param if provided
  if (initialUrl) {
    urlInput.value = initialUrl;
    loadUrlBtn.click();
  }
}

// Docs/Getting Started page
function renderDocsPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="docs-page">
      <h1 class="page-title">Getting Started</h1>

      <div class="docs-content">
        <section>
          <h2>Overview</h2>
          <p>AI Code Battle is a competitive bot programming platform. You write an HTTP server that controls units on a grid world, competing against other bots for supremacy.</p>
        </section>

        <section>
          <h2>Game Basics</h2>
          <ul>
            <li><strong>Grid:</strong> The game is played on a toroidal (wrapping) grid</li>
            <li><strong>Units:</strong> Each player controls bots that move one tile per turn</li>
            <li><strong>Resources:</strong> Collect energy from nodes to spawn new bots</li>
            <li><strong>Objectives:</strong> Capture enemy cores, eliminate opponents, or dominate through numbers</li>
          </ul>
        </section>

        <section>
          <h2>HTTP Protocol</h2>
          <p>Your bot must expose an HTTPS endpoint that accepts POST requests with JSON game state and returns JSON move commands.</p>

          <h3>Request Format</h3>
          <pre><code>{
  "match_id": "abc123",
  "turn": 42,
  "player_id": 0,
  "config": { ... },
  "visible_grid": { ... },
  "my_bots": [
    { "id": "bot-1", "position": {"row": 10, "col": 20} }
  ],
  "my_energy": 5,
  "my_score": 3
}</code></pre>

          <h3>Response Format</h3>
          <pre><code>{
  "moves": [
    { "bot_id": "bot-1", "direction": "N" }
  ]
}</code></pre>

          <h3>Valid Directions</h3>
          <p><code>N</code> (North), <code>E</code> (East), <code>S</code> (South), <code>W</code> (West)</p>
        </section>

        <section>
          <h2>Authentication</h2>
          <p>Requests from the game engine are signed with HMAC-SHA256. The signature is sent in the <code>X-Signature</code> header.</p>
          <p>Format: <code>{match_id}.{turn}.{timestamp}.{sha256(body)}</code></p>
          <p>Your bot should verify signatures using your API key to ensure requests are authentic.</p>
        </section>

        <section>
          <h2>Requirements</h2>
          <ul>
            <li>HTTPS endpoint accessible from the internet</li>
            <li>Response time under 3 seconds per turn</li>
            <li>Handle concurrent requests (multiple matches)</li>
            <li>Return valid JSON for every request</li>
          </ul>
        </section>

        <section>
          <h2>Example Bot</h2>
          <p>See the <a href="https://github.com/aicodebattle/acb/tree/main/bots" target="_blank">example bots</a> in various languages for reference implementations.</p>
        </section>

        <section>
          <h2>Data &amp; API</h2>
          <p>All match data (leaderboards, replays, bot profiles) is exposed as static JSON files served from CDN.</p>
          <p><a href="#/compete/docs" class="btn secondary">View API Reference</a></p>
        </section>
      </div>
    </div>

    <style>
      .docs-content { max-width: 800px; }
      .docs-content section { background-color: var(--bg-secondary); border-radius: 8px; padding: 20px; margin-bottom: 20px; }
      .docs-content h2 { color: var(--text-primary); margin-bottom: 12px; }
      .docs-content h3 { color: var(--text-primary); margin: 16px 0 8px; font-size: 1rem; }
      .docs-content p { color: var(--text-muted); margin-bottom: 10px; }
      .docs-content ul { color: var(--text-muted); margin-left: 20px; }
      .docs-content li { margin-bottom: 6px; }
      .docs-content pre { background-color: var(--bg-primary); border-radius: 6px; padding: 16px; overflow-x: auto; margin: 10px 0; }
      .docs-content code { font-family: 'Fira Code', 'Monaco', monospace; font-size: 0.875rem; color: var(--text-secondary); }
      .docs-content a { color: var(--accent); }
    </style>
  `;
}

// 404 page
function renderNotFoundPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="not-found-page">
      <h1>404</h1>
      <p>Page not found</p>
      <a href="#/" class="btn primary">Go Home</a>
    </div>

    <style>
      .not-found-page { text-align: center; padding: 100px 20px; }
      .not-found-page h1 { font-size: 4rem; color: var(--text-primary); margin-bottom: 10px; }
      .not-found-page p { color: var(--text-muted); margin-bottom: 20px; }
    </style>
  `;
}

// ─── Utilities ───────────────────────────────────────────────────────────────────

function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// ─── Navigation & UI ───────────────────────────────────────────────────────────────

// Update active nav link on route change
function updateActiveNavLink(): void {
  const currentPath = router.getCurrentPath();

  // Clear all active states
  document.querySelectorAll('.nav-link').forEach(link => {
    link.classList.remove('active');
  });

  // Set active state for matching links
  document.querySelectorAll('.nav-link').forEach(link => {
    const href = link.getAttribute('href');
    if (href) {
      const linkPath = href.slice(2); // Remove '#/'
      // Check for exact match or prefix match for hub pages
      if (currentPath === linkPath ||
          (linkPath !== '' && currentPath.startsWith(linkPath)) ||
          (linkPath === '/watch' && currentPath.startsWith('/watch')) ||
          (linkPath === '/compete' && currentPath.startsWith('/compete'))) {
        link.classList.add('active');
      }
    }
  });
}

// Mobile menu toggle
function initMobileMenu(): void {
  const toggle = document.getElementById('mobile-menu-toggle');
  const menu = document.getElementById('mobile-menu');

  if (!toggle || !menu) return;

  toggle.addEventListener('click', () => {
    menu.classList.toggle('open');
  });

  // Close menu when clicking outside
  document.addEventListener('click', (e) => {
    if (!menu.contains(e.target as Node) && !toggle.contains(e.target as Node)) {
      menu.classList.remove('open');
    }
  });

  // Close menu on route change
  const originalNavigate = router.navigate.bind(router);
  router.navigate = (path: string) => {
    originalNavigate(path);
    menu.classList.remove('open');
  };
}

// Initialize mobile menu on DOM ready
initMobileMenu();

// Override router navigation to update nav links
const originalNavigate = router.navigate.bind(router);
router.navigate = (path: string) => {
  originalNavigate(path);
  updateActiveNavLink();
};

// ─── Route definitions ─────────────────────────────────────────────────────────────

router
  // Main routes
  .on('/', lazyRoute(loadHomePage))
  .on('/watch', renderWatchHubPage)
  .on('/watch/replays', lazyRoute(loadMatchesPage))
  .on('/watch/replay/:id', renderReplayPage)
  .on('/watch/series/:id', lazyRoute(loadSeriesPage))
  .on('/watch/predictions', lazyRoute(loadPredictionsPage))
  .on('/watch/series', lazyRoute(loadSeriesPage))
  .on('/compete', renderCompeteHubPage)
  .on('/compete/sandbox', lazyRoute(loadSandboxPage))
  .on('/compete/register', lazyRoute(loadRegisterPage))
  .on('/compete/bot/:id', lazyRoute(loadBotProfilePage))
  .on('/compete/docs', renderDocsPage)
  .on('/leaderboard', lazyRoute(loadLeaderboardPage))
  .on('/evolution', lazyRoute(loadEvolutionPage))
  .on('/blog', lazyRoute(async () => (await loadBlogPages()).renderBlogPage))
  .on('/blog/:slug', lazyRoute(async () => (await loadBlogPages()).renderBlogPostPage))
  .on('/season/:id', renderSeasonDetailPage)
  .on('/seasons', lazyRoute(loadSeasonsPage))
  .on('/bot/:id', lazyRoute(loadBotProfilePage))
  // Backwards compatibility redirects
  .on('/matches', redirect('/watch/replays'))
  .on('/playlists', redirect('/watch/replays'))
  .on('/replay', redirect('/watch/replay'))
  .on('/predictions', redirect('/watch/predictions'))
  .on('/series', redirect('/watch/series'))
  .on('/sandbox', redirect('/compete/sandbox'))
  .on('/register', redirect('/compete/register'))
  .on('/bots', redirect('/leaderboard'))
  .on('/docs', redirect('/compete/docs'))
  .on('/docs/api', redirect('/compete/docs'))
  .on('/clip-maker', redirect('/watch/replays'))
  .on('/rivalries', redirect('/watch/replays'))
  .on('/feedback', redirect('/compete/docs'))
  .notFound(renderNotFoundPage);

// ─── Initialization ────────────────────────────────────────────────────────────────

// Start the router - Agentation is no longer auto-loaded on every page
document.addEventListener('DOMContentLoaded', () => {
  updateActiveNavLink();
  router.start();
  // Agentation removed from auto-init - now loads only when needed
});

// Update nav on initial load
window.addEventListener('load', () => {
  updateActiveNavLink();
});
