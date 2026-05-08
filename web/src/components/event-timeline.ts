// Event Timeline Ribbon - Visual event strip with click-to-jump
// §14.8 Match Event Timeline - client-side event extraction with computed events
import type { GameEvent } from '../types';
import type { Annotation } from './annotation';

export interface TimelineEvent {
  turn: number;
  type: string;
  icon: string;
  color: string;
  description: string;
}

// Map event types to icons and colors (matching plan §14.8)
const EVENT_CONFIG: Record<string, { icon: string; color: string; label: string }> = {
  // Raw game events
  bot_spawned: { icon: '🐣', color: '#22c55e', label: 'Spawn' },
  bot_died: { icon: '💀', color: '#ef4444', label: 'Death' },
  energy_collected: { icon: '💎', color: '#fbbf24', label: 'Energy' },
  core_captured: { icon: '🏰', color: '#3b82f6', label: 'Core' },
  core_destroyed: { icon: '🏰', color: '#6b7280', label: 'Core' },
  // Computed events (§14.8)
  combat: { icon: '⚔️', color: '#f97316', label: 'Combat' },
  mass_death: { icon: '💀', color: '#dc2626', label: 'Mass Death' },
  spawn_wave: { icon: '🐣', color: '#22c55e', label: 'Spawn Wave' },
  momentum_shift: { icon: '📈', color: '#8b5cf6', label: 'Momentum' },
  critical_moment: { icon: '🌟', color: '#fbbf24', label: 'Critical' },
};

export class EventTimeline {
  private container: HTMLElement;
  private events: TimelineEvent[] = [];
  private currentTurn: number = 0;
  private totalTurns: number = 0;
  private onTurnClick?: (turn: number) => void;
  private annotations: Annotation[] = [];

  constructor(container: HTMLElement, options?: { onTurnClick?: (turn: number) => void }) {
    this.container = container;
    this.onTurnClick = options?.onTurnClick;
    this.render();
  }

  // Extract events from replay turns with client-side computed events (§14.8)
  setEvents(
    turns: { turn: number; events?: GameEvent[]; energy_collected?: any }[],
    winProb?: number[][]
  ): void {
    this.events = [];
    this.totalTurns = turns.length;

    for (const turnData of turns) {
      const turn = turnData.turn;
      const rawEvents = turnData.events ?? [];

      // Count deaths this turn
      const deaths = rawEvents.filter((e: GameEvent) => e.type === 'bot_died').length;

      // Count spawns this turn
      const spawns = rawEvents.filter((e: GameEvent) => e.type === 'bot_spawned').length;

      // Count energy collected per player
      const energyByPlayer = new Map<number, number>();
      for (const e of rawEvents) {
        if (e.type === 'energy_collected') {
          const owner = (e.details as any)?.owner ?? 0;
          energyByPlayer.set(owner, (energyByPlayer.get(owner) ?? 0) + 1);
        }
      }
      const maxEnergy = Math.max(0, ...energyByPlayer.values());

      // Add raw game events
      for (const event of rawEvents) {
        const config = EVENT_CONFIG[event.type] || { icon: '•', color: '#94a3b8', label: 'Event' };
        this.events.push({
          turn,
          type: event.type,
          icon: config.icon,
          color: config.color,
          description: `${config.label}: turn ${turn}`,
        });
      }

      // Add computed events (§14.8)

      // Mass death: 5+ bots died this turn
      if (deaths >= 5) {
        const config = EVENT_CONFIG.mass_death;
        this.events.push({
          turn,
          type: 'mass_death',
          icon: config.icon,
          color: config.color,
          description: `Mass Death: ${deaths} bots eliminated`,
        });
      }

      // Combat: 2+ bots died (but not mass death)
      if (deaths >= 2 && deaths < 5) {
        const config = EVENT_CONFIG.combat;
        this.events.push({
          turn,
          type: 'combat',
          icon: config.icon,
          color: config.color,
          description: `Combat: ${deaths} bots killed`,
        });
      }

      // Spawn wave: 3+ bots spawned this turn
      if (spawns >= 3) {
        const config = EVENT_CONFIG.spawn_wave;
        this.events.push({
          turn,
          type: 'spawn_wave',
          icon: config.icon,
          color: config.color,
          description: `Spawn Wave: ${spawns} new bots`,
        });
      }

      // Energy milestone: player collected 3+ energy
      if (maxEnergy >= 3) {
        const config = EVENT_CONFIG.energy_collected;
        this.events.push({
          turn,
          type: 'energy_milestone',
          icon: config.icon,
          color: config.color,
          description: `Energy: ${maxEnergy} collected`,
        });
      }

      // Momentum shift: win probability crossed 50%
      if (winProb && winProb[turn] && winProb[turn - 1]) {
        const prevProb = winProb[turn - 1];
        const currProb = winProb[turn];
        // Check if any player crossed 50%
        for (let p = 0; p < currProb.length; p++) {
          const prevWasAbove = prevProb[p] > 0.5;
          const currIsAbove = currProb[p] > 0.5;
          if (prevWasAbove !== currIsAbove) {
            const config = EVENT_CONFIG.momentum_shift;
            this.events.push({
              turn,
              type: 'momentum_shift',
              icon: config.icon,
              color: config.color,
              description: `Momentum Shift: player ${p}`,
            });
            break; // Only add one marker per turn
          }
        }
      }

      // Critical moment: win probability shifted >15%
      if (winProb && winProb[turn] && winProb[turn - 1]) {
        const prevProb = winProb[turn - 1];
        const currProb = winProb[turn];
        for (let p = 0; p < currProb.length; p++) {
          const delta = Math.abs(currProb[p] - prevProb[p]);
          if (delta > 0.15) {
            const config = EVENT_CONFIG.critical_moment;
            this.events.push({
              turn,
              type: 'critical_moment',
              icon: config.icon,
              color: config.color,
              description: `Critical: ${(delta * 100).toFixed(0)}% shift`,
            });
            break; // Only add one marker per turn
          }
        }
      }
    }

    this.render();
  }

  setCurrentTurn(turn: number): void {
    this.currentTurn = turn;
    this.updateHighlight();
  }

  setAnnotations(annotations: Annotation[]): void {
    this.annotations = annotations;
    this.render();
  }

  private render(): void {
    const hasEvents = this.events.length > 0;
    const hasAnnotations = this.annotations.length > 0;

    if (!hasEvents && !hasAnnotations) {
      this.container.innerHTML = '<div class="timeline-empty">No events</div>';
      return;
    }

    const eventMarkers = this.events.map((e, idx) => {
      const leftPercent = (e.turn / Math.max(1, this.totalTurns - 1)) * 100;
      return `
        <div class="timeline-event"
             data-index="${idx}"
             data-turn="${e.turn}"
             data-tooltip="${e.description}"
             style="left: ${leftPercent}%; color: ${e.color}"
             title="Turn ${e.turn}: ${e.description}">
          ${e.icon}
        </div>
      `;
    }).join('');

    // Annotation badges on timeline
    const annotationIcons: Record<string, string> = {
      insight: '\u{1F4A1}',
      mistake: '⚠️',
      idea: '\u{1F9EA}',
      highlight: '⭐',
    };
    const annotationColors: Record<string, string> = {
      insight: '#3b82f6',
      mistake: '#ef4444',
      idea: '#22c55e',
      highlight: '#fbbf24',
    };
    const annotationMarkers = hasAnnotations ? this.buildAnnotationMarkers(annotationIcons, annotationColors) : '';

    this.container.innerHTML = `
      <div class="timeline-track">
        <div class="timeline-progress" id="timeline-progress"></div>
        ${eventMarkers}
        ${annotationMarkers}
      </div>
      <div class="timeline-turn-label">
        <span id="timeline-current">0</span> / <span id="timeline-total">${this.totalTurns}</span>
      </div>
    `;

    // Wire click handlers (both events and annotation markers)
    this.container.querySelectorAll('.timeline-event').forEach(el => {
      el.addEventListener('click', (e) => {
        const turn = parseInt((e.currentTarget as HTMLElement).dataset.turn || '0', 10);
        if (this.onTurnClick) {
          this.onTurnClick(turn);
        }
      });
    });

    // Click on track to seek
    const track = this.container.querySelector('.timeline-track') as HTMLElement | null;
    if (track) {
      track.addEventListener('click', (e: MouseEvent) => {
        const rect = track.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const percent = x / rect.width;
        const turn = Math.floor(percent * this.totalTurns);
        if (this.onTurnClick) {
          this.onTurnClick(turn);
        }
      });
    }

    this.updateHighlight();
  }

  private buildAnnotationMarkers(
    icons: Record<string, string>,
    colors: Record<string, string>,
  ): string {
    // Group annotations by turn
    const grouped = new Map<number, Annotation[]>();
    for (const ann of this.annotations) {
      const list = grouped.get(ann.turn) ?? [];
      list.push(ann);
      grouped.set(ann.turn, list);
    }

    return [...grouped.entries()].map(([turn, anns]) => {
      const leftPercent = (turn / Math.max(1, this.totalTurns - 1)) * 100;
      const primary = anns[0].type;
      const icon = icons[primary] ?? '\u{1F4CC}';
      const color = colors[primary] ?? '#94a3b8';
      const count = anns.length > 1 ? `<span class="ann-marker-count">${anns.length}</span>` : '';
      return `<div class="timeline-event timeline-annotation"
                   data-turn="${turn}"
                   style="left: ${leftPercent}%; color: ${color}"
                   title="Turn ${turn}: ${anns.length} annotation${anns.length > 1 ? 's' : ''}">
          ${icon}${count}
        </div>`;
    }).join('');
  }

  private updateHighlight(): void {
    const progress = this.container.querySelector('#timeline-progress') as HTMLElement;
    const currentLabel = this.container.querySelector('#timeline-current') as HTMLElement;

    if (progress) {
      const percent = (this.currentTurn / Math.max(1, this.totalTurns - 1)) * 100;
      progress.style.width = `${percent}%`;
    }

    if (currentLabel) {
      currentLabel.textContent = String(this.currentTurn);
    }

    // Highlight events and annotations at current turn
    this.container.querySelectorAll('.timeline-event, .timeline-annotation').forEach(el => {
      const turn = parseInt((el as HTMLElement).dataset.turn || '0', 10);
      el.classList.toggle('active', turn === this.currentTurn);
    });
  }
}

// CSS styles for timeline (inject into document)
export const EVENT_TIMELINE_STYLES = `
  .event-timeline-container {
    background-color: var(--bg-secondary, #1e293b);
    border-radius: 8px;
    padding: 12px;
    margin-bottom: 16px;
  }

  .timeline-track {
    position: relative;
    height: 32px;
    background-color: var(--bg-tertiary, #334155);
    border-radius: 4px;
    cursor: pointer;
    overflow: visible;
  }

  .timeline-progress {
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    background-color: var(--accent, #3b82f6);
    opacity: 0.3;
    border-radius: 4px;
    transition: width 0.05s linear;
  }

  .timeline-event {
    position: absolute;
    top: 50%;
    transform: translate(-50%, -50%);
    font-size: 16px;
    cursor: pointer;
    transition: transform 0.15s, text-shadow 0.15s;
    z-index: 1;
    text-shadow: 0 0 2px rgba(0,0,0,0.8);
  }

  .timeline-event:hover {
    transform: translate(-50%, -50%) scale(1.4);
    text-shadow: 0 0 4px currentColor;
    z-index: 10;
  }

  .timeline-event:hover::after {
    content: attr(data-tooltip);
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    background-color: rgba(0, 0, 0, 0.9);
    color: #fff;
    padding: 4px 8px;
    border-radius: 4px;
    font-size: 11px;
    white-space: nowrap;
    pointer-events: none;
    margin-bottom: 4px;
  }

  .timeline-event.active {
    transform: translate(-50%, -50%) scale(1.5);
    text-shadow: 0 0 6px currentColor;
    z-index: 5;
  }

  .timeline-turn-label {
    text-align: center;
    font-size: 12px;
    color: var(--text-muted, #94a3b8);
    margin-top: 6px;
  }

  .timeline-empty {
    text-align: center;
    color: var(--text-muted, #94a3b8);
    font-size: 14px;
    padding: 8px;
  }

  .timeline-annotation {
    font-size: 10px;
  }

  .ann-marker-count {
    position: absolute;
    top: -8px;
    font-size: 8px;
    font-weight: 600;
    color: var(--text-muted, #94a3b8);
  }
`;
