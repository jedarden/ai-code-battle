// Series Page - Browse multi-game series between bots with bracket visualization
import type { Series, SeriesIndex, SeriesGame } from '../types';

function bracketRoundLabel(round?: string): string {
  switch (round) {
    case 'quarterfinal': return 'Quarterfinals';
    case 'semifinal': return 'Semifinals';
    case 'final': return 'Final';
    default: return '';
  }
}

const PAGES_BASE = '';

export async function renderSeriesPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="series-page">
      <h1 class="page-title">Series</h1>
      <p class="page-subtitle">Best-of-N matchups between bots</p>

      <div class="series-filters">
        <select id="status-filter">
          <option value="">All Status</option>
          <option value="active">In Progress</option>
          <option value="completed">Completed</option>
          <option value="pending">Upcoming</option>
        </select>
        <select id="bot-filter">
          <option value="">All Bots</option>
        </select>
      </div>

      <div class="spoiler-toggle">
        <input type="checkbox" id="spoiler-toggle">
        <label for="spoiler-toggle">Hide spoilers (scores/results)</label>
      </div>

      <div class="series-list" id="series-list">
        <div class="loading">Loading series...</div>
      </div>

      <div class="series-detail" id="series-detail" style="display: none;">
        <button class="back-btn" id="back-btn">&larr; Back to Series</button>
        <div id="series-detail-content"></div>
      </div>
    </div>

    <style>
      .series-page {
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

      .series-filters {
        display: flex;
        gap: 12px;
        margin-bottom: 20px;
      }

      .series-filters select {
        background-color: var(--bg-secondary);
        border: 1px solid var(--border);
        color: var(--text-primary);
        padding: 8px 12px;
        border-radius: 6px;
        font-size: 14px;
      }

      .spoiler-toggle {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 16px;
      }

      .spoiler-toggle input {
        width: 16px;
        height: 16px;
      }

      .spoiler-hidden .bracket-progress,
      .spoiler-hidden .bracket-dot,
      .spoiler-hidden .game-result-text {
        filter: blur(4px);
        cursor: pointer;
      }

      .series-list {
        display: flex;
        flex-direction: column;
        gap: 12px;
      }

      .series-card {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 16px;
        cursor: pointer;
        transition: transform 0.2s, box-shadow 0.2s;
      }

      .series-card:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
      }

      .series-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 12px;
      }

      .series-matchup {
        display: flex;
        align-items: center;
        gap: 16px;
        flex: 1;
      }

      .series-bot {
        display: flex;
        flex-direction: column;
        align-items: center;
        min-width: 100px;
      }

      .series-bot-name {
        font-weight: 500;
        color: var(--text-primary);
      }

      .series-vs {
        font-size: 0.875rem;
        color: var(--text-muted);
        font-weight: 600;
      }

      /* Bracket progress bar: horizontal track with dots */
      .bracket-container {
        margin-top: 12px;
        padding-top: 12px;
        border-top: 1px solid var(--border);
      }

      .bracket-labels {
        display: flex;
        justify-content: space-between;
        margin-bottom: 6px;
      }

      .bracket-label {
        font-size: 0.75rem;
        font-weight: 600;
      }

      .bracket-label.bot-a { color: #3b82f6; }
      .bracket-label.bot-b { color: #ef4444; }

      .bracket-track {
        display: flex;
        align-items: center;
        gap: 4px;
        justify-content: center;
      }

      .bracket-dot {
        width: 28px;
        height: 28px;
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 0.65rem;
        font-weight: 700;
        border: 2px solid var(--border);
        background-color: var(--bg-tertiary);
        color: var(--text-muted);
        transition: all 0.2s;
      }

      .bracket-dot.win-a {
        background-color: #3b82f6;
        border-color: #3b82f6;
        color: white;
      }

      .bracket-dot.win-b {
        background-color: #ef4444;
        border-color: #ef4444;
        color: white;
      }

      .bracket-dot.draw {
        background-color: #6b7280;
        border-color: #6b7280;
        color: white;
      }

      .bracket-dot.pending {
        background-color: var(--bg-tertiary);
        border-color: var(--border);
        color: var(--text-muted);
        border-style: dashed;
      }

      .bracket-connector {
        width: 12px;
        height: 2px;
        background-color: var(--border);
      }

      .series-meta {
        display: flex;
        justify-content: space-between;
        color: var(--text-muted);
        font-size: 0.75rem;
        margin-top: 8px;
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
      .status-badge.pending { background-color: #6b7280; color: white; }

      /* Detail view styles */
      .detail-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 24px;
      }

      .detail-score {
        display: flex;
        align-items: center;
        gap: 12px;
        font-size: 2.5rem;
        font-weight: 700;
        margin-bottom: 24px;
        justify-content: center;
      }

      .detail-score .score-a { color: #3b82f6; }
      .detail-score .score-b { color: #ef4444; }
      .detail-score .score-dash { color: var(--text-muted); }

      /* Bracket tree visualization */
      .bracket-tree {
        display: flex;
        flex-direction: column;
        gap: 6px;
        margin-bottom: 24px;
        padding: 16px;
        background-color: var(--bg-secondary);
        border-radius: 8px;
      }

      .bracket-tree-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 12px;
        padding-bottom: 8px;
        border-bottom: 1px solid var(--border);
      }

      .bracket-tree-header h3 {
        font-size: 0.8rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: var(--text-muted);
        margin: 0;
      }

      .bracket-round-label {
        font-size: 0.7rem;
        color: var(--text-muted);
        font-weight: 600;
        padding: 4px 0 2px;
        text-transform: uppercase;
        letter-spacing: 0.04em;
      }

      .bracket-matchup {
        display: flex;
        align-items: center;
        padding: 8px 12px;
        background-color: var(--bg-tertiary);
        border-radius: 6px;
        gap: 8px;
        border-left: 3px solid var(--border);
      }

      .bracket-matchup.team-a-win { border-left-color: #3b82f6; }
      .bracket-matchup.team-b-win { border-left-color: #ef4444; }

      .bracket-seed {
        font-size: 0.65rem;
        color: var(--text-muted);
        font-weight: 700;
        min-width: 20px;
      }

      .bracket-team {
        flex: 1;
        font-size: 0.8rem;
        font-weight: 500;
      }

      .bracket-team.winner { color: var(--accent); }
      .bracket-team.loser { color: var(--text-muted); text-decoration: line-through; }

      .bracket-score {
        font-family: monospace;
        font-size: 0.8rem;
        font-weight: 700;
        min-width: 16px;
        text-align: center;
      }

      .bracket-score.won { color: #22c55e; }
      .bracket-score.lost { color: #ef4444; }

      .bracket-vs {
        font-size: 0.65rem;
        color: var(--text-muted);
      }

      .map-type-label {
        font-size: 0.6rem;
        color: var(--text-muted);
        opacity: 0.7;
        min-width: 60px;
        text-align: center;
        text-transform: uppercase;
        letter-spacing: 0.03em;
      }

      /* Progress bar for series */
      .series-progress-bar {
        height: 6px;
        background-color: var(--bg-tertiary);
        border-radius: 3px;
        overflow: hidden;
        margin-top: 8px;
      }

      .series-progress-fill {
        height: 100%;
        border-radius: 3px;
        transition: width 0.3s;
      }

      .series-progress-fill.bot-a-fill { background-color: #3b82f6; }
      .series-progress-fill.bot-b-fill { background-color: #ef4444; }

      /* Large bracket for detail view */
      .detail-bracket {
        display: flex;
        flex-direction: column;
        gap: 8px;
        margin-bottom: 24px;
        padding: 16px;
        background-color: var(--bg-secondary);
        border-radius: 8px;
      }

      .game-row {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 10px 12px;
        background-color: var(--bg-tertiary);
        border-radius: 6px;
        border-left: 3px solid transparent;
        transition: background-color 0.15s;
      }

      .game-row:hover {
        background-color: rgba(255, 255, 255, 0.05);
      }

      .game-row.win-a { border-left-color: #3b82f6; }
      .game-row.win-b { border-left-color: #ef4444; }
      .game-row.draw { border-left-color: #6b7280; }
      .game-row.pending { border-left-color: var(--border); }

      .game-number {
        font-weight: 600;
        color: var(--text-muted);
        min-width: 60px;
        font-size: 0.8rem;
      }

      .game-result-text {
        flex: 1;
        color: var(--text-primary);
        font-size: 0.875rem;
      }

      .game-result-text.winner-a { color: #3b82f6; }
      .game-result-text.winner-b { color: #ef4444; }

      .game-badge {
        display: inline-block;
        width: 8px;
        height: 8px;
        border-radius: 50%;
        margin-right: 6px;
      }

      .game-badge.win-a { background-color: #3b82f6; }
      .game-badge.win-b { background-color: #ef4444; }
      .game-badge.draw { background-color: #6b7280; }

      .watch-btn {
        background-color: var(--accent);
        color: white;
        border: none;
        padding: 4px 10px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 12px;
      }

      .watch-btn:hover {
        opacity: 0.9;
      }

      .loading {
        color: var(--text-muted);
        text-align: center;
        padding: 40px;
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

      .empty-message {
        color: var(--text-muted);
        text-align: center;
        padding: 40px;
      }
    </style>
  `;

  // Load series data
  await loadSeries();

  // Setup spoiler toggle
  document.getElementById('spoiler-toggle')?.addEventListener('change', (e) => {
    const checked = (e.target as HTMLInputElement).checked;
    document.querySelector('.series-list')?.classList.toggle('spoiler-hidden', checked);
  });
}

async function loadSeries(): Promise<void> {
  const list = document.getElementById('series-list');
  const botFilter = document.getElementById('bot-filter') as HTMLSelectElement;
  const statusFilter = document.getElementById('status-filter') as HTMLSelectElement;

  if (!list) return;

  try {
    const response = await fetch(`${PAGES_BASE}/data/series/index.json`);
    if (!response.ok) throw new Error('Failed to load series');
    const index: SeriesIndex = await response.json();

    if (index.series.length === 0) {
      list.innerHTML = '<div class="empty-message">No series available yet</div>';
      return;
    }

    // Build bot name map from series data (names are already included in the JSON)
    const botNames = new Map<string, string>();
    const bots = new Set<string>();
    index.series.forEach((s: Series) => {
      bots.add(s.bot1_id);
      bots.add(s.bot2_id);
      if (s.bot1_name) botNames.set(s.bot1_id, s.bot1_name);
      if (s.bot2_name) botNames.set(s.bot2_id, s.bot2_name);
    });

    // Update filter options
    bots.forEach(botId => {
      const option = document.createElement('option');
      option.value = botId;
      option.textContent = botNames.get(botId) || botId;
      botFilter.appendChild(option);
    });

    // Render series cards with bracket visualization
    renderSeriesList(index.series, list);

    // Filter handlers
    const applyFilters = () => {
      const statusVal = statusFilter.value;
      const botVal = botFilter.value;
      const filtered = index.series.filter((s: Series) => {
        if (statusVal && s.status !== statusVal) return false;
        if (botVal && s.bot1_id !== botVal && s.bot2_id !== botVal) return false;
        return true;
      });
      renderSeriesList(filtered, list);
    };

    statusFilter.addEventListener('change', applyFilters);
    botFilter.addEventListener('change', applyFilters);

  } catch (err) {
    console.error('Failed to load series:', err);
    list.innerHTML = '<div class="empty-message">Failed to load series. Please try again later.</div>';
  }
}

function renderBracketProgress(bot1Name: string, bot2Name: string, games: SeriesGame[], format: number): string {
  const dots: string[] = [];
  for (let i = 0; i < format; i++) {
    const game = games[i];
    if (!game || !game.winner_id) {
      dots.push(`<div class="bracket-dot pending">${i + 1}</div>`);
    } else if (game.winner_slot === 0) {
      dots.push(`<div class="bracket-dot win-a">${i + 1}</div>`);
    } else if (game.winner_slot === 1) {
      dots.push(`<div class="bracket-dot win-b">${i + 1}</div>`);
    } else {
      dots.push(`<div class="bracket-dot draw">${i + 1}</div>`);
    }
  }

  return `
    <div class="bracket-container">
      <div class="bracket-labels">
        <span class="bracket-label bot-a">${bot1Name}</span>
        <span class="bracket-label bot-b">${bot2Name}</span>
      </div>
      <div class="bracket-track">
        ${dots.join('<div class="bracket-connector"></div>')}
      </div>
    </div>
  `;
}

function renderSeriesList(series: Series[], container: HTMLElement): void {
  container.innerHTML = series.map(s => {
    const winsNeeded = Math.ceil(s.best_of / 2);
    const totalProgress = s.bot1_wins + s.bot2_wins;
    const aPct = (s.bot1_wins / winsNeeded) * 100;
    const bPct = (s.bot2_wins / winsNeeded) * 100;
    const roundLabel = bracketRoundLabel(s.bracket_round);

    return `
    <div class="series-card" data-series-id="${s.id}">
      <div class="series-header">
        <div class="series-matchup">
          <div class="series-bot">
            <span class="series-bot-name">${s.bot1_name}</span>
          </div>
          <span class="series-vs">vs</span>
          <div class="series-bot">
            <span class="series-bot-name">${s.bot2_name}</span>
          </div>
        </div>
        ${roundLabel ? `<span class="bracket-round-badge">${roundLabel}</span>` : ''}
      </div>
      ${renderBracketProgress(s.bot1_name, s.bot2_name, s.games || [], s.best_of)}
      <div class="series-progress-bar">
        <div class="series-progress-fill bot-a-fill" style="width: ${Math.min(aPct, 100)}%; float: left;"></div>
        <div class="series-progress-fill bot-b-fill" style="width: ${Math.min(bPct, 100)}%; float: right;"></div>
      </div>
      <div class="series-meta">
        <span class="status-badge ${s.status}">${s.status}</span>
        <span>Best of ${s.best_of} &middot; ${s.bot1_wins}-${s.bot2_wins} ${totalProgress < s.best_of ? `(${s.best_of - totalProgress} games left)` : ''}</span>
        <span>${s.completed_at ? new Date(s.completed_at).toLocaleDateString() : 'In progress'}</span>
      </div>
    </div>
  `;
  }).join('');

  // Wire click handlers
  container.querySelectorAll('.series-card').forEach(card => {
    card.addEventListener('click', () => {
      const seriesId = (card as HTMLElement).dataset.seriesId;
      if (seriesId) showSeriesDetail(seriesId);
    });
  });
}

function renderBracketTree(series: Series): string {
  const games = series.games || [];
  const winsNeeded = Math.ceil(series.best_of / 2);

  // Map types per §14.7: game 1 = classic, 2 = corridors, 3 = open, 4+ = untested
  function mapTypeLabel(gameNum: number): string {
    switch (gameNum) {
      case 1: return 'Classic';
      case 2: return 'Corridors';
      case 3: return 'Open Field';
      case 4: return 'New Terrain';
      default: return 'Random';
    }
  }

  // Render each game as a matchup row in the bracket tree
  const gameRows = games.map((g, idx) => {
    const isAWin = g.winner_slot === 0;
    const isBWin = g.winner_slot === 1;
    const isDecider = series.bot1_wins === winsNeeded - 1 && series.bot2_wins === winsNeeded - 1 && !g.winner_id;
    const rowClass = isAWin ? 'team-a-win' : isBWin ? 'team-b-win' : '';
    const mapLabel = mapTypeLabel(idx + 1);

    return `
      <div class="bracket-matchup ${rowClass}">
        <span class="bracket-seed">${idx + 1}</span>
        <span class="bracket-team ${isAWin ? 'winner' : isBWin ? 'loser' : ''}">${series.bot1_name}</span>
        <span class="bracket-score ${isAWin ? 'won' : isBWin ? 'lost' : ''}">${isAWin ? 'W' : isBWin ? '' : '-'}</span>
        <span class="bracket-vs">vs</span>
        <span class="bracket-score ${isBWin ? 'won' : isAWin ? 'lost' : ''}">${isBWin ? 'W' : isAWin ? '' : '-'}</span>
        <span class="bracket-team ${isBWin ? 'winner' : isAWin ? 'loser' : ''}">${series.bot2_name}</span>
        <span class="map-type-label">${mapLabel}</span>
        ${g.match_id ? `<button class="watch-btn" data-match-id="${g.match_id}" style="margin-left: auto; font-size: 0.7rem; padding: 2px 8px;">Watch</button>` : ''}
        ${isDecider ? '<span style="color: gold; font-size: 0.7rem; font-weight: 600; margin-left: 4px;">DECIDER</span>' : ''}
      </div>
    `;
  });

  // Add remaining games if series is still active
  const remainingGames = series.best_of - games.length;
  if (remainingGames > 0 && series.status !== 'completed') {
    for (let i = games.length; i < series.best_of; i++) {
      const isDecider = series.bot1_wins === winsNeeded - 1 && series.bot2_wins === winsNeeded - 1;
      const mapLabel = mapTypeLabel(i + 1);
      gameRows.push(`
        <div class="bracket-matchup" style="opacity: 0.5;">
          <span class="bracket-seed">${i + 1}</span>
          <span class="bracket-team">${series.bot1_name}</span>
          <span class="bracket-score">-</span>
          <span class="bracket-vs">vs</span>
          <span class="bracket-score">-</span>
          <span class="bracket-team">${series.bot2_name}</span>
          <span class="map-type-label">${mapLabel}</span>
          ${isDecider ? '<span style="color: gold; font-size: 0.7rem; font-weight: 600; margin-left: 4px;">DECIDER</span>' : ''}
        </div>
      `);
    }
  }

  // Group games into rounds for longer series
  const rounds: string[][] = [];
  if (series.best_of <= 5) {
    rounds.push(gameRows);
  } else {
    // For bo7: group games 1-4, 5-7
    rounds.push(gameRows.slice(0, 4));
    if (gameRows.length > 4) rounds.push(gameRows.slice(4));
  }

  return rounds.map((round, ri) => {
    const roundLabel = rounds.length > 1
      ? (ri === 0 ? 'Games 1-4' : `Games 5-${series.best_of}`)
      : `Games 1-${series.best_of}`;
    return `
      <div class="bracket-round-label">${roundLabel}</div>
      ${round.join('')}
    `;
  }).join('');
}

async function showSeriesDetail(seriesId: string): Promise<void> {
  const list = document.getElementById('series-list');
  const detail = document.getElementById('series-detail');
  const detailContent = document.getElementById('series-detail-content');
  const backBtn = document.getElementById('back-btn');
  const spoilerToggle = document.getElementById('spoiler-toggle') as HTMLInputElement;
  const spoilerActive = spoilerToggle?.checked;

  if (!list || !detail || !detailContent) return;

  try {
    const response = await fetch(`${PAGES_BASE}/data/series/${seriesId}.json`);
    if (!response.ok) throw new Error('Series not found');
    const series: Series = await response.json();

    const gamesHtml = (series.games || []).map(g => {
      const winnerClass = g.winner_slot === 0 ? 'win-a' : g.winner_slot === 1 ? 'win-b' : 'draw';
      const resultClass = g.winner_slot === 0 ? 'winner-a' : g.winner_slot === 1 ? 'winner-b' : '';
      const winnerName = g.winner_slot === 0 ? series.bot1_name : g.winner_slot === 1 ? series.bot2_name : 'Draw';
      const badgeClass = g.winner_slot === 0 ? 'win-a' : g.winner_slot === 1 ? 'win-b' : 'draw';

      const resultText = spoilerActive
        ? '***'
        : (g.completed_at
          ? (g.winner_id ? `Winner: ${winnerName}` : 'Draw')
          : 'Not yet played');

      return `
        <div class="game-row ${winnerClass}">
          <span class="game-number">Game ${g.game_number}</span>
          <span class="game-badge ${badgeClass}"></span>
          <span class="game-result-text ${resultClass}">
            ${resultText}
            ${g.turns ? ` (${g.turns} turns)` : ''}
          </span>
          ${g.match_id ? `<button class="watch-btn" data-match-id="${g.match_id}">Watch</button>` : ''}
        </div>
      `;
    }).join('');

    detailContent.innerHTML = `
      <div class="detail-header">
        <h2>${series.bot1_name} vs ${series.bot2_name}</h2>
        <span class="status-badge ${series.status}">${series.status}</span>
      </div>

      <div class="detail-score">
        <span class="score-a">${series.bot1_wins}</span>
        <span class="score-dash">-</span>
        <span class="score-b">${series.bot2_wins}</span>
      </div>

      ${renderBracketProgress(series.bot1_name, series.bot2_name, series.games || [], series.best_of)}

      <div class="bracket-tree">
        <div class="bracket-tree-header">
          <h3>Bracket</h3>
          <span style="font-size: 0.75rem; color: var(--text-muted);">Best of ${series.best_of}</span>
        </div>
        ${renderBracketTree(series)}
      </div>

      <div class="detail-bracket">
        <h3 style="margin-bottom: 12px; font-size: 0.875rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em;">Game Results</h3>
        ${gamesHtml}
      </div>
    `;

    // Wire watch buttons
    detailContent.querySelectorAll('.watch-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const matchId = (btn as HTMLElement).dataset.matchId;
        if (matchId) {
          window.location.hash = `/replay?match=${matchId}`;
        }
      });
    });

    // Hide spoilers in detail view if active
    if (spoilerActive) {
      detailContent.querySelectorAll('.game-result-text, .bracket-dot').forEach(el => {
        el.classList.add('spoiler-blur');
      });
    }

    list.style.display = 'none';
    detail.style.display = 'block';

    // Also hide the filters and spoiler toggle
    const filters = document.querySelector('.series-filters') as HTMLElement;
    const spoilerDiv = document.querySelector('.spoiler-toggle') as HTMLElement;
    if (filters) filters.style.display = 'none';
    if (spoilerDiv) spoilerDiv.style.display = 'none';

    backBtn!.onclick = () => {
      detail.style.display = 'none';
      list.style.display = 'flex';
      if (filters) filters.style.display = 'flex';
      if (spoilerDiv) spoilerDiv.style.display = 'flex';
    };

  } catch (err) {
    console.error('Failed to load series:', err);
    alert('Failed to load series details');
  }
}
