// Match history page - displays recent matches

import { fetchMatchIndex, type MatchSummary } from '../api-types';

export async function renderMatchesPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="matches-page">
      <h1>Match History</h1>
      <div id="matches-content" class="loading">Loading...</div>
    </div>
  `;

  const content = document.getElementById('matches-content');
  if (!content) return;

  try {
    const data = await fetchMatchIndex();
    renderMatchesList(content, data.matches, data.updated_at);
  } catch (error) {
    content.innerHTML = `
      <div class="error">
        <p>Failed to load match history: ${error}</p>
        <p class="hint">Match data may not be available yet.</p>
      </div>
    `;
  }
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
        <a href="#/replay?url=/replays/${match.id}.json" class="btn small">Watch</a>
      </div>
    </div>
  `;
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
