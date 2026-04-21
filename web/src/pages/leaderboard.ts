// Leaderboard page - displays bot rankings

import { fetchLeaderboard, type LeaderboardEntry } from '../api-types';

export async function renderLeaderboardPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="leaderboard-page">
      <h1>Leaderboard</h1>
      <div id="leaderboard-content" class="loading">Loading...</div>
    </div>
  `;

  const content = document.getElementById('leaderboard-content');
  if (!content) return;

  try {
    const data = await fetchLeaderboard();
    renderLeaderboardTable(content, data.entries, data.updated_at);
  } catch (error) {
    content.innerHTML = `
      <div class="error">
        <p>Failed to load leaderboard: ${error}</p>
        <p class="hint">The leaderboard data may not be available yet. Check back after some matches have been played.</p>
      </div>
    `;
  }
}

function renderLeaderboardTable(
  container: HTMLElement,
  entries: LeaderboardEntry[],
  updatedAt: string
): void {
  if (entries.length === 0) {
    container.innerHTML = `
      <div class="empty-state">
        <p>No bots on the leaderboard yet.</p>
        <p>Bots appear here after completing their first match.</p>
        <a href="#/compete/register" class="btn primary">Register a Bot</a>
      </div>
    `;
    return;
  }

  container.innerHTML = `
    <p class="updated-at">Last updated: ${formatTimestamp(updatedAt)}</p>
    <table class="leaderboard-table">
      <thead>
        <tr>
          <th>Rank</th>
          <th>Bot</th>
          <th>Rating</th>
          <th>W/L</th>
          <th>Win Rate</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        ${entries.map(entry => renderLeaderboardRow(entry)).join('')}
      </tbody>
    </table>
  `;
}

function renderLeaderboardRow(entry: LeaderboardEntry): string {
  const rankClass = entry.rank <= 3 ? `rank-${entry.rank}` : '';
  const statusClass = entry.health_status === 'healthy' ? 'status-healthy' :
                      entry.health_status === 'unhealthy' ? 'status-unhealthy' : 'status-unknown';

  return `
    <tr class="${rankClass}">
      <td class="rank">${entry.rank}</td>
      <td class="bot-name">
        <a href="#/bot/${encodeURIComponent(entry.bot_id)}">${escapeHtml(entry.name)}</a>
      </td>
      <td class="rating">
        <span class="rating-value">${entry.rating}</span>
        <span class="rating-dev">±${entry.rating_deviation}</span>
      </td>
      <td class="wl">${entry.matches_won}/${entry.matches_played}</td>
      <td class="win-rate">${entry.win_rate.toFixed(1)}%</td>
      <td class="status ${statusClass}">${entry.health_status}</td>
    </tr>
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
