// Bot profile page - displays individual bot details

import { fetchBotProfile, type BotProfile } from '../api-types';
import { updateOGTags, getBotProfileOGTags, resetOGTags } from '../og-tags';

export async function renderBotProfilePage(params: Record<string, string>): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  const botId = params.id;

  app.innerHTML = `
    <div class="bot-profile-page">
      <nav class="breadcrumb">
        <a href="#/bots">Bots</a> / <span id="bot-breadcrumb-name">Loading...</span>
      </nav>
      <div id="profile-content" class="loading">Loading...</div>
    </div>
  `;

  const content = document.getElementById('profile-content');
  const breadcrumbName = document.getElementById('bot-breadcrumb-name');
  if (!content) return;

  try {
    const profile = await fetchBotProfile(botId);
    if (breadcrumbName) breadcrumbName.textContent = profile.name;

    // Update Open Graph tags for social sharing
    updateOGTags(getBotProfileOGTags({
      id: profile.id,
      name: profile.name,
      rating: profile.rating,
      matches_played: profile.matches_played,
      win_rate: profile.win_rate,
      evolved: profile.evolved,
    }));

    renderProfile(content, profile);
  } catch (error) {
    // Reset OG tags on error
    resetOGTags();

    content.innerHTML = `
      <div class="error">
        <p>Failed to load bot profile: ${error}</p>
        <p class="hint">This bot may not exist or data is not yet available.</p>
        <a href="#/bots" class="btn secondary">Back to Bot Directory</a>
      </div>
    `;
  }
}

function renderProfile(container: HTMLElement, profile: BotProfile): void {
  container.innerHTML = `
    <div class="profile-header">
      <h1>${escapeHtml(profile.name)}</h1>
      <div class="profile-status ${getStatusClass(profile.health_status)}">
        ${profile.health_status}
      </div>
    </div>

    <div class="profile-grid">
      <div class="profile-section ratings">
        <h2>Rating</h2>
        <div class="rating-display">
          <span class="rating-main">${profile.rating}</span>
          <span class="rating-dev">±${profile.rating_deviation}</span>
        </div>
        <div class="rating-chart" id="rating-chart"></div>
      </div>

      <div class="profile-section stats">
        <h2>Statistics</h2>
        <div class="stats-grid">
          <div class="stat">
            <span class="stat-value">${profile.matches_played}</span>
            <span class="stat-label">Matches</span>
          </div>
          <div class="stat">
            <span class="stat-value">${profile.matches_won}</span>
            <span class="stat-label">Wins</span>
          </div>
          <div class="stat">
            <span class="stat-value">${profile.win_rate.toFixed(1)}%</span>
            <span class="stat-label">Win Rate</span>
          </div>
        </div>
      </div>

      <div class="profile-section meta">
        <h2>Info</h2>
        <dl class="meta-list">
          <dt>Owner</dt>
          <dd>${escapeHtml(profile.owner_id)}</dd>
          <dt>Created</dt>
          <dd>${formatTimestamp(profile.created_at)}</dd>
          <dt>Last Updated</dt>
          <dd>${formatTimestamp(profile.updated_at)}</dd>
        </dl>
      </div>

      <div class="profile-section history">
        <h2>Recent Matches</h2>
        <div class="matches-list" id="recent-matches">
          ${renderRecentMatches(profile.recent_matches)}
        </div>
      </div>
    </div>
  `;

  // Render simple rating chart if history exists
  renderRatingChart(profile);
}

function renderRecentMatches(matches: BotProfile['recent_matches']): string {
  if (matches.length === 0) {
    return '<p class="empty-state">No matches played yet.</p>';
  }

  return matches.map(match => {
    const opponent = match.participants.find(p => p.bot_id !== match.winner_id);
    const won = match.participants.some(p => p.won);
    const resultClass = won ? 'match-won' : 'match-lost';

    return `
      <div class="match-item ${resultClass}">
        <span class="match-result">${won ? 'W' : 'L'}</span>
        <span class="match-opponent">${opponent ? escapeHtml(opponent.name) : 'Unknown'}</span>
        <span class="match-score">${match.participants.map(p => p.score).join(' - ')}</span>
        <a href="#/replay?url=/replays/${match.id}.json" class="btn small">Watch</a>
      </div>
    `;
  }).join('');
}

function renderRatingChart(profile: BotProfile): void {
  const chartContainer = document.getElementById('rating-chart');
  if (!chartContainer || profile.rating_history.length < 2) {
    if (chartContainer) {
      chartContainer.innerHTML = '<p class="empty-state">Not enough data for chart.</p>';
    }
    return;
  }

  // Simple SVG sparkline
  const history = profile.rating_history;
  const minRating = Math.min(...history.map(h => h.rating));
  const maxRating = Math.max(...history.map(h => h.rating));
  const range = maxRating - minRating || 1;
  const width = 200;
  const height = 60;

  const points = history.map((h, i) => {
    const x = (i / (history.length - 1)) * width;
    const y = height - ((h.rating - minRating) / range) * height;
    return `${x},${y}`;
  }).join(' ');

  chartContainer.innerHTML = `
    <svg class="rating-sparkline" viewBox="0 0 ${width} ${height}" preserveAspectRatio="none">
      <polyline
        points="${points}"
        fill="none"
        stroke="#3b82f6"
        stroke-width="2"
      />
    </svg>
    <div class="rating-range">
      <span>Min: ${Math.round(minRating)}</span>
      <span>Max: ${Math.round(maxRating)}</span>
    </div>
  `;
}

function getStatusClass(status: string): string {
  if (status === 'healthy') return 'status-healthy';
  if (status === 'unhealthy') return 'status-unhealthy';
  return 'status-unknown';
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
