// Replay enrichment: template-based AI commentary for featured matches.
//
// Commentary is generated from replay event data using a curated set of
// narrative templates. For production, these can be enhanced with an LLM
// by POST-ing the context to /api/commentary.

import type { WinProbPoint, CriticalMoment } from './win-probability';

export interface CommentaryLine {
  turn: number;
  text: string;
  importance: 'low' | 'medium' | 'high';
  type: 'action' | 'analysis' | 'color' | 'milestonecomment';
}

export interface MatchCommentary {
  matchId: string;
  intro: string;
  lines: CommentaryLine[];
  outro: string;
  generatedAt: string;
}

// ────────────────────────────────────────────────────────────────────────────
// Commentary generator
// ────────────────────────────────────────────────────────────────────────────

export function generateCommentary(
  replay: any,
  winProb: WinProbPoint[],
  criticalMoments: CriticalMoment[],
  playerNames?: string[],
): MatchCommentary {
  const p0 = playerNames?.[0] ?? replay.players?.[0]?.name ?? 'Player 0';
  const p1 = playerNames?.[1] ?? replay.players?.[1]?.name ?? 'Player 1';
  const totalTurns = replay.result?.turns ?? replay.turns?.length ?? 0;
  const winner = replay.result?.winner ?? -1;
  const reason = replay.result?.reason ?? 'unknown';

  const lines: CommentaryLine[] = [];

  // Intro
  const intro = pickTemplate(INTROS, { p0, p1, turns: totalTurns, reason });

  // Scan turns for notable events
  const turns = replay.turns ?? [];
  let prevP0Prob = 0.5;

  for (const turn of turns) {
    const t = turn.turn;
    const events: any[] = turn.events ?? [];

    for (const ev of events) {
      switch (ev.type) {
        case 'bot_died':
          if (events.filter((e: any) => e.type === 'bot_died').length >= 3 && lines.every(l => l.turn !== t)) {
            lines.push({
              turn: t,
              text: pickTemplate(MASS_KILL_TEMPLATES, { p0, p1, count: events.filter((e: any) => e.type === 'bot_died').length }),
              importance: 'medium',
              type: 'action',
            });
          }
          break;
        case 'core_captured':
          lines.push({
            turn: t,
            text: pickTemplate(CORE_CAPTURE_TEMPLATES, {
              p0, p1,
              capturer: ev.details?.captureOwner === 0 ? p0 : p1,
              victim: ev.details?.coreOwner === 0 ? p0 : p1,
            }),
            importance: 'high',
            type: 'action',
          });
          break;
        case 'bot_spawned':
          if (t % 20 === 0) { // Only comment on spawns occasionally
            lines.push({
              turn: t,
              text: pickTemplate(SPAWN_TEMPLATES, {
                player: ev.details?.owner === 0 ? p0 : p1,
              }),
              importance: 'low',
              type: 'color',
            });
          }
          break;
      }
    }

    // Probability-based commentary
    const probPoint = winProb.find(wp => wp.turn === t);
    if (probPoint) {
      const delta = probPoint.p0WinProb - prevP0Prob;
      if (Math.abs(delta) >= 0.2) {
        lines.push({
          turn: t,
          text: pickTemplate(PROB_SWING_TEMPLATES, {
            p0, p1,
            leading: delta > 0 ? p0 : p1,
            trailing: delta > 0 ? p1 : p0,
            prob: Math.round(Math.max(probPoint.p0WinProb, probPoint.p1WinProb) * 100),
          }),
          importance: 'medium',
          type: 'analysis',
        });
      }
      prevP0Prob = probPoint.p0WinProb;
    }

    // Milestone turns
    if (t === Math.floor(totalTurns * 0.25)) {
      const p0Bots = turn.bots?.filter((b: any) => b.alive && b.owner === 0).length ?? 0;
      const p1Bots = turn.bots?.filter((b: any) => b.alive && b.owner === 1).length ?? 0;
      lines.push({
        turn: t,
        text: pickTemplate(QUARTER_TEMPLATES, { p0, p1, p0Bots, p1Bots }),
        importance: 'medium',
        type: 'milestonecomment',
      });
    }
    if (t === Math.floor(totalTurns * 0.5)) {
      const p0Score = turn.scores?.[0] ?? 0;
      const p1Score = turn.scores?.[1] ?? 0;
      lines.push({
        turn: t,
        text: pickTemplate(HALFWAY_TEMPLATES, { p0, p1, p0Score, p1Score }),
        importance: 'medium',
        type: 'milestonecomment',
      });
    }
  }

  // Add critical moments that aren't already covered
  for (const cm of criticalMoments) {
    if (!lines.find(l => l.turn === cm.turn)) {
      lines.push({
        turn: cm.turn,
        text: cm.description,
        importance: 'high',
        type: 'analysis',
      });
    }
  }

  // Sort by turn
  lines.sort((a, b) => a.turn - b.turn);

  // Outro
  const outro = buildOutro({ winner, p0, p1, reason, totalTurns });

  return {
    matchId: replay.match_id,
    intro,
    lines,
    outro,
    generatedAt: new Date().toISOString(),
  };
}

// ────────────────────────────────────────────────────────────────────────────
// Template rendering
// ────────────────────────────────────────────────────────────────────────────

function pickTemplate(templates: string[], vars: Record<string, any>): string {
  const tmpl = templates[Math.floor(Math.random() * templates.length)];
  return tmpl.replace(/\{(\w+)\}/g, (_, k) => String(vars[k] ?? `{${k}}`));
}

function buildOutro(vars: { winner: number; p0: string; p1: string; reason: string; totalTurns: number }): string {
  if (vars.winner < 0) return pickTemplate(DRAW_OUTROS, vars);
  const winnerName = vars.winner === 0 ? vars.p0 : vars.p1;
  const loserName = vars.winner === 0 ? vars.p1 : vars.p0;
  return pickTemplate(WIN_OUTROS, { ...vars, winner: winnerName, loser: loserName });
}

// ────────────────────────────────────────────────────────────────────────────
// Commentary renderer (HTML)
// ────────────────────────────────────────────────────────────────────────────

export function renderCommentaryPanel(container: HTMLElement, commentary: MatchCommentary, currentTurn?: number): void {
  const lines = currentTurn !== undefined
    ? commentary.lines.filter(l => l.turn <= currentTurn)
    : commentary.lines;

  container.innerHTML = `
    <div class="commentary-panel">
      <p class="commentary-intro">${escapeHtml(commentary.intro)}</p>
      <div class="commentary-feed">
        ${lines.slice(-10).reverse().map(l => `
          <div class="commentary-line importance-${l.importance}">
            <span class="commentary-turn">Turn ${l.turn}</span>
            <span class="commentary-text">${escapeHtml(l.text)}</span>
          </div>
        `).join('')}
      </div>
      ${currentTurn !== undefined && currentTurn >= (commentary.lines[commentary.lines.length - 1]?.turn ?? 0) - 5
        ? `<p class="commentary-outro">${escapeHtml(commentary.outro)}</p>` : ''}
    </div>
  `;
}

export const COMMENTARY_STYLES = `
<style>
.commentary-panel { font-size: 0.875rem; }
.commentary-intro { color: var(--text-muted); margin-bottom: 12px; font-style: italic; }
.commentary-outro { color: var(--text-primary); margin-top: 12px; font-weight: 600; }
.commentary-feed { display: flex; flex-direction: column; gap: 6px; }
.commentary-line { display: flex; gap: 10px; padding: 6px 10px; border-radius: 4px; border-left: 3px solid transparent; }
.commentary-line.importance-high { background: rgba(59,130,246,0.1); border-color: var(--accent); }
.commentary-line.importance-medium { background: rgba(245,158,11,0.08); border-color: var(--warning); }
.commentary-line.importance-low { background: var(--bg-primary); border-color: var(--bg-tertiary); }
.commentary-turn { color: var(--text-muted); min-width: 52px; font-size: 0.75rem; padding-top: 2px; }
.commentary-text { color: var(--text-secondary); flex: 1; }
</style>
`;

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// ────────────────────────────────────────────────────────────────────────────
// Template banks
// ────────────────────────────────────────────────────────────────────────────

const INTROS = [
  "Welcome to this clash between {p0} and {p1} on a {turns}-turn battlefield. May the best algorithm win!",
  "It's {p0} vs {p1} in what promises to be a tactical showdown. {turns} turns stand between them and glory.",
  "Two bots enter, one leaves victorious. {p0} and {p1} face off in a contest of strategy and speed.",
  "The grid is set, the bots are ready. {p0} against {p1} — {turns} turns to prove dominance.",
  "In the arena of silicon and logic, {p0} squares up against {p1}. Let the match begin!",
];

const MASS_KILL_TEMPLATES = [
  "Carnage on the grid! {count} bots fall in rapid succession — neither side escapes unscathed.",
  "A fierce skirmish erupts, leaving {count} units destroyed in a matter of moments.",
  "The battlefield runs hot as {count} bots are eliminated in a single dramatic turn.",
  "Chaos reigns! {count} bots are lost in a collision of forces.",
];

const CORE_CAPTURE_TEMPLATES = [
  "{capturer} strikes deep into enemy territory, razing {victim}'s core! The tactical situation shifts dramatically.",
  "A bold offensive play by {capturer} — {victim}'s core falls! This could be the turning point.",
  "{victim}'s core is captured by {capturer}'s forces. The tide of war is turning.",
  "Critical blow! {capturer} eliminates {victim}'s core, threatening to end this match early.",
];

const SPAWN_TEMPLATES = [
  "{player} is rapidly expanding its forces. Numbers could be decisive here.",
  "Steady energy collection allows {player} to keep the bot production line running.",
  "{player}'s economy is humming — fresh units pour onto the battlefield.",
];

const PROB_SWING_TEMPLATES = [
  "The models give {leading} a {prob}% win probability now — {trailing} needs to respond quickly.",
  "Statistical edge shifting toward {leading} ({prob}%). {trailing} is under pressure.",
  "{leading} has established clear momentum, pushing win probability to {prob}%.",
  "A {prob}% win probability for {leading} — but this grid has seen bigger comebacks.",
];

const QUARTER_TEMPLATES = [
  "Quarter-point check: {p0} has {p0Bots} bots, {p1} has {p1Bots}. {p0Bots > p1Bots ? p0 + ' holds the numerical edge' : p1 + ' has the numbers advantage'}.",
  "25 turns in: bot counts are {p0}:{p0Bots} vs {p1}:{p1Bots}. The positioning battle is just beginning.",
];

const HALFWAY_TEMPLATES = [
  "Halfway through! Score: {p0} at {p0Score} vs {p1} at {p1Score}. {p0Score > p1Score ? p0 : p1} leads on energy collected.",
  "The midpoint of the match sees {p0} scoring {p0Score} to {p1}'s {p1Score}. Still everything to play for.",
];

const WIN_OUTROS = [
  "{winner} clinches it via {reason}! A commanding performance that leaves no doubt about the result.",
  "Victory for {winner} by {reason} — {loser} fought hard but couldn't overcome the tactical deficit.",
  "{winner} takes the match! {reason} sealed the deal in {totalTurns} turns of intense grid warfare.",
  "What a match! {winner} prevails through {reason}. {loser} will need to reconsider its strategy.",
];

const DRAW_OUTROS = [
  "The match ends in a draw after {totalTurns} turns! An evenly matched contest that honours both competitors.",
  "Neither {p0} nor {p1} could claim dominance in {totalTurns} turns — honours even!",
  "A stalemate after {totalTurns} turns. Both bots showed equal resilience on the grid.",
];
