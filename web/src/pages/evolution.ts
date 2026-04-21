// Evolution dashboard - shows live evolution pipeline status

import { fetchEvolutionData, type EvolutionLiveData, type IslandStat, type LineageNode, type MetaSnapshot, type GenerationEntry, type CycleInfo, type ActivityEntry, type Totals, type Candidate } from '../api-types';

const ISLAND_COLORS: Record<string, string> = {
  alpha: '#ef4444', // red   - core-rushing
  beta:  '#f59e0b', // amber - energy-focused
  gamma: '#22c55e', // green - defensive
  delta: '#a78bfa', // violet - experimental
};

const ISLAND_LABELS: Record<string, string> = {
  alpha: 'Alpha (Rush)',
  beta:  'Beta (Economy)',
  gamma: 'Gamma (Defense)',
  delta: 'Delta (Experimental)',
};

let pollingInterval: number | null = null;

export async function renderEvolutionPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="evolution-page">
      <h1 class="page-title">Evolution Dashboard</h1>
      <div id="evolution-content" class="loading">Loading evolution data...</div>
    </div>
  `;

  const content = document.getElementById('evolution-content');
  if (!content) return;

  // Clear any existing poll
  if (pollingInterval !== null) {
    clearInterval(pollingInterval);
  }

  // Initial load
  await loadEvolutionData(content);

  // Start polling for live updates (every 10 seconds)
  pollingInterval = window.setInterval(() => {
    loadEvolutionData(content);
  }, 10000);
}

async function loadEvolutionData(content: HTMLElement): Promise<void> {
  try {
    const data = await fetchEvolutionData();
    renderDashboard(content, data);
  } catch {
    content.innerHTML = `
      <div class="error">
        <p>Evolution data not available yet.</p>
        <p class="hint">The evolution pipeline needs to run at least one cycle before data appears here.
           Run <code>acb-evolver live-export</code> to generate the data file.</p>
      </div>
    `;
  }
}

// Stop polling when navigating away
export function cleanupEvolutionPage(): void {
  if (pollingInterval !== null) {
    clearInterval(pollingInterval);
    pollingInterval = null;
  }
}

function renderDashboard(container: HTMLElement, data: EvolutionLiveData): void {
  container.innerHTML = `
    <p class="updated-at">Last updated: ${formatTimestamp(data.updated_at)} &nbsp;·&nbsp;
       ${data.total_programs || 0} programs &nbsp;·&nbsp; ${data.promoted_count || 0} promoted</p>

    <section class="evo-section">
      <h2 class="evo-section-title">Live Status</h2>
      <div id="live-status"></div>
    </section>

    <section class="evo-section">
      <h2 class="evo-section-title">Island Overview</h2>
      <div class="island-grid" id="island-grid"></div>
    </section>

    <section class="evo-section">
      <h2 class="evo-section-title">Statistics</h2>
      <div id="statistics"></div>
    </section>

    <section class="evo-section">
      <h2 class="evo-section-title">Recent Activity</h2>
      <div id="activity-feed"></div>
    </section>

    <section class="evo-section">
      <h2 class="evo-section-title">Meta Tracker <span class="evo-subtitle">Best fitness per island over generations</span></h2>
      <div class="chart-container" id="meta-chart"></div>
    </section>

    <section class="evo-section">
      <h2 class="evo-section-title">Lineage Tree <span class="evo-subtitle">Program ancestry (top 80 by fitness)</span></h2>
      <div class="lineage-container" id="lineage-tree"></div>
    </section>

    <section class="evo-section">
      <h2 class="evo-section-title">Generation Log</h2>
      <div id="generation-log"></div>
    </section>

    <style>
      .evo-section {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 20px;
        margin-bottom: 24px;
      }

      .evo-section-title {
        font-size: 1rem;
        font-weight: 600;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
        margin-bottom: 16px;
      }

      .evo-subtitle {
        font-size: 0.75rem;
        font-weight: 400;
        color: var(--text-muted);
        text-transform: none;
        letter-spacing: 0;
        margin-left: 8px;
      }

      /* Live status */
      .live-status-container {
        display: flex;
        flex-direction: column;
        gap: 16px;
      }

      .live-status-main {
        display: flex;
        flex-wrap: wrap;
        gap: 24px;
        align-items: center;
      }

      .live-status-item {
        display: flex;
        flex-direction: column;
        gap: 4px;
      }

      .live-status-label {
        font-size: 0.75rem;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
      }

      .live-status-value {
        font-size: 1.125rem;
        font-weight: 600;
        color: var(--text-primary);
      }

      .live-status-phase {
        display: inline-block;
        padding: 4px 12px;
        border-radius: 12px;
        font-size: 0.875rem;
        font-weight: 600;
        color: white;
      }

      .candidate-info {
        background-color: var(--bg-primary);
        border-radius: 8px;
        padding: 16px;
        border-left: 4px solid var(--accent-color, #3b82f6);
      }

      .candidate-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 12px;
      }

      .candidate-id {
        font-size: 1rem;
        font-weight: 600;
        color: var(--text-primary);
      }

      .candidate-island {
        font-size: 0.875rem;
        font-weight: 500;
        text-transform: uppercase;
      }

      .candidate-parents {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
        margin-bottom: 12px;
        font-size: 0.8125rem;
        color: var(--text-muted);
      }

      .parent-tag {
        background-color: var(--bg-tertiary);
        padding: 2px 8px;
        border-radius: 4px;
        font-family: monospace;
      }

      .candidate-validation {
        display: flex;
        gap: 8px;
        margin-bottom: 12px;
      }

      .validation-stage {
        font-size: 0.8125rem;
        padding: 4px 8px;
        border-radius: 4px;
        background-color: var(--bg-tertiary);
        color: var(--text-muted);
      }

      .validation-stage.passed {
        background-color: rgba(34, 197, 94, 0.2);
        color: #22c55e;
      }

      .candidate-evaluation {
        display: flex;
        align-items: center;
        gap: 12px;
      }

      .evaluation-progress {
        flex: 1;
        height: 6px;
        background-color: var(--bg-tertiary);
        border-radius: 3px;
        overflow: hidden;
      }

      .evaluation-bar {
        height: 100%;
        background-color: var(--accent-color, #3b82f6);
        transition: width 0.3s;
      }

      .evaluation-text {
        font-size: 0.8125rem;
        color: var(--text-muted);
      }

      /* Statistics grid */
      .stats-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
        gap: 16px;
      }

      .stat-card {
        background-color: var(--bg-primary);
        border-radius: 8px;
        padding: 16px;
        text-align: center;
      }

      .stat-label {
        font-size: 0.75rem;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
        margin-bottom: 8px;
      }

      .stat-value {
        font-size: 1.5rem;
        font-weight: 700;
        color: var(--text-primary);
      }

      /* Activity feed */
      .activity-feed {
        display: flex;
        flex-direction: column;
        gap: 8px;
      }

      .activity-entry {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 8px 12px;
        background-color: var(--bg-primary);
        border-radius: 6px;
        font-size: 0.875rem;
      }

      .activity-time {
        color: var(--text-muted);
        font-size: 0.8125rem;
        min-width: 60px;
      }

      .activity-result {
        font-weight: 600;
        min-width: 90px;
      }

      .activity-result.result-promoted {
        color: #22c55e;
      }

      .activity-result.result-rejected {
        color: #ef4444;
      }

      .activity-candidate {
        font-family: monospace;
        color: var(--text-primary);
      }

      .activity-island {
        text-transform: uppercase;
        font-size: 0.8125rem;
        min-width: 60px;
      }

      .activity-reason {
        color: var(--text-muted);
        font-size: 0.8125rem;
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      /* Island status grid */
      .island-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
        gap: 16px;
      }

      .island-card {
        background-color: var(--bg-primary);
        border-radius: 8px;
        padding: 16px;
        border-left: 4px solid transparent;
      }

      .island-card-name {
        font-size: 0.875rem;
        font-weight: 600;
        color: var(--text-primary);
        margin-bottom: 12px;
      }

      .island-stat-row {
        display: flex;
        justify-content: space-between;
        margin-bottom: 6px;
        font-size: 0.8125rem;
      }

      .island-stat-label {
        color: var(--text-muted);
      }

      .island-stat-value {
        color: var(--text-primary);
        font-weight: 500;
      }

      .island-diversity-bar {
        height: 4px;
        background-color: var(--bg-tertiary);
        border-radius: 2px;
        margin-top: 10px;
        overflow: hidden;
      }

      .island-diversity-fill {
        height: 100%;
        border-radius: 2px;
        transition: width 0.3s;
      }

      /* Chart */
      .chart-container {
        overflow-x: auto;
      }

      .meta-chart-svg {
        display: block;
        min-width: 500px;
      }

      .chart-empty {
        color: var(--text-muted);
        padding: 20px 0;
        font-size: 0.875rem;
      }

      /* Lineage tree */
      .lineage-container {
        overflow: auto;
        max-height: 480px;
        cursor: grab;
      }

      .lineage-svg {
        display: block;
      }

      /* Generation log table */
      .gen-log-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 0.875rem;
      }

      .gen-log-table th,
      .gen-log-table td {
        padding: 10px 14px;
        text-align: left;
        border-bottom: 1px solid var(--bg-tertiary);
      }

      .gen-log-table th {
        background-color: var(--bg-tertiary);
        color: var(--text-muted);
        font-weight: 600;
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
      }

      .gen-log-table tr:last-child td {
        border-bottom: none;
      }

      .gen-log-table tr:hover td {
        background-color: var(--bg-tertiary);
      }

      .island-dot {
        display: inline-block;
        width: 8px;
        height: 8px;
        border-radius: 50%;
        margin-right: 6px;
        vertical-align: middle;
      }

      .fitness-bar-cell {
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .fitness-bar-bg {
        flex: 1;
        height: 6px;
        background-color: var(--bg-tertiary);
        border-radius: 3px;
        overflow: hidden;
        min-width: 60px;
      }

      .fitness-bar-fill {
        height: 100%;
        border-radius: 3px;
      }

      @media (max-width: 700px) {
        .island-grid {
          grid-template-columns: 1fr 1fr;
        }
        .stats-grid {
          grid-template-columns: 1fr 1fr;
        }
      }

      @media (max-width: 480px) {
        .island-grid {
          grid-template-columns: 1fr;
        }
        .stats-grid {
          grid-template-columns: 1fr;
        }
      }
    </style>
  `;

  renderIslandGrid(document.getElementById('island-grid')!, data.islands);
  renderLiveStatus(document.getElementById('live-status')!, data.cycle);
  renderStatistics(document.getElementById('statistics')!, data.totals);
  renderActivityFeed(document.getElementById('activity-feed')!, data.recent_activity || []);
  renderMetaChart(document.getElementById('meta-chart')!, data.meta_snapshots ?? []);
  renderLineageTree(document.getElementById('lineage-tree')!, data.lineage ?? []);
  renderGenerationLog(document.getElementById('generation-log')!, data.generation_log ?? []);
}

// ── Island Status ──────────────────────────────────────────────────────────────

function renderIslandGrid(container: HTMLElement, islands: Record<string, IslandStat>): void {
  const islandOrder = ['alpha', 'beta', 'gamma', 'delta'];
  const cards = islandOrder.map(island => {
    const stat = islands[island];
    if (!stat) return '';
    const color = ISLAND_COLORS[island] ?? '#94a3b8';
    const label = ISLAND_LABELS[island] ?? island;
    return `
      <div class="island-card" style="border-left-color: ${color}">
        <div class="island-card-name" style="color: ${color}">${escapeHtml(label)}</div>
        <div class="island-stat-row">
          <span class="island-stat-label">Population</span>
          <span class="island-stat-value">${stat.population}</span>
        </div>
        <div class="island-stat-row">
          <span class="island-stat-label">Best Rating</span>
          <span class="island-stat-value">${stat.best_rating}</span>
        </div>
        <div class="island-stat-row">
          <span class="island-stat-label">Best Bot</span>
          <span class="island-stat-value" style="font-family: monospace; font-size: 0.8rem;">${escapeHtml(stat.best_bot || '—')}</span>
        </div>
      </div>
    `;
  });
  container.innerHTML = cards.join('');
}

// ── Live Status ─────────────────────────────────────────────────────────────────

function renderLiveStatus(container: HTMLElement, cycle: CycleInfo | undefined): void {
  if (!cycle) {
    container.innerHTML = '<p style="color: var(--text-muted); font-size: 0.875rem;">No active cycle. Evolution is idle.</p>';
    return;
  }

  const phaseColors: Record<string, string> = {
    idle: '#94a3b8',
    generating: '#f59e0b',
    validating: '#3b82f6',
    evaluating: '#8b5cf6',
    promoting: '#22c55e',
  };

  const phaseLabel = cycle.phase.charAt(0).toUpperCase() + cycle.phase.slice(1);

  container.innerHTML = `
    <div class="live-status-container">
      <div class="live-status-main">
        <div class="live-status-item">
          <span class="live-status-label">Generation</span>
          <span class="live-status-value">#${cycle.generation}</span>
        </div>
        <div class="live-status-item">
          <span class="live-status-label">Phase</span>
          <span class="live-status-phase" style="background-color: ${phaseColors[cycle.phase] || '#94a3b8'}">${phaseLabel}</span>
        </div>
        <div class="live-status-item">
          <span class="live-status-label">Started</span>
          <span class="live-status-value">${formatTimestamp(cycle.started_at)}</span>
        </div>
      </div>
      ${cycle.candidate ? renderCandidateInfo(cycle.candidate) : ''}
    </div>
  `;
}

function renderCandidateInfo(candidate: Candidate): string {
  let statusHTML = '';

  if (candidate.validation) {
    const v = candidate.validation;
    statusHTML += `
      <div class="candidate-validation">
        <div class="validation-stage ${v.syntax?.passed ? 'passed' : 'pending'}">Syntax ${v.syntax?.passed ? '✓' : '⋯'}</div>
        <div class="validation-stage ${v.schema?.passed ? 'passed' : 'pending'}">Schema ${v.schema?.passed ? '✓' : '⋯'}</div>
        <div class="validation-stage ${v.smoke?.passed ? 'passed' : 'pending'}">Smoke ${v.smoke?.passed ? '✓' : '⋯'}</div>
      </div>
    `;
  }

  if (candidate.evaluation && candidate.evaluation.matches_total > 0) {
    const played = candidate.evaluation.matches_played;
    const total = candidate.evaluation.matches_total;
    const pct = Math.round((played / total) * 100);
    statusHTML += `
      <div class="candidate-evaluation">
        <div class="evaluation-progress">
          <div class="evaluation-bar" style="width: ${pct}%"></div>
        </div>
        <span class="evaluation-text">Evaluating: ${played}/${total} matches</span>
      </div>
    `;
  }

  return `
    <div class="candidate-info">
      <div class="candidate-header">
        <span class="candidate-id">${escapeHtml(candidate.id)}</span>
        <span class="candidate-island" style="color: ${ISLAND_COLORS[candidate.island] || '#94a3b8'}">${escapeHtml(candidate.island)}</span>
      </div>
      <div class="candidate-parents">
        Parents: ${candidate.parents.map(p => `<span class="parent-tag">${escapeHtml(p.id)} (${p.rating})</span>`).join('')}
      </div>
      ${statusHTML}
    </div>
  `;
}

// ── Statistics ─────────────────────────────────────────────────────────────────

function renderStatistics(container: HTMLElement, totals: Totals): void {
  container.innerHTML = `
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-label">Total Generations</div>
        <div class="stat-value">${totals.generations_total}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Candidates Today</div>
        <div class="stat-value">${totals.candidates_today}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Promoted Today</div>
        <div class="stat-value">${totals.promoted_today}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Promotion Rate (7d)</div>
        <div class="stat-value">${(totals.promotion_rate_7d * 100).toFixed(1)}%</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Highest Evolved Rating</div>
        <div class="stat-value">${totals.highest_evolved_rating}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Evolved in Top 10</div>
        <div class="stat-value">${totals.evolved_in_top_10}</div>
      </div>
    </div>
  `;
}

// ── Activity Feed ───────────────────────────────────────────────────────────────

function renderActivityFeed(container: HTMLElement, activities: ActivityEntry[]): void {
  if (!activities || activities.length === 0) {
    container.innerHTML = '<p style="color: var(--text-muted); font-size: 0.875rem;">No recent activity.</p>';
    return;
  }

  const rows = activities.map(a => {
    const resultClass = a.result === 'promoted' ? 'result-promoted' : 'result-rejected';
    const resultIcon = a.result === 'promoted' ? '🟢' : '🔴';
    const color = ISLAND_COLORS[a.island] || '#94a3b8';

    return `
      <div class="activity-entry">
        <span class="activity-time">${formatTimeAgo(a.time)}</span>
        <span class="activity-result ${resultClass}">${resultIcon} ${escapeHtml(a.result)}</span>
        <span class="activity-candidate">${escapeHtml(a.candidate)}</span>
        <span class="activity-island" style="color: ${color}">${escapeHtml(a.island)}</span>
        <span class="activity-reason">${escapeHtml(a.reason)}</span>
      </div>
    `;
  }).join('');

  container.innerHTML = `<div class="activity-feed">${rows}</div>`;
}

function formatTimeAgo(iso: string): string {
  try {
    const then = new Date(iso).getTime();
    const now = Date.now();
    const seconds = Math.floor((now - then) / 1000);

    if (seconds < 60) return `${seconds}s ago`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
    return `${Math.floor(seconds / 86400)}d ago`;
  } catch {
    return iso;
  }
}

// ── Meta Tracker Chart ─────────────────────────────────────────────────────────

function renderMetaChart(container: HTMLElement, snapshots: MetaSnapshot[]): void {
  if (!snapshots || snapshots.length === 0) {
    container.innerHTML = '<p class="chart-empty">No generation data yet.</p>';
    return;
  }

  const islands = ['alpha', 'beta', 'gamma', 'delta'];
  const W = 700, H = 220;
  const padL = 44, padR = 16, padT = 16, padB = 36;
  const chartW = W - padL - padR;
  const chartH = H - padT - padB;

  const gens = snapshots.map(s => s.generation);
  const minGen = gens[0];
  const maxGen = gens[gens.length - 1];
  const genRange = Math.max(maxGen - minGen, 1);

  // Find max count across all islands/snapshots for Y scale
  let maxCount = 1;
  for (const snap of snapshots) {
    for (const island of islands) {
      const v = snap.island_counts[island] ?? 0;
      if (v > maxCount) maxCount = v;
    }
  }

  const xOf = (gen: number) => padL + ((gen - minGen) / genRange) * chartW;
  const yOf = (v: number) => padT + chartH - (v / maxCount) * chartH;

  const lineEls: string[] = [];
  const dotEls: string[] = [];
  const legendEls: string[] = [];

  for (const island of islands) {
    const color = ISLAND_COLORS[island] ?? '#94a3b8';
    const points = snapshots.map(s => ({
      x: xOf(s.generation),
      y: yOf(s.island_counts[island] ?? 0),
    }));

    if (points.length < 2) {
      // single point — draw a dot
      if (points.length === 1) {
        dotEls.push(`<circle cx="${points[0].x}" cy="${points[0].y}" r="4" fill="${color}" />`);
      }
    } else {
      const d = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ');
      lineEls.push(`<path d="${d}" fill="none" stroke="${color}" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />`);
      for (const p of points) {
        dotEls.push(`<circle cx="${p.x.toFixed(1)}" cy="${p.y.toFixed(1)}" r="3" fill="${color}" />`);
      }
    }
  }

  // Legend
  islands.forEach((island, i) => {
    const color = ISLAND_COLORS[island] ?? '#94a3b8';
    const lx = padL + i * 120;
    const ly = H - 6;
    legendEls.push(`
      <circle cx="${lx + 6}" cy="${ly - 4}" r="4" fill="${color}" />
      <text x="${lx + 14}" y="${ly}" fill="#94a3b8" font-size="11">${escapeHtml(ISLAND_LABELS[island] ?? island)}</text>
    `);
  });

  // Y axis ticks
  const yTicks: string[] = [];
  const tickCount = 4;
  for (let i = 0; i <= tickCount; i++) {
    const v = Math.round((maxCount / tickCount) * i);
    const y = yOf(v);
    yTicks.push(`
      <line x1="${padL - 4}" y1="${y.toFixed(1)}" x2="${W - padR}" y2="${y.toFixed(1)}"
            stroke="#334155" stroke-width="1" />
      <text x="${padL - 7}" y="${(y + 4).toFixed(1)}" fill="#94a3b8" font-size="10" text-anchor="end">${v}</text>
    `);
  }

  // X axis ticks (up to 6)
  const xTicks: string[] = [];
  const xTickCount = Math.min(6, snapshots.length);
  const step = Math.max(1, Math.floor(snapshots.length / xTickCount));
  for (let i = 0; i < snapshots.length; i += step) {
    const snap = snapshots[i];
    const x = xOf(snap.generation);
    xTicks.push(`
      <text x="${x.toFixed(1)}" y="${(padT + chartH + 18).toFixed(1)}"
            fill="#94a3b8" font-size="10" text-anchor="middle">G${snap.generation}</text>
    `);
  }

  container.innerHTML = `
    <svg class="meta-chart-svg" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}">
      ${yTicks.join('')}
      ${xTicks.join('')}
      ${lineEls.join('')}
      ${dotEls.join('')}
      ${legendEls.join('')}
    </svg>
  `;
}

// ── Lineage Tree ───────────────────────────────────────────────────────────────

function renderLineageTree(container: HTMLElement, nodes: LineageNode[]): void {
  if (!nodes || nodes.length === 0) {
    container.innerHTML = '<p style="color: var(--text-muted); font-size: 0.875rem;">No lineage data yet.</p>';
    return;
  }

  // Keep top 80 by fitness to keep the tree readable
  const sorted = [...nodes].sort((a, b) => b.fitness - a.fitness).slice(0, 80);
  const nodeById = new Map<number, LineageNode>(sorted.map(n => [n.id as unknown as number, n]));

  // Group by generation for Y layout
  const genSet = new Set(sorted.map(n => n.generation));
  const gens = Array.from(genSet).sort((a, b) => a - b);
  const genIndex = new Map(gens.map((g, i) => [g, i]));
  const maxGenIdx = gens.length - 1;

  const NODE_R = 6;
  const H_GAP = 38;  // horizontal spacing between nodes on same generation
  const V_GAP = 54;  // vertical spacing between generation rows
  const PAD_X = 20;
  const PAD_Y = 20;

  // Count nodes per generation for X layout
  const nodesPerGen = new Map<number, LineageNode[]>();
  for (const n of sorted) {
    if (!nodesPerGen.has(n.generation)) nodesPerGen.set(n.generation, []);
    nodesPerGen.get(n.generation)!.push(n);
  }

  // Assign x positions — spread per generation
  const nodePos = new Map<number, { x: number; y: number }>();
  for (const [gen, genNodes] of nodesPerGen) {
    const gIdx = genIndex.get(gen) ?? 0;
    const y = PAD_Y + gIdx * V_GAP;
    genNodes.forEach((n, i) => {
      const x = PAD_X + i * H_GAP;
      nodePos.set(n.id as unknown as number, { x, y });
    });
  }

  // SVG dimensions
  const svgW = Math.max(...Array.from(nodePos.values()).map(p => p.x)) + PAD_X + NODE_R + 60;
  const svgH = PAD_Y + maxGenIdx * V_GAP + PAD_Y + 20;

  const edges: string[] = [];
  const nodeEls: string[] = [];

  // Draw edges
  for (const n of sorted) {
    const pos = nodePos.get(n.id as unknown as number);
    if (!pos) continue;
    for (const pid of (n.parent_ids ?? [])) {
      if (!nodeById.has(pid as unknown as number)) continue;
      const ppos = nodePos.get(pid as unknown as number);
      if (!ppos) continue;
      edges.push(`<line x1="${pos.x}" y1="${pos.y}" x2="${ppos.x}" y2="${ppos.y}"
        stroke="#475569" stroke-width="1" stroke-dasharray="3,2" />`);
    }
  }

  // Draw nodes
  for (const n of sorted) {
    const pos = nodePos.get(n.id as unknown as number);
    if (!pos) continue;
    const color = ISLAND_COLORS[n.island] ?? '#94a3b8';
    const strokeW = n.promoted ? 2.5 : 1;
    const strokeColor = n.promoted ? '#ffffff' : color;
    const r = n.promoted ? NODE_R + 2 : NODE_R;
    const title = `#${n.id} ${n.island} gen${n.generation} ${n.language} fit=${(n.fitness * 100).toFixed(1)}%${n.promoted ? ' PROMOTED' : ''}`;
    nodeEls.push(`
      <circle cx="${pos.x}" cy="${pos.y}" r="${r}"
        fill="${color}" stroke="${strokeColor}" stroke-width="${strokeW}"
        opacity="0.9">
        <title>${escapeHtml(title)}</title>
      </circle>
    `);
  }

  // Generation labels on the left
  const genLabels = gens.map(gen => {
    const gIdx = genIndex.get(gen) ?? 0;
    const y = PAD_Y + gIdx * V_GAP;
    return `<text x="0" y="${y + 4}" fill="#475569" font-size="10" font-family="monospace">G${gen}</text>`;
  });

  // Legend
  const legendIslands = ['alpha', 'beta', 'gamma', 'delta'];
  const legendY = svgH - 4;
  const legendEls = legendIslands.map((island, i) => {
    const color = ISLAND_COLORS[island] ?? '#94a3b8';
    const lx = PAD_X + i * 110;
    return `
      <circle cx="${lx + 5}" cy="${legendY - 4}" r="5" fill="${color}" />
      <text x="${lx + 14}" y="${legendY}" fill="#94a3b8" font-size="10">${island}</text>
    `;
  });
  const legendPromo = `
    <circle cx="${PAD_X + 450}" cy="${legendY - 4}" r="7" fill="#94a3b8" stroke="#ffffff" stroke-width="2.5" />
    <text x="${PAD_X + 462}" y="${legendY}" fill="#94a3b8" font-size="10">promoted</text>
  `;

  const fullSvgH = svgH + 20;

  container.innerHTML = `
    <svg class="lineage-svg" viewBox="0 0 ${svgW} ${fullSvgH}" width="${svgW}" height="${fullSvgH}">
      <g transform="translate(36,0)">
        ${edges.join('')}
        ${nodeEls.join('')}
      </g>
      <g transform="translate(0,0)">
        ${genLabels.join('')}
      </g>
      <g>
        ${legendEls.join('')}
        ${legendPromo}
      </g>
    </svg>
  `;
}

// ── Generation Log Table ───────────────────────────────────────────────────────

function renderGenerationLog(container: HTMLElement, log: GenerationEntry[]): void {
  if (!log || log.length === 0) {
    container.innerHTML = '<p style="color: var(--text-muted); font-size: 0.875rem;">No generation history yet.</p>';
    return;
  }

  const rows = log.map(e => {
    const color = ISLAND_COLORS[e.island] ?? '#94a3b8';
    const bestPct = (e.best_fitness * 100).toFixed(1);
    const avgPct  = (e.avg_fitness  * 100).toFixed(1);
    const barWidth = Math.round(e.best_fitness * 100);
    return `
      <tr>
        <td>${e.generation}</td>
        <td><span class="island-dot" style="background-color:${color}"></span>${escapeHtml(e.island)}</td>
        <td>${e.count}</td>
        <td>${e.promoted}</td>
        <td>
          <div class="fitness-bar-cell">
            <span style="min-width:42px; color: var(--text-primary)">${bestPct}%</span>
            <div class="fitness-bar-bg">
              <div class="fitness-bar-fill" style="width:${barWidth}%; background-color:${color}"></div>
            </div>
          </div>
        </td>
        <td>${avgPct}%</td>
        <td style="color: var(--text-muted); font-size: 0.75rem;">${formatTimestamp(e.evaluated_at)}</td>
      </tr>
    `;
  });

  container.innerHTML = `
    <table class="gen-log-table">
      <thead>
        <tr>
          <th>Gen</th>
          <th>Island</th>
          <th>Programs</th>
          <th>Promoted</th>
          <th>Best Fitness</th>
          <th>Avg Fitness</th>
          <th>Timestamp</th>
        </tr>
      </thead>
      <tbody>
        ${rows.join('')}
      </tbody>
    </table>
  `;
}

// ── Helpers ────────────────────────────────────────────────────────────────────

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
