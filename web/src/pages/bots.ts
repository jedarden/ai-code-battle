// Bot directory page - lists all registered bots
// §16.15: windowed rendering for large directories, "Show more" button,
// keyboard-accessible affordances.

import { fetchBotDirectory, type BotDirectoryEntry } from '../api-types';

const INITIAL_COUNT = 30;
const BATCH_SIZE = 50;

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

  container.innerHTML = `
    <p class="updated-at">Last updated: ${formatTimestamp(updatedAt)}</p>
    <div class="bots-grid">
      ${initial.map((bot, idx) => renderBotCard(bot, idx + 1)).join('')}
    </div>
    ${remaining.length > 0 ? `<div id="bots-remaining"></div>` : ''}
  `;

  // Lazy-load remaining bots when scrolled near the sentinel
  if (remaining.length > 0) {
    const sentinel = document.getElementById('bots-remaining');
    if (sentinel) {
      let offset = 0;
      const total = remaining.length;

      const observer = new IntersectionObserver((entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          observer.disconnect();
          appendBotBatch(sentinel, remaining, offset, total);
        }
      }, { rootMargin: '300px' });
      observer.observe(sentinel);
    }
  }
}

function appendBotBatch(
  sentinel: HTMLElement,
  remaining: BotDirectoryEntry[],
  startOffset: number,
  totalCount: number
): void {
  const batch = remaining.slice(startOffset, startOffset + BATCH_SIZE);
  if (batch.length === 0) return;

  const grid = sentinel.previousElementSibling as HTMLElement | null;
  if (!grid) return;

  // Adjust rank to continue from where initial batch left off
  const rankOffset = INITIAL_COUNT + startOffset;
  const html = batch.map((bot, i) => renderBotCard(bot, rankOffset + i + 1)).join('');
  const temp = document.createElement('div');
  temp.innerHTML = html;
  while (temp.firstChild) {
    grid.appendChild(temp.firstChild);
  }

  const newOffset = startOffset + batch.length;
  if (newOffset < totalCount) {
    // Add "Show more" button
    const left = totalCount - newOffset;
    const next = Math.min(BATCH_SIZE, left);
    const btn = document.createElement('button');
    btn.className = 'btn secondary show-more-btn';
    btn.type = 'button';
    btn.textContent = `Show ${next} more bots (${left} remaining)`;
    btn.setAttribute('aria-label', `Show ${next} more bots, ${left} remaining`);

    btn.addEventListener('click', () => {
      btn.remove();
      appendBotBatch(sentinel, remaining, newOffset, totalCount);
    });

    sentinel.before(btn);
  } else {
    sentinel.remove();
  }
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
