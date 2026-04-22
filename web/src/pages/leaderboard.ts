// Leaderboard page - displays bot rankings with progressive disclosure per §16.15.
// Uses virtual scrolling for 1000+ entries, expandable rows for secondary detail,
// and IntersectionObserver for below-the-fold content.

import { fetchLeaderboard, type LeaderboardEntry } from '../api-types';
import { VirtualList } from '../lib/virtual-list';
import { initLazySections, lazySection } from '../lib/lazy-section';

const ROW_HEIGHT = 48;

export async function renderLeaderboardPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="leaderboard-page">
      <h1>Leaderboard</h1>
      <div id="leaderboard-content" class="loading">Loading...</div>
    </div>
  `;

  const content = document.getElementById('leaderboard-content');
  if (!content) return;

  try {
    const data = await fetchLeaderboard();
    renderLeaderboard(content, data.entries, data.updated_at);
  } catch (error) {
    content.innerHTML = `
      <div class="error">
        <p>Failed to load leaderboard: ${error}</p>
        <p class="hint">The leaderboard data may not be available yet. Check back after some matches have been played.</p>
      </div>
    `;
  }
}

function renderLeaderboard(
  container: HTMLElement,
  entries: LeaderboardEntry[],
  updatedAt: string
): void {
  if (entries.length === 0) {
    container.innerHTML = `
      <div class="empty-state">
        <p>No bots on the leaderboard yet.</p>
        <p>Bots appear here after completing their first match.</p>
        <a href="#/compete/register" class="btn primary">Register a Bot</a>
      </div>
    `;
    return;
  }

  const useVirtualList = entries.length > 50;

  container.innerHTML = `
    <p class="updated-at">Last updated: ${formatTimestamp(updatedAt)}</p>
    <p class="lb-hint">${useVirtualList ? 'Click a row to see full stats' : ''}</p>
    <div id="lb-desktop"></div>
    <div id="lb-mobile" class="mobile-cards" role="list"></div>
  `;

  // Desktop: virtual list or static table depending on size
  renderDesktopList(document.getElementById('lb-desktop')!, entries, useVirtualList);

  // Mobile: lazy-rendered expandable cards for large lists
  if (useVirtualList) {
    // Wrap mobile cards in a lazy section so they don't render until scrolled into view
    const mobileEl = document.getElementById('lb-mobile')!;
    mobileEl.innerHTML = lazySection(
      'lb-mobile-cards',
      entries.slice(0, 20).map(entry => renderMobileCard(entry)).join(''),
      { placeholder: '<div class="lazy-placeholder" style="min-height:400px"></div>' }
    );
    initMobileCardToggles(mobileEl);
    if (entries.length > 20) {
      addMobileShowMore(mobileEl, entries, 20);
    }
  } else {
    renderMobileCards(document.getElementById('lb-mobile')!, entries);
  }

  // Activate lazy sections
  initLazySections(container);
}

// ─── Desktop rendering ──────────────────────────────────────────────────────────

function renderDesktopList(el: HTMLElement, entries: LeaderboardEntry[], useVirtual: boolean): void {
  if (useVirtual) {
    const vl = new VirtualList<LeaderboardEntry>({
      items: entries,
      rowHeight: ROW_HEIGHT,
      initialCount: 100,
      renderRow: renderDesktopRow,
      renderExpanded: renderDesktopExpanded,
      containerClass: 'leaderboard-virtual',
      ariaLabel: 'Bot leaderboard',
    });
    vl.mount(el);
    // Store reference for cleanup (page navigation replaces innerHTML)
    (el as any)._virtualList = vl;
  } else {
    renderStaticTable(el, entries);
  }
}

function renderStaticTable(container: HTMLElement, entries: LeaderboardEntry[]): void {
  container.innerHTML = `
    <div class="table-container">
      <table class="leaderboard-table">
        <thead>
          <tr>
            <th>Rank</th>
            <th>Bot</th>
            <th>Rating</th>
            <th>W/L</th>
            <th>Win Rate</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          ${entries.map(entry => renderDesktopRow(entry, 0)).join('')}
        </tbody>
      </table>
    </div>
  `;

  // Wire expand on click for small tables too
  initDesktopExpandToggle(container);
}

function renderDesktopRow(entry: LeaderboardEntry, _index: number): string {
  const rankClass = entry.rank <= 3 ? `rank-${entry.rank}` : '';
  const statusClass = entry.health_status === 'healthy' ? 'status-healthy' :
                      entry.health_status === 'unhealthy' ? 'status-unhealthy' : 'status-unknown';
  return `
    <div class="lb-row ${rankClass}" data-bot-id="${encodeURIComponent(entry.bot_id)}">
      <span class="lb-rank">${entry.rank}</span>
      <span class="lb-name">
        <a href="#/bot/${encodeURIComponent(entry.bot_id)}">${escapeHtml(entry.name)}</a>
      </span>
      <span class="lb-rating">
        <span class="rating-value">${entry.rating}</span>
        <span class="rating-dev">±${entry.rating_deviation}</span>
      </span>
      <span class="lb-wl">${entry.matches_won}/${entry.matches_played}</span>
      <span class="lb-winrate">${entry.win_rate.toFixed(1)}%</span>
      <span class="lb-status ${statusClass}">${entry.health_status}</span>
      <span class="lb-expand-icon" aria-hidden="true">▸</span>
    </div>
  `;
}

function renderDesktopExpanded(entry: LeaderboardEntry, _index: number): string {
  const losses = entry.matches_played - entry.matches_won;
  return `
    <div class="lb-expanded">
      <div class="lb-expanded-stats">
        <div class="lb-stat"><span class="lb-stat-val">${entry.matches_played}</span><span class="lb-stat-label">Matches</span></div>
        <div class="lb-stat"><span class="lb-stat-val">${entry.matches_won}</span><span class="lb-stat-label">Wins</span></div>
        <div class="lb-stat"><span class="lb-stat-val">${losses}</span><span class="lb-stat-label">Losses</span></div>
        <div class="lb-stat"><span class="lb-stat-val">${entry.win_rate.toFixed(1)}%</span><span class="lb-stat-label">Win Rate</span></div>
        <div class="lb-stat"><span class="lb-stat-val">±${entry.rating_deviation}</span><span class="lb-stat-label">Deviation</span></div>
      </div>
      <a href="#/bot/${encodeURIComponent(entry.bot_id)}" class="btn small lb-profile-link">Full Profile →</a>
    </div>
  `;
}

function initDesktopExpandToggle(container: HTMLElement): void {
  container.addEventListener('click', (e) => {
    const row = (e.target as HTMLElement).closest('.lb-row') as HTMLElement | null;
    if (!row) return;
    if ((e.target as HTMLElement).closest('a, button')) return;
    const expanded = row.classList.toggle('row-expanded');
    row.setAttribute('aria-expanded', String(expanded));
    const icon = row.querySelector('.lb-expand-icon');
    if (icon) icon.textContent = expanded ? '▾' : '▸';
  });
}

// ─── Mobile rendering ───────────────────────────────────────────────────────────

function renderMobileCards(container: HTMLElement, entries: LeaderboardEntry[]): void {
  const showAll = entries.length <= 20;
  const visibleCount = showAll ? entries.length : 20;

  container.innerHTML = entries.slice(0, visibleCount).map(entry => renderMobileCard(entry)).join('');

  initMobileCardToggles(container);

  if (!showAll) {
    addMobileShowMore(container, entries, visibleCount);
  }
}

function renderMobileCard(entry: LeaderboardEntry): string {
  const rankClass = entry.rank <= 3 ? `rank-${entry.rank}` : '';
  const statusClass = entry.health_status === 'healthy' ? 'status-healthy' :
                      entry.health_status === 'unhealthy' ? 'status-unhealthy' : 'status-unknown';
  const winRate = entry.win_rate.toFixed(1);
  const losses = entry.matches_played - entry.matches_won;

  return `
    <div class="leaderboard-mobile-card" role="listitem" data-bot-id="${encodeURIComponent(entry.bot_id)}" aria-expanded="false">
      <button class="mobile-card-toggle" aria-label="Expand details for ${escapeHtml(entry.name)}" type="button">
        <div class="leaderboard-mobile-rank ${rankClass}">${entry.rank}</div>
        <div class="leaderboard-mobile-info">
          <div class="leaderboard-mobile-name">${escapeHtml(entry.name)}</div>
          <div class="leaderboard-mobile-rating">${entry.rating} <span style="opacity:.6;font-size:.8em">±${entry.rating_deviation}</span></div>
        </div>
        <span class="mobile-card-arrow" aria-hidden="true">▸</span>
      </button>
      <div class="leaderboard-mobile-details">
        <div class="leaderboard-mobile-stat">
          <span class="leaderboard-mobile-stat-label">W / L</span>
          <span class="leaderboard-mobile-stat-value">${entry.matches_won} / ${losses}</span>
        </div>
        <div class="leaderboard-mobile-stat">
          <span class="leaderboard-mobile-stat-label">Win Rate</span>
          <span class="leaderboard-mobile-stat-value">${winRate}%</span>
        </div>
        <div class="leaderboard-mobile-stat">
          <span class="leaderboard-mobile-stat-label">Matches</span>
          <span class="leaderboard-mobile-stat-value">${entry.matches_played}</span>
        </div>
        <div class="leaderboard-mobile-stat">
          <span class="leaderboard-mobile-stat-label">Status</span>
          <span class="leaderboard-mobile-stat-value ${statusClass}">${entry.health_status}</span>
        </div>
        <a href="#/bot/${encodeURIComponent(entry.bot_id)}"
           class="btn small"
           style="margin-top:10px;display:block;text-align:center"
           aria-label="Full stats for ${escapeHtml(entry.name)}">Full Stats →</a>
      </div>
    </div>
  `;
}

function initMobileCardToggles(container: HTMLElement): void {
  container.querySelectorAll<HTMLElement>('.leaderboard-mobile-card').forEach(card => {
    const toggle = card.querySelector<HTMLButtonElement>('.mobile-card-toggle');
    if (!toggle) return;
    toggle.addEventListener('click', (e) => {
      if ((e.target as HTMLElement).closest('a')) return;
      const details = card.querySelector<HTMLElement>('.leaderboard-mobile-details');
      if (!details) return;
      const expanded = details.classList.toggle('expanded');
      card.setAttribute('aria-expanded', String(expanded));
      const arrow = card.querySelector('.mobile-card-arrow');
      if (arrow) arrow.textContent = expanded ? '▾' : '▸';
      toggle.setAttribute('aria-expanded', String(expanded));
    });
  });
}

function addMobileShowMore(
  container: HTMLElement,
  allEntries: LeaderboardEntry[],
  currentVisible: number
): void {
  const btn = document.createElement('button');
  btn.className = 'btn secondary show-more-btn';
  btn.type = 'button';
  updateShowMoreButton(btn, allEntries.length, currentVisible);

  btn.addEventListener('click', () => {
    const cards = container.querySelectorAll('.leaderboard-mobile-card');
    const lastIdx = cards.length;
    const nextBatch = 50;
    const end = Math.min(lastIdx + nextBatch, allEntries.length);

    const temp = document.createElement('div');
    temp.innerHTML = allEntries.slice(lastIdx, end).map(e => renderMobileCard(e)).join('');

    while (temp.firstChild) {
      container.appendChild(temp.firstChild);
    }

    initMobileCardToggles(container);

    const newCount = end;
    if (newCount >= allEntries.length) {
      btn.remove();
    } else {
      updateShowMoreButton(btn, allEntries.length, newCount);
    }
  });

  container.after(btn);
}

function updateShowMoreButton(btn: HTMLButtonElement, total: number, visible: number): void {
  const remaining = total - visible;
  const next = Math.min(50, remaining);
  btn.textContent = `Show ${next} more (${remaining} remaining)`;
  btn.setAttribute('aria-label', `Show ${next} more bots, ${remaining} remaining`);
}

// ─── Utilities ──────────────────────────────────────────────────────────────────

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
