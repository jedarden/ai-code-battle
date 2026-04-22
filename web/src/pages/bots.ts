// Bot directory page - lists all registered bots
// §16.15: windowed rendering for large directories, "Show more" button,
// keyboard-accessible affordances, IntersectionObserver lazy loading.

import { fetchBotDirectory, type BotDirectoryEntry } from '../api-types';
import { initLazySections, lazySection } from '../lib/lazy-section';

const INITIAL_COUNT = 30;

export async function renderBotsPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="bots-page">
      <h1>Bot Directory</h1>
      <div id="bots-content" class="loading">Loading...</div>
    </div>
  `;

  const content = document.getElementById('bots-content');
  if (!content) return;

  try {
    const data = await fetchBotDirectory();
    renderBotsList(content, data.bots, data.updated_at);
  } catch (error) {
    content.innerHTML = `
      <div class="error">
        <p>Failed to load bot directory: ${error}</p>
        <p class="hint">Bot data may not be available yet.</p>
      </div>
    `;
  }
}

function renderBotsList(
  container: HTMLElement,
  bots: BotDirectoryEntry[],
  updatedAt: string
): void {
  if (bots.length === 0) {
    container.innerHTML = `
      <div class="empty-state">
        <p>No bots registered yet.</p>
        <a href="#/compete/register" class="btn primary">Register a Bot</a>
      </div>
    `;
    return;
  }

  // Sort by rating descending
  const sortedBots = [...bots].sort((a, b) => b.rating - a.rating);

  const initial = sortedBots.slice(0, INITIAL_COUNT);
  const remaining = sortedBots.slice(INITIAL_COUNT);

  const initialHtml = initial.map((bot, idx) => renderBotCard(bot, idx + 1)).join('');

  if (remaining.length === 0) {
    container.innerHTML = `
      <p class="updated-at">Last updated: ${formatTimestamp(updatedAt)}</p>
      <div class="bots-grid">${initialHtml}</div>
    `;
    return;
  }

  // Use lazySection for below-the-fold bot cards
  const remainingHtml = remaining.map((bot, i) => renderBotCard(bot, INITIAL_COUNT + i + 1)).join('');
  container.innerHTML = `
    <p class="updated-at">Last updated: ${formatTimestamp(updatedAt)}</p>
    <div class="bots-grid">
      ${initialHtml}
      ${lazySection(
        'bots-remaining',
        remainingHtml,
        { placeholder: '<div class="lazy-placeholder" style="min-height:200px"></div>' }
      )}
    </div>
  `;

  initLazySections(container);
}

function renderBotCard(bot: BotDirectoryEntry, rank: number): string {
  return `
    <a href="#/bot/${encodeURIComponent(bot.id)}" class="bot-card">
      <div class="bot-rank">#${rank}</div>
      <div class="bot-info">
        <h3 class="bot-name">${escapeHtml(bot.name)}</h3>
        <div class="bot-stats">
          <span class="bot-rating">${bot.rating} rating</span>
          <span class="bot-matches">${bot.matches_played} matches</span>
          <span class="bot-winrate">${bot.win_rate.toFixed(1)}% win</span>
        </div>
      </div>
    </a>
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
