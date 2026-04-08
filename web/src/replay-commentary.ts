// Replay Commentary Module
// Provides AI-generated commentary for featured matches based on critical moments.

import type { Replay } from './types';
import { WinProbabilityEngine, type CriticalMoment, type WinProbPoint } from './win-probability';
import type { Replay as EngineReplay } from './engine';

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

export interface CommentarySegment {
  turn: number;
  type: 'opening' | 'critical' | 'milestone' | 'closing';
  headline: string;
  detail: string;
  playerFocus?: number; // Which player is the focus (0 or 1)
}

export interface ReplayCommentary {
  matchId: string;
  summary: string;
  segments: CommentarySegment[];
  highlights: MatchHighlight[];
  winnerNarrative: string;
}

export interface MatchHighlight {
  turn: number;
  description: string;
  importance: 'high' | 'medium' | 'low';
}

// ─────────────────────────────────────────────────────────────────────────────
// Commentary Generator
// ─────────────────────────────────────────────────────────────────────────────

export class CommentaryGenerator {
  private replay: Replay;
  private playerName0: string;
  private playerName1: string;

  constructor(replay: Replay) {
    this.replay = replay;
    this.playerName0 = replay.players[0]?.name ?? 'Player 0';
    this.playerName1 = replay.players[1]?.name ?? 'Player 1';
  }

  async generateCommentary(simulations = 30): Promise<ReplayCommentary> {
    // Compute win probabilities and critical moments
    // Cast to EngineReplay - the shape is compatible for our purposes
    const wpEngine = new WinProbabilityEngine(this.replay as unknown as EngineReplay);
    await wpEngine.computeAll(simulations, 5);
    const sparkline = wpEngine.getSparkline();
    const criticalMoments = wpEngine.getCriticalMoments();

    const segments: CommentarySegment[] = [];
    const highlights: MatchHighlight[] = [];

    // Opening commentary
    segments.push(this.generateOpeningCommentary());

    // Critical moment commentary
    for (const cm of criticalMoments) {
      const segment = this.generateCriticalMomentCommentary(cm);
      segments.push(segment);
      highlights.push({
        turn: cm.turn,
        description: cm.description,
        importance: Math.abs(cm.deltaP0) > 0.25 ? 'high' : 'medium',
      });
    }

    // Add milestone commentaries (every ~25% of match)
    segments.push(...this.generateMilestoneCommentaries(sparkline));

    // Closing commentary
    segments.push(this.generateClosingCommentary());

    // Sort segments by turn
    segments.sort((a, b) => a.turn - b.turn);

    // Generate summary
    const summary = this.generateMatchSummary(sparkline, criticalMoments);

    // Winner narrative
    const winnerNarrative = this.generateWinnerNarrative();

    return {
      matchId: this.replay.match_id,
      summary,
      segments,
      highlights,
      winnerNarrative,
    };
  }

  private generateOpeningCommentary(): CommentarySegment {
    const openings = [
      `${this.playerName0} and ${this.playerName1} square off on the grid. The opening moves will set the tone.`,
      `A new contest begins. Both commanders position their forces for the battle ahead.`,
      `The grid comes alive as ${this.playerName0} and ${this.playerName1} deploy their initial strategies.`,
      `Two bots enter the arena. Who will claim dominance?`,
    ];

    return {
      turn: 0,
      type: 'opening',
      headline: 'Match Start',
      detail: openings[Math.floor(Math.random() * openings.length)],
    };
  }

  private generateCriticalMomentCommentary(cm: CriticalMoment): CommentarySegment {
    const playerAhead = cm.deltaP0 > 0 ? this.playerName0 : this.playerName1;

    const templates = this.getCommentaryTemplates(cm.type);
    const template = templates[Math.floor(Math.random() * templates.length)];

    const headline = this.generateHeadline(cm);
    const detail = template
      .replace(/\{winner\}/g, playerAhead)
      .replace(/\{loser\}/g, cm.deltaP0 > 0 ? this.playerName1 : this.playerName0)
      .replace(/\{delta\}/g, Math.abs(cm.deltaP0 * 100).toFixed(0));

    return {
      turn: cm.turn,
      type: 'critical',
      headline,
      detail,
      playerFocus: cm.deltaP0 > 0 ? 0 : 1,
    };
  }

  private getCommentaryTemplates(type: CriticalMoment['type']): string[] {
    switch (type) {
      case 'capture':
        return [
          '{winner} seizes a core! A {delta}% swing in win probability.',
          'Core captured! {winner} claims vital territory. The odds shift {delta}%',
          'Strategic masterstroke from {winner} — a core falls, shifting momentum by {delta}%.',
        ];
      case 'kill':
        return [
          'Carnage on the grid! {winner} eliminates multiple units. {delta}% swing.',
          '{loser} suffers heavy losses as {winner} strikes decisively. {delta}% probability shift.',
        ];
      case 'energy':
        return [
          '{winner} secures crucial energy resources. {delta}% shift in their favor.',
          'Resource advantage builds for {winner}. The tide turns {delta}%.',
        ];
      default:
        return [
          'A pivotal moment! {winner} {delta}% swing in win probability.',
          'The momentum shifts — {winner} pulls ahead with a {delta}% advantage.',
        ];
    }
  }

  private generateHeadline(cm: CriticalMoment): string {
    switch (cm.type) {
      case 'capture': return 'Core Captured!';
      case 'kill': return 'Combat Clash!';
      case 'energy': return 'Energy Secured';
      default: return 'Momentum Shift';
    }
  }

  private generateMilestoneCommentaries(sparkline: WinProbPoint[]): CommentarySegment[] {
    const segments: CommentarySegment[] = [];
    const totalTurns = this.replay.turns.length;
    const quarters = [0.25, 0.5, 0.75];

    for (const q of quarters) {
      const turn = Math.floor(totalTurns * q);
      if (turn < 10) continue;

      const pt = sparkline.find(p => p.turn >= turn) ?? sparkline[sparkline.length - 1];
      if (!pt) continue;

      const leader = pt.p0WinProb > pt.p1WinProb ? this.playerName0 : this.playerName1;
      const prob = Math.max(pt.p0WinProb, pt.p1WinProb);

      let status: string;
      if (prob > 0.75) {
        status = `${leader} holds a commanding lead (${(prob * 100).toFixed(0)}% win probability)`;
      } else if (prob > 0.55) {
        status = `${leader} has a slight edge (${(prob * 100).toFixed(0)}% win probability)`;
      } else {
        status = `The match remains deadlocked at 50-50`;
      }

      segments.push({
        turn,
        type: 'milestone',
        headline: `Turn ${turn}`,
        detail: status,
      });
    }

    return segments;
  }

  private generateClosingCommentary(): CommentarySegment {
    const result = this.replay.result;
    const winner = result.winner >= 0 ? (result.winner === 0 ? this.playerName0 : this.playerName1) : null;

    const closings = winner
      ? [
          `${winner} claims victory by ${result.reason}!`,
          `The final blow lands — ${winner} wins by ${result.reason}.`,
          `${result.reason} ends it! ${winner} stands triumphant.`,
        ]
      : [
          `The match ends in a draw! Neither bot could claim dominance.`,
          `A stalemate! The grid remains contested as time runs out.`,
        ];

    return {
      turn: this.replay.turns.length - 1,
      type: 'closing',
      headline: 'Match Complete',
      detail: closings[Math.floor(Math.random() * closings.length)],
    };
  }

  private generateMatchSummary(sparkline: WinProbPoint[], moments: CriticalMoment[]): string {
    const result = this.replay.result;
    const winner = result.winner >= 0 ? (result.winner === 0 ? this.playerName0 : this.playerName1) : 'No one';
    const turns = result.turns;
    const criticalCount = moments.length;

    const leadChanges = this.countLeadChanges(sparkline);
    const biggestSwing = moments.length > 0
      ? Math.max(...moments.map(m => Math.abs(m.deltaP0)))
      : 0;

    let narrative: string;
    if (criticalCount === 0) {
      narrative = `A methodical ${turns}-turn match with ${winner} winning by ${result.reason}. No major momentum swings.`;
    } else if (leadChanges > 3) {
      narrative = `An action-packed ${turns}-turn battle! ${leadChanges} lead changes, ${criticalCount} critical moments, and ${winner} ultimately prevails by ${result.reason}.`;
    } else if (biggestSwing > 0.3) {
      narrative = `A match defined by a ${(biggestSwing * 100).toFixed(0)}% swing! ${winner} claims victory in ${turns} turns by ${result.reason}.`;
    } else {
      narrative = `${turns} turns of strategic play. ${winner} wins by ${result.reason} with ${criticalCount} pivotal moments.`;
    }

    return narrative;
  }

  private countLeadChanges(sparkline: WinProbPoint[]): number {
    let changes = 0;
    let prevLeader: number | null = null;

    for (const pt of sparkline) {
      const leader = pt.p0WinProb > pt.p1WinProb ? 0 : 1;
      if (prevLeader !== null && leader !== prevLeader) {
        changes++;
      }
      prevLeader = leader;
    }

    return changes;
  }

  private generateWinnerNarrative(): string {
    const result = this.replay.result;
    const winner = result.winner >= 0 ? (result.winner === 0 ? this.playerName0 : this.playerName1) : null;

    if (!winner) {
      return `An evenly matched contest ends in a draw. Both ${this.playerName0} and ${this.playerName1} proved worthy opponents.`;
    }

    const loser = winner === this.playerName0 ? this.playerName1 : this.playerName0;
    const scoreDiff = Math.abs(result.scores[0] - result.scores[1]);

    if (result.reason === 'elimination') {
      return `${winner} achieves total dominance, eliminating ${loser}'s forces from the grid!`;
    } else if (result.reason === 'dominance') {
      return `${winner} establishes overwhelming control, forcing the match to end by dominance.`;
    } else if (scoreDiff > 20) {
      return `${winner} crushes the competition with a commanding ${scoreDiff}-point lead.`;
    } else if (scoreDiff > 5) {
      return `${winner} secures a solid victory with a ${scoreDiff}-point advantage.`;
    } else {
      return `${winner} edges out ${loser} in a nail-biting finish!`;
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Commentary Renderer
// ─────────────────────────────────────────────────────────────────────────────

export function renderCommentaryPanel(commentary: ReplayCommentary): string {
  return `
    <div class="commentary-panel">
      <div class="commentary-summary">${escapeHtml(commentary.summary)}</div>
      <div class="commentary-timeline">
        ${commentary.segments.map(s => `
          <div class="commentary-segment type-${s.type}" data-turn="${s.turn}">
            <div class="segment-turn">Turn ${s.turn}</div>
            <div class="segment-content">
              <div class="segment-headline">${escapeHtml(s.headline)}</div>
              <div class="segment-detail">${escapeHtml(s.detail)}</div>
            </div>
          </div>
        `).join('')}
      </div>
    </div>
  `;
}

export function renderHighlightMarkers(
  highlights: MatchHighlight[],
  totalTurns: number,
): string {
  return highlights.map(h => {
    const pct = (h.turn / totalTurns) * 100;
    const color = h.importance === 'high' ? '#ef4444' : h.importance === 'medium' ? '#f59e0b' : '#22c55e';
    return `<div class="highlight-marker" style="left:${pct.toFixed(1)}%;background:${color}" title="${escapeHtml(h.description)}"></div>`;
  }).join('');
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// ─────────────────────────────────────────────────────────────────────────────
// Quick Commentary (No Monte Carlo - fast)
// ─────────────────────────────────────────────────────────────────────────────

export function quickCommentary(replay: Replay): ReplayCommentary {
  const segments: CommentarySegment[] = [];
  const highlights: MatchHighlight[] = [];
  const playerName0 = replay.players[0]?.name ?? 'Player 0';
  const playerName1 = replay.players[1]?.name ?? 'Player 1';

  // Opening
  segments.push({
    turn: 0,
    type: 'opening',
    headline: 'Match Start',
    detail: `${playerName0} and ${playerName1} begin their battle.`,
  });

  // Scan for events
  for (const turn of replay.turns) {
    if (!turn.events) continue;

    for (const event of turn.events) {
      if (event.type === 'core_captured') {
        const details = event.details as { new_owner?: number };
        const capturer = details?.new_owner === 0 ? playerName0 : playerName1;
        segments.push({
          turn: turn.turn,
          type: 'critical',
          headline: 'Core Captured!',
          detail: `${capturer} claims an enemy core.`,
          playerFocus: details?.new_owner,
        });
        highlights.push({
          turn: turn.turn,
          description: 'Core captured',
          importance: 'high',
        });
      }

      // Check for mass kills
      const deaths = turn.events.filter(e => e.type === 'bot_died' || e.type === 'combat_death').length;
      if (deaths >= 3) {
        segments.push({
          turn: turn.turn,
          type: 'critical',
          headline: 'Major Combat!',
          detail: `${deaths} bots eliminated in a single turn.`,
        });
        highlights.push({
          turn: turn.turn,
          description: `${deaths} bots killed`,
          importance: 'medium',
        });
      }
    }
  }

  // Closing
  const winner = replay.result.winner >= 0
    ? (replay.result.winner === 0 ? playerName0 : playerName1)
    : null;

  segments.push({
    turn: replay.turns.length - 1,
    type: 'closing',
    headline: 'Match Complete',
    detail: winner
      ? `${winner} wins by ${replay.result.reason}!`
      : 'The match ends in a draw.',
  });

  return {
    matchId: replay.match_id,
    summary: `${replay.result.turns} turns. ${winner ? winner + ' wins by ' + replay.result.reason : 'Draw'}.`,
    segments: segments.sort((a, b) => a.turn - b.turn),
    highlights,
    winnerNarrative: winner
      ? `${winner} emerges victorious!`
      : 'A hard-fought draw.',
  };
}
