// Predictions Page - Prediction leaderboard and stats
import type { BotProfile, PredictorStats } from '../api-types';
import { fetchPredictionsLeaderboard } from '../api-types';

const PAGES_BASE = '';

export async function renderPredictionsPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="predictions-page">
      <h1 class="page-title">Prediction Leaderboard</h1>
      <p class="page-subtitle">Top predictors and their accuracy stats</p>

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
          <p>Log in to track your predictions</p>
          <button class="btn primary" id="login-btn">Connect</button>
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
    </style>
  `;

  // Load leaderboard
  await loadLeaderboard();
}

// fetch bot names for leaderboard display
async function loadLeaderboard(): Promise<void> {
  const container = document.getElementById('leaderboard-container');
  if (!container) return;

  try {
    const data = await fetchPredictionsLeaderboard();

    if (data.entries.length === 0) {
      container.innerHTML = '<div class="empty-message">No predictions have been made yet</div>';
      return;
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

            return `
              <tr class="rank-${idx + 1}">
                <td class="rank">#${idx + 1}</td>
                <td class="predictor-name">${botName}</td>
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
