// Seasons Page - Browse seasonal competitions with per-season rankings
import type { Season, SeasonIndex, SeasonSnapshot, ChampionshipBracketSeries } from '../types';

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
        flex-wrap: wrap;
        gap: 12px;
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

      /* Active season mini-leaderboard */
      .mini-leaderboard {
        margin-top: 16px;
        padding-top: 16px;
        border-top: 1px solid var(--border);
      }

      .mini-leaderboard h4 {
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: var(--text-muted);
        margin-bottom: 8px;
      }

      .mini-leaderboard-row {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 4px 0;
        font-size: 0.8rem;
      }

      .mini-leaderboard-row .rank {
        font-weight: 700;
        width: 24px;
        text-align: center;
        color: var(--text-muted);
      }

      .mini-leaderboard-row .rank-1 { color: gold; }
      .mini-leaderboard-row .rank-2 { color: silver; }
      .mini-leaderboard-row .rank-3 { color: #cd7f32; }

      .mini-leaderboard-row .bot-name {
        flex: 1;
        color: var(--text-primary);
      }

      .mini-leaderboard-row .bot-rating {
        font-family: monospace;
        color: var(--text-muted);
      }

      .mini-leaderboard-row .bot-record {
        font-size: 0.7rem;
        color: var(--text-muted);
        min-width: 60px;
        text-align: right;
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

      /* Detail view leaderboard */
      .leaderboard-table {
        width: 100%;
        border-collapse: collapse;
        background-color: var(--bg-secondary);
        border-radius: 8px;
        overflow: hidden;
        margin-bottom: 24px;
      }

      .leaderboard-table th,
      .leaderboard-table td {
        padding: 12px 16px;
        text-align: left;
        border-bottom: 1px solid var(--bg-tertiary);
      }

      .leaderboard-table th {
        background-color: var(--bg-tertiary);
        color: var(--text-muted);
        font-weight: 600;
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
      }

      .leaderboard-table .rank {
        font-weight: 700;
        color: var(--text-muted);
      }

      .leaderboard-table tr.rank-1 .rank { color: #fbbf24; }
      .leaderboard-table tr.rank-2 .rank { color: #94a3b8; }
      .leaderboard-table tr.rank-3 .rank { color: #cd7f32; }

      .leaderboard-table .bot-name-cell {
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .leaderboard-table .win-bar {
        height: 4px;
        border-radius: 2px;
        background-color: #22c55e;
        min-width: 2px;
      }

      .leaderboard-table .loss-bar {
        height: 4px;
        border-radius: 2px;
        background-color: #ef4444;
        min-width: 2px;
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

      /* Stats summary row */
      .stats-row {
        display: flex;
        gap: 16px;
        margin-bottom: 24px;
        flex-wrap: wrap;
      }

      .stat-card {
        flex: 1;
        min-width: 120px;
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 16px;
        text-align: center;
      }

      .stat-value {
        font-size: 1.5rem;
        font-weight: 700;
        color: var(--text-primary);
      }

      .stat-label {
        font-size: 0.7rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: var(--text-muted);
        margin-top: 4px;
      }

      /* Championship bracket */
      .championship-bracket {
        margin-top: 24px;
        padding: 16px;
        background-color: var(--bg-secondary);
        border-radius: 8px;
      }

      .championship-bracket h3 {
        margin-bottom: 16px;
        font-size: 0.875rem;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
      }

      .bracket-round {
        margin-bottom: 16px;
      }

      .bracket-round-title {
        font-size: 0.7rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: var(--text-muted);
        margin-bottom: 8px;
        font-weight: 600;
      }

      .bracket-series-row {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 8px 12px;
        background-color: var(--bg-tertiary);
        border-radius: 6px;
        margin-bottom: 4px;
        border-left: 3px solid var(--border);
        transition: background-color 0.15s;
      }

      .bracket-series-row:hover {
        background-color: rgba(255, 255, 255, 0.05);
      }

      .bracket-series-row.completed {
        border-left-color: #3b82f6;
      }

      .bracket-series-row.active {
        border-left-color: #22c55e;
      }

      .bracket-series-row .seed {
        font-size: 0.65rem;
        color: var(--text-muted);
        font-weight: 700;
        min-width: 20px;
      }

      .bracket-series-row .team {
        flex: 1;
        font-size: 0.8rem;
        font-weight: 500;
      }

      .bracket-series-row .team.winner { color: #22c55e; }
      .bracket-series-row .team.loser { color: var(--text-muted); text-decoration: line-through; }

      .bracket-series-row .series-score {
        font-family: monospace;
        font-size: 0.8rem;
        font-weight: 700;
        color: var(--text-primary);
        min-width: 30px;
        text-align: center;
      }

      .bracket-series-row .series-status {
        font-size: 0.6rem;
        text-transform: uppercase;
        padding: 1px 6px;
        border-radius: 3px;
      }

      .bracket-series-row .series-status.active { background-color: #22c55e; color: white; }
      .bracket-series-row .series-status.completed { background-color: #3b82f6; color: white; }
      .bracket-series-row .series-status.pending { background-color: #6b7280; color: white; }

      .bracket-spacer {
        height: 8px;
      }
    </style>
  `;

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

      // For active seasons, also load the live leaderboard to show current standings
      let liveSnapshot: SeasonSnapshot[] | null = index.active_season.final_snapshot;
      if (!liveSnapshot || liveSnapshot.length === 0) {
        try {
          const lbResponse = await fetch(`${PAGES_BASE}/data/leaderboard.json`);
          if (lbResponse.ok) {
            const lb = await lbResponse.json();
            if (lb.entries && lb.entries.length > 0) {
              // Convert leaderboard entries to snapshot format for the active season
              liveSnapshot = lb.entries.slice(0, 20).map((entry: any, idx: number) => ({
                bot_id: entry.bot_id,
                bot_name: entry.name,
                rating: entry.rating,
                rank: idx + 1,
                wins: entry.wins || 0,
                losses: entry.losses || 0,
              }));
            }
          }
        } catch {
          // Live leaderboard fetch failed — show season without leaderboard
        }
      }

      activeSeasonContent.innerHTML = renderActiveSeason({
        ...index.active_season,
        final_snapshot: liveSnapshot,
      });

      activeSeasonContent.querySelector('.season-card')?.addEventListener('click', () => {
        showSeasonDetail(index.active_season!.id);
      });
    }

    if (index.seasons.length === 0) {
      list.innerHTML = '<div class="empty-message">No seasons available yet</div>';
      return;
    }

    list.innerHTML = index.seasons.map((s: Season) => `
      <div class="season-card" data-season-id="${s.id}">
        <h3>${escapeHtml(s.name)}</h3>
        ${s.champion_name ? `
          <div class="champion">
            <span class="champion-crown">&#x1F451;</span>
            <span class="champion-name">${escapeHtml(s.champion_name)}</span>
          </div>
        ` : ''}
        <div class="meta">
          <span class="status-badge ${s.status}">${s.status}</span>
          <span>${s.total_matches} matches</span>
        </div>
      </div>
    `).join('');

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

function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
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

  // Build mini-leaderboard from snapshot if available
  let miniLeaderboard = '';
  if (season.final_snapshot && season.final_snapshot.length > 0) {
    const top5 = season.final_snapshot.slice(0, 5);
    miniLeaderboard = `
      <div class="mini-leaderboard">
        <h4>Top Bots</h4>
        ${top5.map((entry: SeasonSnapshot) => `
          <div class="mini-leaderboard-row">
            <span class="rank rank-${entry.rank}">#${entry.rank}</span>
            <span class="bot-name">${escapeHtml(entry.bot_name)}</span>
            <span class="bot-rating">${Math.round(entry.rating)}</span>
            <span class="bot-record">${entry.wins}W ${entry.losses}L</span>
          </div>
        `).join('')}
      </div>
    `;
  }

  return `
    <div class="season-card" data-season-id="${season.id}">
      <div class="season-header">
        <div class="season-info">
          <h3>${escapeHtml(season.name)}</h3>
          <p class="season-theme">${escapeHtml(season.theme)}</p>
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
      ${miniLeaderboard}
    </div>
  `;
}

function renderChampionshipBracket(bracket: ChampionshipBracketSeries[]): string {
  if (!bracket || bracket.length === 0) return '';

  // Group by round
  const rounds = new Map<string, ChampionshipBracketSeries[]>();
  const roundOrder = ['quarterfinal', 'semifinal', 'final'];
  for (const s of bracket) {
    const round = s.round || 'quarterfinal';
    if (!rounds.has(round)) rounds.set(round, []);
    rounds.get(round)!.push(s);
  }

  // Sort each round by bracket_position
  for (const [, series] of rounds) {
    series.sort((a, b) => a.bracket_position - b.bracket_position);
  }

  const roundLabels: Record<string, string> = {
    quarterfinal: 'Quarterfinals',
    semifinal: 'Semifinals',
    final: 'Final',
  };

  const html = roundOrder
    .filter(r => rounds.has(r))
    .map(round => {
      const series = rounds.get(round)!;
      return `
        <div class="bracket-round">
          <div class="bracket-round-title">${roundLabels[round] || round}</div>
          ${series.map(s => {
            const isCompleted = s.status === 'completed';
            const isActive = s.status === 'active';
            const bot1Won = s.winner_id === s.bot1_id;
            const bot2Won = s.winner_id === s.bot2_id;
            return `
              <div class="bracket-series-row ${isCompleted ? 'completed' : isActive ? 'active' : ''}">
                <span class="team ${bot1Won ? 'winner' : isCompleted && !bot1Won ? 'loser' : ''}">${escapeHtml(s.bot1_name)}</span>
                <span class="series-score">${s.bot1_wins}-${s.bot2_wins}</span>
                <span class="team ${bot2Won ? 'winner' : isCompleted && !bot2Won ? 'loser' : ''}">${escapeHtml(s.bot2_name)}</span>
                <span class="series-status ${s.status}">${s.status}</span>
              </div>
            `;
          }).join('')}
        </div>
      `;
    }).join('<div class="bracket-spacer"></div>');

  return `
    <div class="championship-bracket">
      <h3>Championship Bracket</h3>
      ${html}
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

    // Compute max wins/losses for bar scaling
    const snapshotForScale = season.final_snapshot || [];
    const maxGames = snapshotForScale.reduce((max: number, e: SeasonSnapshot) => {
      return Math.max(max, e.wins, e.losses);
    }, 1) || 1;

    // For active seasons without snapshot, load live leaderboard
    let leaderboardData = season.final_snapshot;
    let isLiveData = false;
    if ((!leaderboardData || leaderboardData.length === 0) && season.status === 'active') {
      try {
        const lbResponse = await fetch(`${PAGES_BASE}/data/leaderboard.json`);
        if (lbResponse.ok) {
          const lb = await lbResponse.json();
          if (lb.entries && lb.entries.length > 0) {
            leaderboardData = lb.entries.slice(0, 30).map((entry: any, idx: number) => ({
              bot_id: entry.bot_id,
              bot_name: entry.name,
              rating: entry.rating,
              rank: idx + 1,
              wins: entry.wins || 0,
              losses: entry.losses || 0,
            }));
            isLiveData = true;
          }
        }
      } catch {
        // Live leaderboard fetch failed
      }
    }

    const leaderboardHtml = leaderboardData && leaderboardData.length > 0
      ? `
        <table class="leaderboard-table">
          <thead>
            <tr>
              <th>Rank</th>
              <th>Bot</th>
              <th>Rating</th>
              <th>Record</th>
              <th>Win Rate</th>
            </tr>
          </thead>
          <tbody>
            ${leaderboardData.map((entry: SeasonSnapshot) => {
              const total = entry.wins + entry.losses;
              const winRate = total > 0 ? (entry.wins / total * 100).toFixed(0) : '-';
              const winWidth = maxGames > 0 ? (entry.wins / maxGames * 60) : 0;
              const lossWidth = maxGames > 0 ? (entry.losses / maxGames * 60) : 0;
              return `
              <tr class="rank-${entry.rank}">
                <td class="rank">#${entry.rank}</td>
                <td>${escapeHtml(entry.bot_name)}</td>
                <td style="font-family: monospace">${Math.round(entry.rating)}</td>
                <td>${entry.wins}W / ${entry.losses}L</td>
                <td>
                  <div style="display: flex; gap: 2px; align-items: center;">
                    <div class="win-bar" style="width: ${winWidth}px;"></div>
                    <div class="loss-bar" style="width: ${lossWidth}px;"></div>
                    <span style="margin-left: 6px; font-size: 0.75rem; color: var(--text-muted)">${winRate}%</span>
                  </div>
                </td>
              </tr>
            `}).join('')}
          </tbody>
        </table>
        ${isLiveData ? '<p style="color: var(--text-muted); font-size: 0.7rem; text-align: center; margin-top: 8px;">Live ratings from current season</p>' : ''}
      `
      : '<p style="color: var(--text-muted); text-align: center; padding: 24px;">No leaderboard data available yet.</p>';

    detailContent.innerHTML = `
      <div class="season-header" style="margin-bottom: 24px;">
        <div class="season-info">
          <h2>${escapeHtml(season.name)}</h2>
          <p class="season-theme">${escapeHtml(season.theme)}</p>
        </div>
        <div class="season-dates">
          <span class="status-badge ${season.status}">${season.status}</span>
          <div>Started: ${new Date(season.starts_at).toLocaleDateString()}</div>
          ${season.ends_at ? `<div>Ended: ${new Date(season.ends_at).toLocaleDateString()}</div>` : ''}
        </div>
      </div>

      ${season.champion_name ? `
        <div class="champion" style="justify-content: center; padding: 20px; margin-bottom: 24px;">
          <span class="champion-crown" style="font-size: 2rem;">&#x1F451;</span>
          <div>
            <div style="color: var(--text-muted); font-size: 0.75rem;">CHAMPION</div>
            <span class="champion-name" style="font-size: 1.25rem;">${escapeHtml(season.champion_name)}</span>
          </div>
        </div>
      ` : ''}

      <div class="stats-row">
        <div class="stat-card">
          <div class="stat-value">${season.total_matches}</div>
          <div class="stat-label">Matches Played</div>
        </div>
        ${leaderboardData ? `
          <div class="stat-card">
            <div class="stat-value">${leaderboardData.length}</div>
            <div class="stat-label">Ranked Bots</div>
          </div>
        ` : ''}
        ${leaderboardData && leaderboardData.length > 0 ? `
          <div class="stat-card">
            <div class="stat-value">${Math.round(leaderboardData[0].rating)}</div>
            <div class="stat-label">Highest Rating</div>
          </div>
        ` : ''}
      </div>

      ${season.championship_bracket && season.championship_bracket.length > 0
        ? renderChampionshipBracket(season.championship_bracket)
        : ''}

      <h3 style="margin-bottom: 16px;">${isLiveData ? 'Live Standings' : 'Season Leaderboard'}</h3>
      ${leaderboardHtml}

      <div class="season-rules">
        <h4>Rules Version: ${escapeHtml(season.rules_version)}</h4>
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
