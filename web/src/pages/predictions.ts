// Predictions Page - Prediction leaderboard, open matches, and submission
import type { PredictorStats, OpenMatch, BotProfile } from '../api-types';
import {
  fetchPredictionsLeaderboard,
  fetchOpenPredictions,
  submitPrediction,
  getOrCreatePredictorId,
  fetchPredictionHistory,
} from '../api-types';

const PAGES_BASE = '';

let openMatches: OpenMatch[] = [];
let pollTimer: ReturnType<typeof setInterval> | null = null;
let predictorId = '';

export async function renderPredictionsPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  predictorId = getOrCreatePredictorId();

  app.innerHTML = `
    <div class="predictions-page">
      <h1 class="page-title">Predictions</h1>
      <p class="page-subtitle">Predict match outcomes and climb the leaderboard</p>

      <div class="how-it-works">
        <h2>How It Works</h2>
        <p>Predict the winner of upcoming matches before they start. The more accurate your predictions, the higher you climb the leaderboard.</p>
        <div class="rules-grid">
          <div class="rule-card">
            <span class="rule-icon">1</span>
            <div class="rule-text">
              <h3>Make a Pick</h3>
              <p>Choose which bot you think will win a match</p>
            </div>
          </div>
          <div class="rule-card">
            <span class="rule-icon">2</span>
            <div class="rule-text">
              <h3>Wait for Result</h3>
              <p>After the match completes, predictions are resolved</p>
            </div>
          </div>
          <div class="rule-card">
            <span class="rule-icon">3</span>
            <div class="rule-text">
              <h3>Climb the Ranks</h3>
              <p>Correct predictions increase your streak and ranking</p>
            </div>
          </div>
        </div>
      </div>

      <div class="open-section">
        <h2>Open Matches</h2>
        <div id="open-matches-container">
          <div class="loading">Loading open matches...</div>
        </div>
      </div>

      <div class="history-section">
        <h2>Your Predictions</h2>
        <div id="history-container">
          <div class="loading">Loading your predictions...</div>
        </div>
      </div>

      <div class="leaderboard-section">
        <h2>Top Predictors</h2>
        <div id="leaderboard-container">
          <div class="loading">Loading leaderboard...</div>
        </div>
      </div>

      <div class="stats-section">
        <h2>Your Stats</h2>
        <div id="your-stats" style="display: none;">
          <div class="your-stats-card">
            <div class="stat-row">
              <span class="stat-label">Predictions Made</span>
              <span class="stat-value" id="stat-total">-</span>
            </div>
            <div class="stat-row">
              <span class="stat-label">Accuracy</span>
              <span class="stat-value" id="stat-accuracy">-</span>
            </div>
            <div class="stat-row">
              <span class="stat-label">Current Streak</span>
              <span class="stat-value" id="stat-streak">-</span>
            </div>
            <div class="stat-row">
              <span class="stat-label">Best Streak</span>
              <span class="stat-value" id="stat-best-streak">-</span>
            </div>
          </div>
        </div>
        <div id="stats-login-prompt">
          <p>Make your first prediction above to start tracking stats</p>
        </div>
      </div>
    </div>

    <style>
      .predictions-page {
        max-width: 1000px;
        margin: 0 auto;
      }

      .page-title {
        margin-bottom: 8px;
      }

      .page-subtitle {
        color: var(--text-muted);
        margin-bottom: 32px;
      }

      .how-it-works {
        background-color: var(--bg-secondary);
        border-radius: 12px;
        padding: 24px;
        margin-bottom: 32px;
      }

      .how-it-works h2 {
        margin-bottom: 16px;
      }

      .how-it-works p {
        color: var(--text-muted);
        margin-bottom: 20px;
      }

      .rules-grid {
        display: grid;
        grid-template-columns: repeat(3, 1fr);
        gap: 16px;
      }

      .rule-card {
        display: flex;
        align-items: flex-start;
        gap: 12px;
        background-color: var(--bg-tertiary);
        border-radius: 8px;
        padding: 16px;
      }

      .rule-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 32px;
        height: 32px;
        background-color: var(--accent);
        color: white;
        border-radius: 50%;
        font-weight: 700;
        flex-shrink: 0;
      }

      .rule-text h3 {
        font-size: 0.875rem;
        margin-bottom: 4px;
      }

      .rule-text p {
        font-size: 0.75rem;
        color: var(--text-muted);
        margin: 0;
      }

      .open-section {
        margin-bottom: 32px;
      }

      .open-section h2 {
        margin-bottom: 16px;
      }

      .history-section {
        margin-bottom: 32px;
      }

      .history-section h2 {
        margin-bottom: 16px;
      }

      .history-card {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 14px 20px;
        margin-bottom: 10px;
        display: flex;
        align-items: center;
        gap: 14px;
      }

      .history-card .result-icon {
        width: 28px;
        height: 28px;
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 0.8rem;
        font-weight: 700;
        flex-shrink: 0;
      }

      .history-card .result-icon.correct {
        background-color: rgba(34, 197, 94, 0.2);
        color: #22c55e;
      }

      .history-card .result-icon.incorrect {
        background-color: rgba(239, 68, 68, 0.2);
        color: #ef4444;
      }

      .history-card .result-icon.pending {
        background-color: rgba(107, 114, 128, 0.2);
        color: #94a3b8;
      }

      .history-card .history-details {
        flex: 1;
        min-width: 0;
      }

      .history-card .history-match {
        font-weight: 600;
        color: var(--text-primary);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      .history-card .history-meta {
        font-size: 0.75rem;
        color: var(--text-muted);
        margin-top: 2px;
      }

      .history-card .history-status {
        font-size: 0.75rem;
        font-weight: 600;
        padding: 3px 10px;
        border-radius: 4px;
        flex-shrink: 0;
      }

      .history-card .history-status.correct { background: rgba(34,197,94,0.15); color: #22c55e; }
      .history-card .history-status.incorrect { background: rgba(239,68,68,0.15); color: #ef4444; }
      .history-card .history-status.pending { background: rgba(107,114,128,0.15); color: #94a3b8; }

      .open-match-card {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 16px 20px;
        margin-bottom: 12px;
        display: flex;
        align-items: center;
        gap: 16px;
      }

      .open-match-card .vs {
        color: var(--text-muted);
        font-size: 0.8rem;
        flex-shrink: 0;
      }

      .open-match-card .bot-option {
        display: flex;
        align-items: center;
        gap: 10px;
        flex: 1;
        min-width: 0;
      }

      .open-match-card .bot-option .bot-info {
        min-width: 0;
      }

      .open-match-card .bot-option .bot-name {
        font-weight: 600;
        color: var(--text-primary);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      .open-match-card .bot-option .bot-rating {
        font-size: 0.75rem;
        color: var(--text-muted);
      }

      .open-match-card .pick-btn {
        padding: 6px 14px;
        border-radius: 6px;
        border: 1px solid var(--accent);
        background: transparent;
        color: var(--accent);
        font-size: 0.8rem;
        font-weight: 600;
        cursor: pointer;
        transition: all 0.15s;
        flex-shrink: 0;
      }

      .open-match-card .pick-btn:hover {
        background: var(--accent);
        color: white;
      }

      .open-match-card .pick-btn.picked {
        background: var(--accent);
        color: white;
        border-color: var(--accent);
      }

      .open-match-card .pick-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }

      .leaderboard-section {
        margin-bottom: 32px;
      }

      .leaderboard-section h2 {
        margin-bottom: 16px;
      }

      .predictions-table {
        width: 100%;
        border-collapse: collapse;
        background-color: var(--bg-secondary);
        border-radius: 8px;
        overflow: hidden;
      }

      .predictions-table th,
      .predictions-table td {
        padding: 12px 16px;
        text-align: left;
        border-bottom: 1px solid var(--bg-tertiary);
      }

      .predictions-table th {
        background-color: var(--bg-tertiary);
        color: var(--text-muted);
        font-weight: 600;
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
      }

      .predictions-table tr:hover {
        background-color: var(--bg-tertiary);
      }

      .predictions-table .rank {
        font-weight: 700;
        color: var(--text-muted);
        min-width: 40px;
      }

      .predictions-table tr.rank-1 .rank { color: #fbbf24; }
      .predictions-table tr.rank-2 .rank { color: #94a3b8; }
      .predictions-table tr.rank-3 .rank { color: #cd7f32; }

      .predictor-name {
        color: var(--text-primary);
        font-weight: 500;
      }

      .accuracy-bar {
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .accuracy-fill {
        height: 8px;
        background-color: var(--accent);
        border-radius: 4px;
        transition: width 0.3s;
      }

      .accuracy-text {
        font-size: 0.875rem;
        color: var(--text-secondary);
        min-width: 50px;
      }

      .streak-badge {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        padding: 4px 8px;
        border-radius: 4px;
        font-size: 0.75rem;
        font-weight: 600;
      }

      .streak-badge.positive {
        background-color: rgba(34, 197, 94, 0.2);
        color: #22c55e;
      }

      .streak-badge.negative {
        background-color: rgba(239, 68, 68, 0.2);
        color: #ef4444;
      }

      .streak-badge.neutral {
        background-color: rgba(107, 114, 128, 0.2);
        color: #94a3b8;
      }

      .loading {
        color: var(--text-muted);
        text-align: center;
        padding: 40px;
      }

      .empty-message {
        color: var(--text-muted);
        text-align: center;
        padding: 40px;
      }

      .stats-section h2 {
        margin-bottom: 16px;
      }

      .your-stats-card {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 20px;
      }

      .stat-row {
        display: flex;
        justify-content: space-between;
        padding: 12px 0;
        border-bottom: 1px solid var(--bg-tertiary);
      }

      .stat-row:last-child {
        border-bottom: none;
      }

      .stat-label {
        color: var(--text-muted);
      }

      .stat-value {
        color: var(--text-primary);
        font-weight: 600;
      }

      #stats-login-prompt {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 24px;
        text-align: center;
      }

      #stats-login-prompt p {
        color: var(--text-muted);
        margin-bottom: 16px;
      }

      .updated-at {
        color: var(--text-muted);
        font-size: 0.75rem;
        margin-top: 16px;
      }

      .prediction-error {
        color: #ef4444;
        font-size: 0.8rem;
        margin-top: 8px;
      }

      @media (max-width: 640px) {
        .rules-grid {
          grid-template-columns: 1fr;
        }
        .open-match-card {
          flex-direction: column;
          align-items: stretch;
        }
        .open-match-card .bot-option {
          justify-content: space-between;
        }
      }
    </style>
  `;

  // Load open matches, leaderboard, and history in parallel
  await Promise.all([loadOpenMatches(), loadLeaderboard(), loadHistory()]);

  // Poll for resolved predictions every 15 seconds
  pollTimer = setInterval(async () => {
    await Promise.all([loadOpenMatches(), loadHistory()]);
  }, 15000);
}

// Cleanup polling when navigating away (called by SPA router)
export function cleanupPredictionsPage(): void {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

async function loadOpenMatches(): Promise<void> {
  const container = document.getElementById('open-matches-container');
  if (!container) return;

  try {
    const data = await fetchOpenPredictions(predictorId);
    openMatches = data.matches || [];

    if (openMatches.length === 0) {
      container.innerHTML = '<div class="empty-message">No open matches available for prediction right now. Check back soon!</div>';
      return;
    }

    container.innerHTML = openMatches.map(m => {
      const participants = m.participants || [];
      if (participants.length < 2) return '';

      const botA = participants[0];
      const botB = participants[1];
      const pickedA = m.your_pick === botA.bot_id;
      const pickedB = m.your_pick === botB.bot_id;

      return `
        <div class="open-match-card" data-match-id="${m.match_id}">
          <div class="bot-option">
            <div class="bot-info">
              <div class="bot-name">${escapeHtml(botA.name)}</div>
              <div class="bot-rating">Rating: ${Math.round(botA.rating)}</div>
            </div>
            <button class="pick-btn ${pickedA ? 'picked' : ''}"
                    data-match="${m.match_id}" data-bot="${botA.bot_id}"
                    ${pickedA ? 'disabled' : ''}>
              ${pickedA ? 'Picked' : 'Pick'}
            </button>
          </div>
          <span class="vs">vs</span>
          <div class="bot-option">
            <div class="bot-info">
              <div class="bot-name">${escapeHtml(botB.name)}</div>
              <div class="bot-rating">Rating: ${Math.round(botB.rating)}</div>
            </div>
            <button class="pick-btn ${pickedB ? 'picked' : ''}"
                    data-match="${m.match_id}" data-bot="${botB.bot_id}"
                    ${pickedB ? 'disabled' : ''}>
              ${pickedB ? 'Picked' : 'Pick'}
            </button>
          </div>
        </div>
      `;
    }).join('');

    // Attach click handlers
    container.querySelectorAll('.pick-btn:not(.picked)').forEach(btn => {
      btn.addEventListener('click', handlePick);
    });
  } catch (err) {
    console.error('Failed to load open matches:', err);
    container.innerHTML = '<div class="empty-message">Failed to load open matches</div>';
  }
}

async function handlePick(e: Event): Promise<void> {
  const btn = e.target as HTMLButtonElement;
  const matchId = btn.getAttribute('data-match')!;
  const botId = btn.getAttribute('data-bot')!;
  const card = btn.closest('.open-match-card') as HTMLElement;

  // Disable all buttons in this card
  card.querySelectorAll('.pick-btn').forEach(b => {
    (b as HTMLButtonElement).disabled = true;
  });
  btn.textContent = 'Submitting...';

  try {
    await submitPrediction(matchId, botId, predictorId);

    // Mark the picked button
    btn.textContent = 'Picked';
    btn.classList.add('picked');

    // Update the other button to show it wasn't picked
    card.querySelectorAll('.pick-btn:not(.picked)').forEach(b => {
      (b as HTMLButtonElement).textContent = 'Not picked';
    });

    // Refresh history to show the new prediction
    loadHistory();
  } catch (err) {
    console.error('Failed to submit prediction:', err);
    btn.textContent = 'Error';
    card.querySelectorAll('.pick-btn').forEach(b => {
      (b as HTMLButtonElement).disabled = false;
    });

    // Show error message
    const errDiv = card.querySelector('.prediction-error');
    if (errDiv) errDiv.textContent = (err as Error).message;
  }
}

async function loadHistory(): Promise<void> {
  const container = document.getElementById('history-container');
  if (!container) return;

  try {
    const data = await fetchPredictionHistory(predictorId, 20);
    const predictions = data.predictions || [];

    if (predictions.length === 0) {
      container.innerHTML = '<div class="empty-message">You haven\'t made any predictions yet. Pick a bot above!</div>';
      return;
    }

    container.innerHTML = predictions.map(p => {
      let icon: string, iconClass: string, statusText: string, statusClass: string;

      if (p.correct === true) {
        icon = '✓';
        iconClass = 'correct';
        statusText = 'Correct!';
        statusClass = 'correct';
      } else if (p.correct === false) {
        icon = '✗';
        iconClass = 'incorrect';
        statusText = p.winner_name ? `Wrong — ${p.winner_name} won` : 'Wrong';
        statusClass = 'incorrect';
      } else {
        icon = '…';
        iconClass = 'pending';
        statusText = 'Pending';
        statusClass = 'pending';
      }

      return `
        <div class="history-card">
          <div class="result-icon ${iconClass}">${icon}</div>
          <div class="history-details">
            <div class="history-match">Picked ${escapeHtml(p.predicted_name || p.predicted_bot)}</div>
            <div class="history-meta">${formatTimeAgo(p.created_at)}</div>
          </div>
          <span class="history-status ${statusClass}">${statusText}</span>
        </div>
      `;
    }).join('');
  } catch (err) {
    console.error('Failed to load prediction history:', err);
    container.innerHTML = '<div class="empty-message">Failed to load prediction history</div>';
  }
}

function formatTimeAgo(isoString: string): string {
  const date = new Date(isoString);
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

async function loadLeaderboard(): Promise<void> {
  const container = document.getElementById('leaderboard-container');
  if (!container) return;

  try {
    const data = await fetchPredictionsLeaderboard();

    if (data.entries.length === 0) {
      container.innerHTML = '<div class="empty-message">No predictions have been made yet</div>';
      return;
    }

    // Check if current predictor is in the list
    const myEntry = data.entries.find((e: PredictorStats) => e.predictor_id === predictorId);
    if (myEntry) {
      const statsEl = document.getElementById('your-stats');
      const promptEl = document.getElementById('stats-login-prompt');
      if (statsEl && promptEl) {
        statsEl.style.display = 'block';
        promptEl.style.display = 'none';
        const total = myEntry.correct + myEntry.incorrect;
        const accuracy = total > 0 ? Math.round((myEntry.correct / total) * 100) : 0;
        document.getElementById('stat-total')!.textContent = String(total);
        document.getElementById('stat-accuracy')!.textContent = `${accuracy}%`;
        document.getElementById('stat-streak')!.textContent = String(myEntry.streak);
        document.getElementById('stat-best-streak')!.textContent = String(myEntry.best_streak);
      }
    }

    // Fetch bot names for predictor IDs
    const botNames = await fetchBotNames(data.entries.map((e: PredictorStats) => e.predictor_id));

    container.innerHTML = `
      <table class="predictions-table">
        <thead>
          <tr>
            <th>Rank</th>
            <th>Predictor</th>
            <th>Correct</th>
            <th>Incorrect</th>
            <th>Accuracy</th>
            <th>Streak</th>
          </tr>
        </thead>
        <tbody>
          ${data.entries.map((entry: PredictorStats, idx: number) => {
            const total = entry.correct + entry.incorrect;
            const accuracy = total > 0 ? Math.round((entry.correct / total) * 100) : 0;
            const streakClass = entry.streak > 0 ? 'positive' : entry.streak < 0 ? 'negative' : 'neutral';
            const botName = botNames.get(entry.predictor_id) || entry.predictor_id;
            const isYou = entry.predictor_id === predictorId;

            return `
              <tr class="rank-${idx + 1}">
                <td class="rank">#${idx + 1}</td>
                <td class="predictor-name">${botName}${isYou ? ' (you)' : ''}</td>
                <td>${entry.correct}</td>
                <td>${entry.incorrect}</td>
                <td>
                  <div class="accuracy-bar">
                    <div class="accuracy-fill" style="width: ${accuracy}%"></div>
                    <span class="accuracy-text">${accuracy}%</span>
                  </div>
                </td>
                <td>
                  <span class="streak-badge ${streakClass}">
                    ${entry.streak > 0 ? '+' : ''}${entry.streak}
                  </span>
                </td>
              </tr>
            `;
          }).join('')}
        </tbody>
      </table>
      <p class="updated-at">Updated: ${new Date(data.updated_at).toLocaleString()}</p>
    `;

  } catch (err) {
    console.error('Failed to load predictions leaderboard:', err);
    container.innerHTML = '<div class="empty-message">Failed to load leaderboard</div>';
  }
}

function escapeHtml(str: string): string {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

async function fetchBotNames(botIds: string[]): Promise<Map<string, string>> {
  const names = new Map<string, string>();
  const uniqueIds = [...new Set(botIds)];

  await Promise.all(uniqueIds.map(async id => {
    try {
      const response = await fetch(`${PAGES_BASE}/data/bots/${id}.json`);
      if (response.ok) {
        const bot: BotProfile = await response.json();
        names.set(id, bot.name);
      }
    } catch {
      // Ignore errors, will use ID as fallback
    }
  }));

  return names;
}
