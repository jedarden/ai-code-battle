// Bot directory page - lists all registered bots
// §16.15: windowed rendering for large directories, "Show more" button,
// keyboard-accessible affordances, IntersectionObserver lazy loading.

import { fetchBotDirectory, type BotDirectoryEntry } from '../api-types';
import { initLazySections, lazySection } from '../lib/lazy-section';

const INITIAL_COUNT = 30;
const BATCH_SIZE = 30;

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

  // For remaining bots, wrap in a lazy section that shows a "Show more" button
  // instead of rendering everything at once when revealed
  container.innerHTML = `
    <p class="updated-at">Last updated: ${formatTimestamp(updatedAt)}</p>
    <div class="bots-grid" id="bots-grid">${initialHtml}</div>
    <div id="bots-remaining-anchor"></div>
  `;

  const grid = document.getElementById('bots-grid')!;
  const anchor = document.getElementById('bots-remaining-anchor')!;

  // Use IntersectionObserver to trigger "Show more" batching when user scrolls near bottom
  let offset = 0;
  const total = remaining.length;

  function updateShowMoreBtn(btn: HTMLButtonElement): void {
    const left = total - offset;
    if (left <= 0) { btn.remove(); return; }
    const next = Math.min(BATCH_SIZE, left);
    btn.textContent = `Show ${next} more bots (${left} remaining)`;
    btn.setAttribute('aria-label', `Show ${next} more bots, ${left} remaining`);
  }

  if (total > BATCH_SIZE) {
    // Lazy-load with IntersectionObserver + "Show more" button
    const observer = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        observer.disconnect();

        // Render first batch of remaining
        appendBatch();
      }
    }, { rootMargin: '300px' });
    observer.observe(anchor);

    // Cleanup on navigation
    window.addEventListener('hashchange', () => observer.disconnect(), { once: true });
  } else {
    // Small remainder — render in a lazy section
    const remainingHtml = remaining.map((bot, i) => renderBotCard(bot, INITIAL_COUNT + i + 1)).join('');
    anchor.innerHTML = lazySection(
      'bots-remaining',
      remainingHtml,
      { placeholder: '<div class="lazy-placeholder" style="min-height:200px"></div>' }
    );
    initLazySections(anchor);
  }

  function appendBatch(): void {
    const batch = remaining.slice(offset, offset + BATCH_SIZE);
    if (batch.length === 0) return;

    const html = batch.map((bot, i) => renderBotCard(bot, INITIAL_COUNT + offset + i + 1)).join('');
    const temp = document.createElement('div');
    temp.innerHTML = html;
    while (temp.firstChild) {
      grid.appendChild(temp.firstChild);
    }

    offset += batch.length;

    if (offset < total) {
      const btn = document.createElement('button');
      btn.className = 'btn secondary show-more-btn';
      btn.type = 'button';
      updateShowMoreBtn(btn);
      btn.addEventListener('click', () => {
        btn.remove();
        appendBatch();
      });
      grid.after(btn);
    } else {
      anchor.remove();
    }
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
