// Individual rivalry page: /rivalry/{bot_a}/{bot_b}
// Shows detailed head-to-head history between two bots (§13.5)

import { fetchRivalries, type RivalryEntry } from '../api-types';

// ─── Page render ─────────────────────────────────────────────────────────────

export async function renderRivalryPage(params: Record<string, string>): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  const botA = params.bot_a;
  const botB = params.bot_b;

  app.innerHTML = `
    <div class="rivalry-page">
      <h1 class="page-title">Rivalry</h1>
      <div id="rivalry-content" class="loading">Loading rivalry…</div>
    </div>
    ${RIVALRY_STYLES}
  `;

  const content = document.getElementById('rivalry-content')!;

  try {
    const { rivalries } = await fetchRivalries();

    // Find the rivalry entry for this pair (order-agnostic)
    const rivalry = rivalries.find(r =>
      (r.bot_a.id === botA && r.bot_b.id === botB) ||
      (r.bot_a.id === botB && r.bot_b.id === botA)
    );

    if (!rivalry) {
      content.innerHTML = `
        <div class="empty-state">
          <p>No rivalry found between these bots.</p>
          <p class="hint">Rivalries appear when two bots have played at least 10 head-to-head matches.
             <a href="#/rivalries">View all rivalries</a></p>
        </div>
      `;
      return;
    }

    // Normalize so botA is always first
    const isReversed = rivalry.bot_a.id !== botA;
    const entry = isReversed ? swapRivalry(rivalry) : rivalry;

    renderRivalryDetail(content, entry);
  } catch (err) {
    content.innerHTML = `<div class="error">Failed to load rivalry: ${err}</div>`;
  }
}

// ─── Detail renderer ──────────────────────────────────────────────────────────

function renderRivalryDetail(container: HTMLElement, r: RivalryEntry): void {
  const total = r.matches;
  const pctA = total > 0 ? (r.record.a_wins / total) * 100 : 50;
  const pctD = total > 0 ? (r.record.draws / total) * 100 : 0;
  const pctB = 100 - pctA - pctD;

  container.innerHTML = `
    <div class="rivalry-detail">
      <div class="rivalry-header-large">
        <div class="combatant-large">
          <a href="#/bot/${r.bot_a.id}" class="combatant-name-large">${escapeHtml(r.bot_a.name)}</a>
          <span class="combatant-record-large">${r.record.a_wins}W</span>
        </div>
        <div class="vs-large">VS</div>
        <div class="combatant-large right">
          <a href="#/bot/${r.bot_b.id}" class="combatant-name-large">${escapeHtml(r.bot_b.name)}</a>
          <span class="combatant-record-large">${r.record.b_wins}W</span>
        </div>
      </div>

      <div class="rivalry-stats">
        <div class="stat-box">
          <div class="stat-value">${total}</div>
          <div class="stat-label">Matches</div>
        </div>
        <div class="stat-box">
          <div class="stat-value">${pctA.toFixed(0)}%</div>
          <div class="stat-label">${r.bot_a.name} Win Rate</div>
        </div>
        ${r.record.draws > 0 ? `
        <div class="stat-box">
          <div class="stat-value">${r.record.draws}</div>
          <div class="stat-label">Draws</div>
        </div>` : ''}
      </div>

      <div class="win-bar-large">
        <div class="win-bar-seg seg0" style="width:${pctA.toFixed(1)}%"></div>
        <div class="win-bar-seg seg-draw" style="width:${pctD.toFixed(1)}%"></div>
        <div class="win-bar-seg seg1" style="width:${pctB.toFixed(1)}%"></div>
      </div>

      <p class="rivalry-narrative-large">${escapeHtml(r.narrative)}</p>

      ${r.longest_streak ? `
      <div class="streak-highlight">
        <span class="streak-icon">🔥</span>
        <span><strong>${escapeHtml(r.longest_streak.holder)}</strong>'s ${r.longest_streak.length}-win streak</span>
      </div>` : ''}

      <div class="rivalry-matches">
        <h3>Recent Matches</h3>
        ${r.recent_matches.length > 0 ? `
        <div class="match-list">
          ${r.recent_matches.map(matchId => `
            <a href="#/watch/replay/${matchId}" class="match-link">Match ${matchId}</a>
          `).join('')}
        </div>` : '<p class="hint">No recent matches available.</p>'}
      </div>

      ${r.closest_match ? `
      <div class="rivalry-footer">
        <a href="#/watch/replay/${r.closest_match}" class="btn primary">Watch Closest Match</a>
      </div>` : ''}

      <div class="rivalry-footer">
        <a href="#/rivalries" class="btn secondary">← All Rivalries</a>
      </div>
    </div>
  `;
}

// Swap bot_a/b and their stats when rivalry is queried in reverse order
function swapRivalry(r: RivalryEntry): RivalryEntry {
  return {
    ...r,
    bot_a: r.bot_b,
    bot_b: r.bot_a,
    record: {
      a_wins: r.record.b_wins,
      b_wins: r.record.a_wins,
      draws: r.record.draws,
    },
  };
}

function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// ─── Styles ───────────────────────────────────────────────────────────────────

const RIVALRY_STYLES = `
<style>
.rivalry-page {
  max-width: 900px;
  margin: 0 auto;
  padding: 20px;
}
.rivalry-detail {
  background: var(--bg-secondary, #1e293b);
  border-radius: 8px;
  padding: 24px;
}
.rivalry-header-large {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
}
.combatant-large {
  flex: 1;
  text-align: center;
}
.combatant-large.right {
  text-align: center;
}
.combatant-name-large {
  display: block;
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-primary, #e5e7eb);
  text-decoration: none;
}
.combatant-name-large:hover {
  color: var(--accent, #3b82f6);
}
.combatant-record-large {
  display: block;
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
  margin-top: 8px;
}
.vs-large {
  font-size: 1.5rem;
  font-weight: 900;
  color: var(--text-muted, #94a3b8);
}
.rivalry-stats {
  display: flex;
  gap: 16px;
  justify-content: center;
  margin-bottom: 24px;
}
.stat-box {
  text-align: center;
  padding: 16px;
  background: var(--bg-tertiary, #0f172a);
  border-radius: 8px;
  min-width: 100px;
}
.stat-value {
  font-size: 2rem;
  font-weight: 700;
  color: var(--text-primary, #e5e7eb);
}
.stat-label {
  font-size: 0.875rem;
  color: var(--text-muted, #94a3b8);
  margin-top: 4px;
}
.win-bar-large {
  height: 24px;
  border-radius: 4px;
  overflow: hidden;
  display: flex;
  margin-bottom: 24px;
}
.win-bar-seg {
  height: 100%;
}
.win-bar-seg.seg0 { background: #3b82f6; }
.win-bar-seg.seg-draw { background: #64748b; }
.win-bar-seg.seg1 { background: #ef4444; }
.rivalry-narrative-large {
  font-size: 1.125rem;
  line-height: 1.6;
  color: var(--text-primary, #e5e7eb);
  text-align: center;
  margin-bottom: 24px;
}
.streak-highlight {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px;
  background: var(--bg-tertiary, #0f172a);
  border-radius: 8px;
  margin-bottom: 24px;
  font-size: 1rem;
  color: var(--text-primary, #e5e7eb);
}
.streak-icon {
  font-size: 1.25rem;
}
.rivalry-matches {
  margin-bottom: 24px;
}
.rivalry-matches h3 {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--text-primary, #e5e7eb);
  margin-bottom: 12px;
}
.match-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.match-link {
  padding: 8px 12px;
  background: var(--bg-tertiary, #0f172a);
  border-radius: 4px;
  color: var(--text-primary, #e5e7eb);
  text-decoration: none;
  font-size: 0.875rem;
  transition: background 0.2s;
}
.match-link:hover {
  background: var(--bg-hover, #334155);
}
.rivalry-footer {
  display: flex;
  justify-content: center;
  gap: 12px;
}
.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: var(--text-muted, #94a3b8);
}
.empty-state p {
  margin-bottom: 8px;
}
.empty-state .hint {
  font-size: 0.875rem;
}
.empty-state a {
  color: var(--accent, #3b82f6);
  text-decoration: none;
}
.empty-state a:hover {
  text-decoration: underline;
}
.error {
  padding: 20px;
  background: var(--bg-error, #450a0a);
  border-radius: 8px;
  color: var(--text-error, #fca5a5);
}
.loading {
  text-align: center;
  padding: 60px 20px;
  color: var(--text-muted, #94a3b8);
}
</style>
`;
