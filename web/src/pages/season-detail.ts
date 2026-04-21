// Season detail page - standalone page for viewing a specific season
import { router } from '../router';

function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

export function renderSeasonDetailPage(params: Record<string, string>): void {
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
