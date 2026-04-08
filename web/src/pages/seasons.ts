// Seasons Page - Browse seasonal competitions
import type { Season, SeasonIndex, SeasonSnapshot } from '../types';

const PAGES_BASE = '';

export async function renderSeasonsPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="seasons-page">
      <h1 class="page-title">Seasons</h1>
      <p class="page-subtitle">Seasonal competition history and archives</p>

      <div class="active-season" id="active-season" style="display: none;">
        <h2>Current Season</h2>
        <div id="active-season-content"></div>
      </div>

      <div class="seasons-list-section">
        <h2>All Seasons</h2>
        <div class="seasons-list" id="seasons-list">
          <div class="loading">Loading seasons...</div>
        </div>
      </div>

      <div class="season-detail" id="season-detail" style="display: none;">
        <button class="back-btn" id="back-btn">← Back to Seasons</button>
        <div id="season-detail-content"></div>
      </div>
    </div>

    <style>
      .seasons-page {
        max-width: 1000px;
        margin: 0 auto;
      }

      .page-title {
        margin-bottom: 8px;
      }

      .page-subtitle {
        color: var(--text-muted);
        margin-bottom: 24px;
      }

      .active-season {
        background-color: var(--bg-secondary);
        border-radius: 12px;
        padding: 24px;
        margin-bottom: 32px;
        border: 2px solid var(--accent);
      }

      .active-season h2 {
        color: var(--accent);
        margin-bottom: 16px;
      }

      .season-header {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        margin-bottom: 16px;
      }

      .season-info h3 {
        margin-bottom: 4px;
      }

      .season-theme {
        color: var(--text-muted);
        font-size: 0.875rem;
      }

      .season-dates {
        color: var(--text-muted);
        font-size: 0.75rem;
        text-align: right;
      }

      .season-progress {
        margin-top: 16px;
      }

      .progress-bar {
        height: 8px;
        background-color: var(--bg-tertiary);
        border-radius: 4px;
        overflow: hidden;
      }

      .progress-fill {
        height: 100%;
        background-color: var(--accent);
        transition: width 0.3s;
      }

      .progress-label {
        display: flex;
        justify-content: space-between;
        font-size: 0.75rem;
        color: var(--text-muted);
        margin-top: 4px;
      }

      .seasons-list {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
        gap: 16px;
      }

      .season-card {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 16px;
        cursor: pointer;
        transition: transform 0.2s, box-shadow 0.2s;
      }

      .season-card:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
      }

      .season-card h3 {
        margin-bottom: 8px;
      }

      .season-card .champion {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 12px;
        padding: 8px;
        background-color: rgba(255, 215, 0, 0.1);
        border-radius: 4px;
      }

      .champion-crown {
        color: gold;
        font-size: 1.25rem;
      }

      .champion-name {
        font-weight: 600;
        color: var(--text-primary);
      }

      .season-card .meta {
        display: flex;
        justify-content: space-between;
        color: var(--text-muted);
        font-size: 0.75rem;
      }

      .status-badge {
        padding: 2px 8px;
        border-radius: 4px;
        font-size: 0.7rem;
        font-weight: 600;
        text-transform: uppercase;
      }

      .status-badge.active { background-color: #22c55e; color: white; }
      .status-badge.completed { background-color: #3b82f6; color: white; }
      .status-badge.upcoming { background-color: #6b7280; color: white; }

      .loading {
        color: var(--text-muted);
        text-align: center;
        padding: 40px;
        grid-column: 1 / -1;
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

      .leaderboard-table {
        width: 100%;
        border-collapse: collapse;
      }

      .leaderboard-table th,
      .leaderboard-table td {
        padding: 12px;
        text-align: left;
        border-bottom: 1px solid var(--border);
      }

      .leaderboard-table th {
        color: var(--text-muted);
        font-weight: 500;
        font-size: 0.75rem;
        text-transform: uppercase;
      }

      .leaderboard-table .rank-1 {
        color: gold;
        font-weight: 700;
      }

      .leaderboard-table .rank-2 {
        color: silver;
        font-weight: 600;
      }

      .leaderboard-table .rank-3 {
        color: #cd7f32;
        font-weight: 500;
      }

      .empty-message {
        color: var(--text-muted);
        text-align: center;
        padding: 40px;
        grid-column: 1 / -1;
      }

      .season-rules {
        margin-top: 24px;
        padding: 16px;
        background-color: var(--bg-tertiary);
        border-radius: 8px;
      }

      .season-rules h4 {
        margin-bottom: 12px;
        color: var(--text-muted);
      }

      .season-rules ul {
        margin-left: 20px;
        color: var(--text-secondary);
      }

      .season-rules li {
        margin-bottom: 4px;
      }
    </style>
  `;

  // Load seasons data
  await loadSeasons();
}

async function loadSeasons(): Promise<void> {
  const activeSeasonContainer = document.getElementById('active-season');
  const activeSeasonContent = document.getElementById('active-season-content');
  const list = document.getElementById('seasons-list');

  if (!list) return;

  try {
    const response = await fetch(`${PAGES_BASE}/data/seasons/index.json`);
    if (!response.ok) throw new Error('Failed to load seasons');
    const index: SeasonIndex = await response.json();

    // Show active season if present
    if (index.active_season && activeSeasonContainer && activeSeasonContent) {
      activeSeasonContainer.style.display = 'block';
      activeSeasonContent.innerHTML = renderActiveSeason(index.active_season);

      // Wire click handler
      activeSeasonContent.querySelector('.season-card')?.addEventListener('click', () => {
        showSeasonDetail(index.active_season!.id);
      });
    }

    // Render all seasons
    if (index.seasons.length === 0) {
      list.innerHTML = '<div class="empty-message">No seasons available yet</div>';
      return;
    }

    list.innerHTML = index.seasons.map((s: Season) => `
      <div class="season-card" data-season-id="${s.id}">
        <h3>${s.name}</h3>
        ${s.champion_name ? `
          <div class="champion">
            <span class="champion-crown">👑</span>
            <span class="champion-name">${s.champion_name}</span>
          </div>
        ` : ''}
        <div class="meta">
          <span class="status-badge ${s.status}">${s.status}</span>
          <span>${s.total_matches} matches</span>
        </div>
      </div>
    `).join('');

    // Wire click handlers
    list.querySelectorAll('.season-card').forEach(card => {
      card.addEventListener('click', () => {
        const seasonId = (card as HTMLElement).dataset.seasonId;
        if (seasonId) showSeasonDetail(seasonId);
      });
    });

  } catch (err) {
    console.error('Failed to load seasons:', err);
    list.innerHTML = '<div class="empty-message">Failed to load seasons. Please try again later.</div>';
  }
}

function renderActiveSeason(season: Season): string {
  const startDate = new Date(season.starts_at);
  const now = new Date();
  let progressPercent = 0;

  if (season.ends_at) {
    const endDate = new Date(season.ends_at);
    const total = endDate.getTime() - startDate.getTime();
    const elapsed = now.getTime() - startDate.getTime();
    progressPercent = Math.min(100, Math.max(0, (elapsed / total) * 100));
  }

  return `
    <div class="season-card" data-season-id="${season.id}">
      <div class="season-header">
        <div class="season-info">
          <h3>${season.name}</h3>
          <p class="season-theme">${season.theme}</p>
        </div>
        <div class="season-dates">
          <span class="status-badge ${season.status}">${season.status}</span>
          <div>Started: ${startDate.toLocaleDateString()}</div>
          ${season.ends_at ? `<div>Ends: ${new Date(season.ends_at).toLocaleDateString()}</div>` : ''}
        </div>
      </div>
      <div class="season-progress">
        <div class="progress-bar">
          <div class="progress-fill" style="width: ${progressPercent}%"></div>
        </div>
        <div class="progress-label">
          <span>${season.total_matches} matches played</span>
          <span>${Math.round(progressPercent)}% complete</span>
        </div>
      </div>
    </div>
  `;
}

async function showSeasonDetail(seasonId: string): Promise<void> {
  const listSection = document.querySelector('.seasons-list-section') as HTMLElement;
  const activeSeason = document.getElementById('active-season');
  const detail = document.getElementById('season-detail');
  const detailContent = document.getElementById('season-detail-content');
  const backBtn = document.getElementById('back-btn');

  if (!detail || !detailContent) return;

  try {
    const response = await fetch(`${PAGES_BASE}/data/seasons/${seasonId}.json`);
    if (!response.ok) throw new Error('Season not found');
    const season: Season = await response.json();

    detailContent.innerHTML = `
      <div class="season-header" style="margin-bottom: 24px;">
        <div class="season-info">
          <h2>${season.name}</h2>
          <p class="season-theme">${season.theme}</p>
        </div>
        <div class="season-dates">
          <span class="status-badge ${season.status}">${season.status}</span>
          <div>Started: ${new Date(season.starts_at).toLocaleDateString()}</div>
          ${season.ends_at ? `<div>Ended: ${new Date(season.ends_at).toLocaleDateString()}</div>` : ''}
        </div>
      </div>

      ${season.champion_name ? `
        <div class="champion" style="justify-content: center; padding: 20px; margin-bottom: 24px;">
          <span class="champion-crown" style="font-size: 2rem;">👑</span>
          <div>
            <div style="color: var(--text-muted); font-size: 0.75rem;">CHAMPION</div>
            <span class="champion-name" style="font-size: 1.25rem;">${season.champion_name}</span>
          </div>
        </div>
      ` : ''}

      ${season.final_snapshot && season.final_snapshot.length > 0 ? `
        <h3>Final Leaderboard</h3>
        <table class="leaderboard-table">
          <thead>
            <tr>
              <th>Rank</th>
              <th>Bot</th>
              <th>Rating</th>
              <th>W/L</th>
            </tr>
          </thead>
          <tbody>
            ${season.final_snapshot.map((entry: SeasonSnapshot) => `
              <tr>
                <td class="rank-${entry.rank}">#${entry.rank}</td>
                <td>${entry.bot_name}</td>
                <td>${Math.round(entry.rating)}</td>
                <td>${entry.wins}/${entry.losses}</td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      ` : '<p>No leaderboard data available yet.</p>'}

      <div class="season-rules">
        <h4>Rules Version: ${season.rules_version}</h4>
        <ul>
          <li>Standard 60x60 toroidal grid</li>
          <li>500 turn limit</li>
          <li>Glicko-2 rating system</li>
          <li>Best-of-1 matches</li>
        </ul>
      </div>
    `;

    if (listSection) listSection.style.display = 'none';
    if (activeSeason) activeSeason.style.display = 'none';
    detail.style.display = 'block';

    backBtn!.onclick = () => {
      detail.style.display = 'none';
      if (listSection) listSection.style.display = 'block';
      if (activeSeason) activeSeason.style.display = 'block';
    };

  } catch (err) {
    console.error('Failed to load season:', err);
    alert('Failed to load season details');
  }
}
