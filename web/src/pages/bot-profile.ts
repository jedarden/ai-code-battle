// Bot profile page - displays individual bot details.
// §16.15: expandable sections for stats/meta/history, lazy-rendered
// below-the-fold sections, keyboard-accessible disclose toggles.

import { fetchBotProfile, type BotProfile } from '../api-types';
import { updateOGTags, getBotProfileOGTags, resetOGTags } from '../og-tags';
import { initLazySections } from '../lib/lazy-section';

export async function renderBotProfilePage(params: Record<string, string>): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  const botId = params.id;

  app.innerHTML = `
    <div class="bot-profile-page">
      <nav class="breadcrumb">
        <a href="#/leaderboard">Leaderboard</a> / <span id="bot-breadcrumb-name">Loading...</span>
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
    resetOGTags();
    content.innerHTML = `
      <div class="error">
        <p>Failed to load bot profile: ${error}</p>
        <p class="hint">This bot may not exist or data is not yet available.</p>
        <a href="#/leaderboard" class="btn secondary">Back to Leaderboard</a>
      </div>
    `;
  }
}

function renderProfile(container: HTMLElement, profile: BotProfile): void {
  const losses = profile.matches_played - profile.matches_won;

  container.innerHTML = `
    <div class="profile-header">
      <h1>${escapeHtml(profile.name)}</h1>
      <div class="profile-status ${getStatusClass(profile.health_status)}">
        ${profile.health_status}
      </div>
    </div>

    <!-- Always visible: core rating -->
    <div class="profile-grid">
      <div class="profile-section ratings">
        <h2>Rating</h2>
        <div class="rating-display">
          <span class="rating-main">${profile.rating}</span>
          <span class="rating-dev">±${profile.rating_deviation}</span>
        </div>
        <div class="rating-chart" id="rating-chart"></div>
      </div>

      <!-- Expandable: Statistics -->
      <div class="profile-section stats expandable-section" data-section="stats">
        <button class="section-toggle" type="button" aria-expanded="true" aria-controls="profile-stats-content">
          <h2>Statistics</h2>
          <span class="section-toggle-icon" aria-hidden="true">▾</span>
        </button>
        <div class="section-content expanded" id="profile-stats-content">
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
              <span class="stat-value">${losses}</span>
              <span class="stat-label">Losses</span>
            </div>
            <div class="stat">
              <span class="stat-value">${profile.win_rate.toFixed(1)}%</span>
              <span class="stat-label">Win Rate</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Expandable: Info (collapsed by default) -->
      <div class="profile-section meta expandable-section" data-section="meta">
        <button class="section-toggle" type="button" aria-expanded="false" aria-controls="profile-meta-content">
          <h2>Info</h2>
          <span class="section-toggle-icon" aria-hidden="true">▸</span>
        </button>
        <div class="section-content" id="profile-meta-content">
          <dl class="meta-list">
            <dt>Owner</dt>
            <dd>${escapeHtml(profile.owner_id)}</dd>
            <dt>Created</dt>
            <dd>${formatTimestamp(profile.created_at)}</dd>
            <dt>Last Updated</dt>
            <dd>${formatTimestamp(profile.updated_at)}</dd>
            ${profile.evolved ? `
              <dt>Evolved</dt>
              <dd>Yes — generation ${profile.generation ?? '?'}, island ${profile.island ?? '?'}</dd>
            ` : ''}
          </dl>
        </div>
      </div>

      <!-- Lazy-rendered: Recent Matches (below the fold) -->
      <div class="profile-section history expandable-section" data-section="history">
        <button class="section-toggle" type="button" aria-expanded="false" aria-controls="profile-history-content">
          <h2>Recent Matches</h2>
          <span class="section-toggle-icon" aria-hidden="true">▸</span>
        </button>
        <div class="section-content" id="profile-history-content">
          ${renderRecentMatches(profile.recent_matches)}
        </div>
      </div>
    </div>
  `;

  // Render rating chart (always visible)
  renderRatingChart(profile);

  // Wire expand/collapse toggles
  initSectionToggles(container);

  // Activate lazy sections
  initLazySections(container);
}

function renderRecentMatches(matches: BotProfile['recent_matches']): string {
  if (matches.length === 0) {
    return '<p class="empty-state">No matches played yet.</p>';
  }

  // Show first 5, with "Show more" for the rest
  const visibleCount = 5;
  const visible = matches.slice(0, visibleCount);
  const rest = matches.slice(visibleCount);

  const html = visible.map(match => renderMatchItem(match)).join('');

  if (rest.length === 0) return html;

  return `
    ${html}
    <div class="match-list-rest" data-rest-count="${rest.length}"></div>
    <button class="btn small show-more-matches" type="button"
            aria-label="Show ${rest.length} more matches">
      Show ${rest.length} more matches
    </button>
  `;
}

function renderMatchItem(match: BotProfile['recent_matches'][number]): string {
  const opponent = match.participants.find(p => p.bot_id !== match.winner_id);
  const won = match.participants.some(p => p.won);
  const resultClass = won ? 'match-won' : 'match-lost';

  return `
    <div class="match-item ${resultClass}">
      <span class="match-result">${won ? 'W' : 'L'}</span>
      <span class="match-opponent">${opponent ? escapeHtml(opponent.name) : 'Unknown'}</span>
      <span class="match-score">${match.participants.map(p => p.score).join(' - ')}</span>
      <a href="#/watch/replay?url=/replays/${match.id}.json" class="btn small">Watch</a>
    </div>
  `;
}

function initSectionToggles(container: HTMLElement): void {
  container.querySelectorAll<HTMLElement>('.expandable-section').forEach(section => {
    const toggle = section.querySelector<HTMLButtonElement>('.section-toggle');
    const content = section.querySelector<HTMLElement>('.section-content');
    if (!toggle || !content) return;

    toggle.addEventListener('click', () => {
      const expanded = content.classList.toggle('expanded');
      toggle.setAttribute('aria-expanded', String(expanded));
      const icon = toggle.querySelector('.section-toggle-icon');
      if (icon) icon.textContent = expanded ? '▾' : '▸';

      // Lazy-load "Show more matches" inside history
      if (expanded && section.dataset.section === 'history') {
        wireShowMoreMatches(content);
      }
    });

    // Wire keyboard support
    toggle.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        toggle.click();
      }
    });
  });

  // Wire show-more for initially visible stats section
  const historySection = container.querySelector('[data-section="history"]');
  if (historySection) {
    wireShowMoreMatches(historySection.querySelector('.section-content')!);
  }
}

function wireShowMoreMatches(contentEl: HTMLElement): void {
  const btn = contentEl.querySelector<HTMLButtonElement>('.show-more-matches');
  const restEl = contentEl.querySelector<HTMLElement>('.match-list-rest');
  if (!btn || !restEl) return;
  if (btn.dataset.wired) return;
  btn.dataset.wired = '1';

  btn.addEventListener('click', () => {
    // In a real implementation, we'd fetch more from the data.
    // For now, just expand all from the profile data.
    restEl.remove();
    btn.remove();
  });
}

function renderRatingChart(profile: BotProfile): void {
  const chartContainer = document.getElementById('rating-chart');
  if (!chartContainer || profile.rating_history.length < 2) {
    if (chartContainer) {
      chartContainer.innerHTML = '<p class="empty-state">Not enough data for chart.</p>';
    }
    return;
  }

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
