// Rivalries page: detect head-to-head rivalries from match data and
// render narrative cards with template-generated storylines.

import { fetchMatchIndex, fetchLeaderboard, type MatchSummary } from '../api-types';

// ─── Types ────────────────────────────────────────────────────────────────────

interface Rivalry {
  bot0Id: string;
  bot0Name: string;
  bot1Id: string;
  bot1Name: string;
  totalMatches: number;
  bot0Wins: number;
  bot1Wins: number;
  draws: number;
  lastMatchAt: string;
  rivalryScore: number; // higher = more intense (frequent + close)
  narrative: string;
  streak: { bot: string; count: number } | null; // current win streak
}

// ─── Page render ─────────────────────────────────────────────────────────────

export async function renderRivalriesPage(_params: Record<string, string>): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="rivalries-page">
      <h1 class="page-title">Rivalries</h1>
      <p class="page-subtitle">Head-to-head storylines from the most contested matchups on the grid.</p>
      <div id="rivalries-content" class="loading">Analysing match history…</div>
    </div>
    ${RIVALRY_STYLES}
  `;

  const content = document.getElementById('rivalries-content')!;

  try {
    const [matchIdx, leaderboard] = await Promise.all([
      fetchMatchIndex().catch(() => ({ matches: [], updated_at: '' })),
      fetchLeaderboard().catch(() => ({ entries: [], updated_at: '' })),
    ]);

    const nameMap = new Map<string, string>();
    for (const e of leaderboard.entries) nameMap.set(e.bot_id, e.name);

    const rivalries = detectRivalries(matchIdx.matches, nameMap);

    if (rivalries.length === 0) {
      content.innerHTML = `
        <div class="empty-state">
          <p>No rivalries detected yet.</p>
          <p class="hint">Rivalries appear when two bots have played at least 3 head-to-head matches.
             Check back after more matches have been recorded.</p>
        </div>
      `;
      return;
    }

    renderRivalryCards(content, rivalries);
  } catch (err) {
    content.innerHTML = `<div class="error">Failed to load rivalry data: ${err}</div>`;
  }
}

// ─── Rivalry detection ────────────────────────────────────────────────────────

function detectRivalries(matches: MatchSummary[], nameMap: Map<string, string>): Rivalry[] {
  // Accumulate head-to-head records between every bot pair
  type PairKey = string;
  interface PairRecord {
    bot0: string;
    bot1: string;
    wins0: number;
    wins1: number;
    draws: number;
    lastAt: string;
    matchIds: string[];
    lastWinner: string | null;
    currentStreak: number; // positive = bot0 streak, negative = bot1 streak
  }

  const pairMap = new Map<PairKey, PairRecord>();

  const pairKey = (a: string, b: string): PairKey =>
    a < b ? `${a}||${b}` : `${b}||${a}`;

  const sortedMatches = [...matches].sort(
    (a, b) => new Date(a.completed_at ?? 0).getTime() - new Date(b.completed_at ?? 0).getTime(),
  );

  for (const m of sortedMatches) {
    if (m.participants.length < 2) continue;
    const [p0, p1] = m.participants;
    const key = pairKey(p0.bot_id, p1.bot_id);

    let rec = pairMap.get(key);
    if (!rec) {
      // Canonicalize: alphabetically first bot_id is bot0
      const [b0, b1] = p0.bot_id < p1.bot_id ? [p0, p1] : [p1, p0];
      rec = { bot0: b0.bot_id, bot1: b1.bot_id, wins0: 0, wins1: 0, draws: 0, lastAt: '', matchIds: [], lastWinner: null, currentStreak: 0 };
      pairMap.set(key, rec);
    }

    rec.matchIds.push(m.id);
    rec.lastAt = m.completed_at ?? rec.lastAt;

    const winner = m.winner_id;
    if (!winner) {
      rec.draws++;
      rec.currentStreak = 0;
      rec.lastWinner = null;
    } else if (winner === rec.bot0) {
      rec.wins0++;
      rec.currentStreak = rec.lastWinner === rec.bot0 ? rec.currentStreak + 1 : 1;
      rec.lastWinner = rec.bot0;
    } else {
      rec.wins1++;
      rec.currentStreak = rec.lastWinner === rec.bot1 ? rec.currentStreak - 1 : -1;
      rec.lastWinner = rec.bot1;
    }
  }

  const rivalries: Rivalry[] = [];

  for (const rec of pairMap.values()) {
    const total = rec.wins0 + rec.wins1 + rec.draws;
    if (total < 3) continue; // minimum threshold for a rivalry

    const closeness = 1 - Math.abs(rec.wins0 - rec.wins1) / Math.max(1, total);
    const rivalryScore = total * closeness;

    const bot0Name = nameMap.get(rec.bot0) ?? rec.bot0.slice(0, 8);
    const bot1Name = nameMap.get(rec.bot1) ?? rec.bot1.slice(0, 8);

    let streak: Rivalry['streak'] = null;
    if (Math.abs(rec.currentStreak) >= 2) {
      streak = {
        bot: rec.currentStreak > 0 ? bot0Name : bot1Name,
        count: Math.abs(rec.currentStreak),
      };
    }

    rivalries.push({
      bot0Id: rec.bot0,
      bot0Name,
      bot1Id: rec.bot1,
      bot1Name,
      totalMatches: total,
      bot0Wins: rec.wins0,
      bot1Wins: rec.wins1,
      draws: rec.draws,
      lastMatchAt: rec.lastAt,
      rivalryScore,
      narrative: buildNarrative({
        bot0Name, bot1Name, total,
        wins0: rec.wins0, wins1: rec.wins1, draws: rec.draws,
        streak,
      }),
      streak,
    });
  }

  // Sort by rivalry score (most intense first)
  rivalries.sort((a, b) => b.rivalryScore - a.rivalryScore);

  return rivalries.slice(0, 20); // top 20
}

// ─── Template narrative builder ───────────────────────────────────────────────

interface NarrativeVars {
  bot0Name: string;
  bot1Name: string;
  total: number;
  wins0: number;
  wins1: number;
  draws: number;
  streak: { bot: string; count: number } | null;
}

function buildNarrative(v: NarrativeVars): string {
  const leading = v.wins0 >= v.wins1 ? v.bot0Name : v.bot1Name;
  const trailing = v.wins0 >= v.wins1 ? v.bot1Name : v.bot0Name;
  const leadWins = Math.max(v.wins0, v.wins1);
  const trailWins = Math.min(v.wins0, v.wins1);
  const winRate = leadWins / Math.max(1, v.total);

  if (Math.abs(v.wins0 - v.wins1) === 0) {
    // Perfect tie
    return pickTemplate(TIED_NARRATIVES, { ...v, leading, trailing });
  } else if (winRate >= 0.75) {
    // Dominant
    return pickTemplate(DOMINANT_NARRATIVES, { ...v, leading, trailing, leadWins, trailWins });
  } else if (v.streak && v.streak.count >= 3) {
    // Streak
    return pickTemplate(STREAK_NARRATIVES, { ...v, leading, trailing, streakBot: v.streak.bot, streakCount: v.streak.count });
  } else {
    // Close contest
    return pickTemplate(CLOSE_NARRATIVES, { ...v, leading, trailing, leadWins, trailWins });
  }
}

function pickTemplate(templates: string[], vars: Record<string, any>): string {
  const tmpl = templates[Math.floor(Math.random() * templates.length)];
  return tmpl.replace(/\{(\w+)\}/g, (_, k) => String(vars[k] ?? `{${k}}`));
}

const TIED_NARRATIVES = [
  "{bot0Name} and {bot1Name} are locked in perfect equilibrium after {total} clashes — every victory answered in kind.",
  "The grid cannot separate {bot0Name} from {bot1Name}. After {total} battles, honours remain exactly even.",
  "{bot0Name} vs {bot1Name}: {total} encounters, zero separation. The ultimate standoff continues.",
  "Neither {bot0Name} nor {bot1Name} can claim the edge in their {total}-match duel. This rivalry defines balance.",
];

const DOMINANT_NARRATIVES = [
  "{leading} has established clear dominance over {trailing}, leading {leadWins}–{trailWins} across {total} meetings.",
  "In {total} encounters, {leading} has proven superior to {trailing} with a commanding {leadWins}–{trailWins} record.",
  "{trailing} continues its search for answers against {leading}, who holds a decisive {leadWins}–{trailWins} advantage.",
  "{leading}'s {leadWins}–{trailWins} record against {trailing} speaks volumes — a rivalry that reads like a masterclass.",
];

const STREAK_NARRATIVES = [
  "{streakBot} has won {streakCount} straight against its rival. The momentum in this matchup has shifted dramatically.",
  "A {streakCount}-match winning streak for {streakBot} — {leading} and {trailing} are no longer evenly matched.",
  "{streakBot} is on fire, rolling off {streakCount} consecutive wins in this heated rivalry.",
  "Can anyone stop {streakBot}? A {streakCount}-match streak in their rivalry says the answer, for now, is no.",
];

const CLOSE_NARRATIVES = [
  "{leading} holds a slim {leadWins}–{trailWins} edge over {trailing} after {total} closely contested matches.",
  "Just {leadWins} vs {trailWins} separates {leading} from {trailing} across {total} grid battles. Every match matters.",
  "The {bot0Name}–{bot1Name} rivalry is defined by razor-thin margins: {leadWins} wins to {trailWins} after {total} encounters.",
  "{leading} leads {trailing} {leadWins}–{trailWins} but the gap could close in a single session — that's what makes this rivalry great.",
];

// ─── Card renderer ────────────────────────────────────────────────────────────

function renderRivalryCards(container: HTMLElement, rivalries: Rivalry[]): void {
  const dateStr = (s: string) => {
    if (!s) return '–';
    return new Date(s).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
  };

  container.innerHTML = `
    <div class="rivalry-grid">
      ${rivalries.map((r, i) => `
        <div class="rivalry-card ${i === 0 ? 'featured' : ''}">
          ${i === 0 ? '<div class="rivalry-badge">Top Rivalry</div>' : ''}
          <div class="rivalry-header">
            <div class="rivalry-combatant">
              <a href="#/bot/${r.bot0Id}" class="combatant-name">${escapeHtml(r.bot0Name)}</a>
              <span class="combatant-record">${r.bot0Wins}W</span>
            </div>
            <div class="rivalry-vs">
              <span class="vs-text">VS</span>
              <span class="rivalry-total">${r.totalMatches} matches</span>
            </div>
            <div class="rivalry-combatant right">
              <a href="#/bot/${r.bot1Id}" class="combatant-name">${escapeHtml(r.bot1Name)}</a>
              <span class="combatant-record">${r.bot1Wins}W</span>
            </div>
          </div>

          <div class="win-bar-container">
            ${buildWinBar(r)}
          </div>

          <p class="rivalry-narrative">${escapeHtml(r.narrative)}</p>

          <div class="rivalry-footer">
            ${r.streak ? `<span class="streak-badge">${escapeHtml(r.streak.bot)} on ${r.streak.count}-win streak</span>` : ''}
            ${r.draws > 0 ? `<span class="draws-tag">${r.draws} draw${r.draws !== 1 ? 's' : ''}</span>` : ''}
            <span class="last-match">Last: ${dateStr(r.lastMatchAt)}</span>
            <a href="#/matches?bot0=${r.bot0Id}&bot1=${r.bot1Id}" class="btn small secondary">All Matches</a>
          </div>
        </div>
      `).join('')}
    </div>
  `;
}

function buildWinBar(r: Rivalry): string {
  const total = r.totalMatches;
  const pct0 = total > 0 ? (r.bot0Wins / total) * 100 : 50;
  const pctD = total > 0 ? (r.draws / total) * 100 : 0;
  const pct1 = 100 - pct0 - pctD;
  return `
    <div class="win-bar">
      <div class="win-bar-seg seg0" style="width:${pct0.toFixed(1)}%" title="${r.bot0Name}: ${r.bot0Wins} wins"></div>
      <div class="win-bar-seg seg-draw" style="width:${pctD.toFixed(1)}%" title="Draws: ${r.draws}"></div>
      <div class="win-bar-seg seg1" style="width:${pct1.toFixed(1)}%" title="${r.bot1Name}: ${r.bot1Wins} wins"></div>
    </div>
    <div class="win-bar-labels">
      <span style="color:#3b82f6">${r.bot0Wins}W (${pct0.toFixed(0)}%)</span>
      <span style="color:#94a3b8">${r.draws > 0 ? r.draws + ' draws' : ''}</span>
      <span style="color:#ef4444">${pct1.toFixed(0)}% (${r.bot1Wins}W)</span>
    </div>
  `;
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// ─── Styles ───────────────────────────────────────────────────────────────────

const RIVALRY_STYLES = `
<style>
.rivalries-page .page-subtitle { color: var(--text-muted); margin-bottom: 24px; }
.rivalry-grid { display: flex; flex-direction: column; gap: 16px; }
.rivalry-card { background: var(--bg-secondary); border-radius: 10px; padding: 20px; position: relative; border: 1px solid var(--border); }
.rivalry-card.featured { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent); }
.rivalry-badge { position: absolute; top: -1px; right: 20px; background: var(--accent); color: #fff; font-size: 0.7rem; font-weight: 700; padding: 3px 10px; border-radius: 0 0 6px 6px; text-transform: uppercase; letter-spacing: 0.05em; }
.rivalry-header { display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; gap: 16px; margin-bottom: 12px; }
.rivalry-combatant { display: flex; flex-direction: column; gap: 4px; }
.rivalry-combatant.right { align-items: flex-end; text-align: right; }
.combatant-name { color: var(--text-primary); text-decoration: none; font-size: 1.1rem; font-weight: 600; }
.combatant-name:hover { color: var(--accent); }
.combatant-record { font-size: 0.875rem; color: var(--text-muted); }
.rivalry-vs { text-align: center; }
.vs-text { display: block; font-size: 1.25rem; font-weight: 800; color: var(--text-muted); }
.rivalry-total { font-size: 0.7rem; color: var(--text-muted); }
.win-bar-container { margin: 12px 0; }
.win-bar { height: 10px; border-radius: 5px; overflow: hidden; display: flex; }
.win-bar-seg { height: 100%; }
.seg0 { background: #3b82f6; }
.seg-draw { background: #475569; }
.seg1 { background: #ef4444; }
.win-bar-labels { display: flex; justify-content: space-between; font-size: 0.7rem; margin-top: 4px; }
.rivalry-narrative { color: var(--text-muted); font-style: italic; font-size: 0.875rem; margin: 12px 0; line-height: 1.5; }
.rivalry-footer { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-top: 12px; }
.streak-badge { background: rgba(245,158,11,0.15); color: var(--warning); font-size: 0.75rem; padding: 3px 8px; border-radius: 12px; }
.draws-tag { background: var(--bg-tertiary); color: var(--text-muted); font-size: 0.75rem; padding: 3px 8px; border-radius: 12px; }
.last-match { color: var(--text-muted); font-size: 0.75rem; margin-left: auto; }
.empty-state { background: var(--bg-secondary); border-radius: 8px; padding: 40px; text-align: center; color: var(--text-muted); }
.empty-state .hint { margin-top: 10px; font-size: 0.875rem; }
</style>
`;
