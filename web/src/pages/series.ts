// Series Page - Browse multi-game series between bots
import type { Series, SeriesIndex } from '../types';
import type { BotProfile } from '../api-types';

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

      <div class="series-list" id="series-list">
        <div class="loading">Loading series...</div>
      </div>

      <div class="series-detail" id="series-detail" style="display: none;">
        <button class="back-btn" id="back-btn">← Back to Series</button>
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

      .series-bot-rating {
        font-size: 0.75rem;
        color: var(--text-muted);
      }

      .series-vs {
        font-size: 0.875rem;
        color: var(--text-muted);
        font-weight: 600;
      }

      .series-score {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 1.25rem;
        font-weight: 600;
      }

      .score-winner {
        color: #22c55e;
      }

      .score-loser {
        color: var(--text-muted);
      }

      .series-meta {
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
      .status-badge.pending { background-color: #6b7280; color: white; }

      .series-games {
        display: flex;
        flex-direction: column;
        gap: 8px;
        margin-top: 16px;
        padding-top: 16px;
        border-top: 1px solid var(--border);
      }

      .game-row {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 8px 12px;
        background-color: var(--bg-tertiary);
        border-radius: 6px;
      }

      .game-number {
        font-weight: 600;
        color: var(--text-muted);
        min-width: 30px;
      }

      .game-result {
        flex: 1;
        color: var(--text-primary);
      }

      .game-result.winner-1 { color: #3b82f6; }
      .game-result.winner-2 { color: #ef4444; }

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

      .spoiler-hidden .series-score,
      .spoiler-hidden .game-result {
        filter: blur(4px);
        cursor: pointer;
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
  const spoilerToggle = document.createElement('div');
  spoilerToggle.className = 'spoiler-toggle';
  spoilerToggle.innerHTML = `
    <input type="checkbox" id="spoiler-toggle">
    <label for="spoiler-toggle">Hide spoilers (scores/results)</label>
  `;
  const seriesList = document.getElementById('series-list');
  seriesList?.parentElement?.insertBefore(spoilerToggle, seriesList);

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

    // Populate bot filter
    const bots = new Set<string>();
    index.series.forEach((s: Series) => {
      bots.add(s.bot1_id);
      bots.add(s.bot2_id);
    });

    // Fetch bot names
    const botNames = new Map<string, string>();
    for (const botId of bots) {
      try {
        const botRes = await fetch(`${PAGES_BASE}/data/bots/${botId}.json`);
        if (botRes.ok) {
          const bot: BotProfile = await botRes.json();
          botNames.set(botId, bot.name);
        }
      } catch {}
    }

    // Update filter options
    bots.forEach(botId => {
      const option = document.createElement('option');
      option.value = botId;
      option.textContent = botNames.get(botId) || botId;
      botFilter.appendChild(option);
    });

    // Render series cards
    renderSeriesList(index.series, list, botNames);

    // Filter handlers
    const applyFilters = () => {
      const statusVal = statusFilter.value;
      const botVal = botFilter.value;
      const filtered = index.series.filter((s: Series) => {
        if (statusVal && s.status !== statusVal) return false;
        if (botVal && s.bot1_id !== botVal && s.bot2_id !== botVal) return false;
        return true;
      });
      renderSeriesList(filtered, list, botNames);
    };

    statusFilter.addEventListener('change', applyFilters);
    botFilter.addEventListener('change', applyFilters);

  } catch (err) {
    console.error('Failed to load series:', err);
    list.innerHTML = '<div class="empty-message">Failed to load series. Please try again later.</div>';
  }
}

function renderSeriesList(series: Series[], container: HTMLElement, _botNames: Map<string, string>): void {
  container.innerHTML = series.map(s => `
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
        <div class="series-score">
          <span class="${s.bot1_wins > s.bot2_wins ? 'score-winner' : 'score-loser'}">${s.bot1_wins}</span>
          <span>-</span>
          <span class="${s.bot2_wins > s.bot1_wins ? 'score-winner' : 'score-loser'}">${s.bot2_wins}</span>
        </div>
      </div>
      <div class="series-meta">
        <span class="status-badge ${s.status}">${s.status}</span>
        <span>Best of ${s.best_of}</span>
        <span>${s.completed_at ? new Date(s.completed_at).toLocaleDateString() : 'In progress'}</span>
      </div>
    </div>
  `).join('');

  // Wire click handlers
  container.querySelectorAll('.series-card').forEach(card => {
    card.addEventListener('click', () => {
      const seriesId = (card as HTMLElement).dataset.seriesId;
      if (seriesId) showSeriesDetail(seriesId);
    });
  });
}

async function showSeriesDetail(seriesId: string): Promise<void> {
  const list = document.getElementById('series-list');
  const detail = document.getElementById('series-detail');
  const detailContent = document.getElementById('series-detail-content');
  const backBtn = document.getElementById('back-btn');

  if (!list || !detail || !detailContent) return;

  try {
    const response = await fetch(`${PAGES_BASE}/data/series/${seriesId}.json`);
    if (!response.ok) throw new Error('Series not found');
    const series: Series = await response.json();

    detailContent.innerHTML = `
      <div class="series-header" style="margin-bottom: 24px;">
        <h2>${series.bot1_name} vs ${series.bot2_name}</h2>
        <span class="status-badge ${series.status}">${series.status}</span>
      </div>

      <div class="series-score" style="justify-content: center; margin-bottom: 24px; font-size: 2rem;">
        <span class="${series.bot1_wins > series.bot2_wins ? 'score-winner' : 'score-loser'}">${series.bot1_wins}</span>
        <span>-</span>
        <span class="${series.bot2_wins > series.bot1_wins ? 'score-winner' : 'score-loser'}">${series.bot2_wins}</span>
      </div>

      <h3>Games</h3>
      <div class="series-games">
        ${series.games.map(g => {
          const winnerClass = g.winner_slot === 0 ? 'winner-1' : g.winner_slot === 1 ? 'winner-2' : '';
          const winnerName = g.winner_slot === 0 ? series.bot1_name : g.winner_slot === 1 ? series.bot2_name : 'Draw';
          return `
            <div class="game-row">
              <span class="game-number">Game ${g.game_number}</span>
              <span class="game-result ${winnerClass}">
                ${g.completed_at ? (g.winner_id ? `Winner: ${winnerName}` : 'Draw') : 'Not played'}
                ${g.turns ? `(${g.turns} turns)` : ''}
              </span>
              ${g.match_id ? `<button class="watch-btn" data-match-id="${g.match_id}">Watch</button>` : ''}
            </div>
          `;
        }).join('')}
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

    list.style.display = 'none';
    detail.style.display = 'block';

    backBtn!.onclick = () => {
      detail.style.display = 'none';
      list.style.display = 'flex';
    };

  } catch (err) {
    console.error('Failed to load series:', err);
    alert('Failed to load series details');
  }
}
