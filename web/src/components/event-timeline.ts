// Event Timeline Ribbon - Visual event strip with click-to-jump
import type { GameEvent } from '../types';

export interface TimelineEvent {
  turn: number;
  type: string;
  icon: string;
  color: string;
}

// Map event types to icons and colors
const EVENT_CONFIG: Record<string, { icon: string; color: string }> = {
  bot_spawned: { icon: '◆', color: '#22c55e' },
  bot_died: { icon: '✕', color: '#ef4444' },
  combat_death: { icon: '⚔', color: '#f97316' },
  collision_death: { icon: '💥', color: '#eab308' },
  energy_collected: { icon: '★', color: '#fbbf24' },
  core_captured: { icon: '◉', color: '#3b82f6' },
  core_destroyed: { icon: '⊘', color: '#6b7280' },
};

export class EventTimeline {
  private container: HTMLElement;
  private events: TimelineEvent[] = [];
  private currentTurn: number = 0;
  private totalTurns: number = 0;
  private onTurnClick?: (turn: number) => void;

  constructor(container: HTMLElement, options?: { onTurnClick?: (turn: number) => void }) {
    this.container = container;
    this.onTurnClick = options?.onTurnClick;
    this.render();
  }

  // Extract events from replay turns
  setEvents(turns: { turn: number; events?: GameEvent[] }[]): void {
    this.events = [];
    this.totalTurns = turns.length;

    for (const turnData of turns) {
      const turnEvents = turnData.events ?? [];
      for (const event of turnEvents) {
        const config = EVENT_CONFIG[event.type] || { icon: '•', color: '#94a3b8' };
        this.events.push({
          turn: turnData.turn,
          type: event.type,
          icon: config.icon,
          color: config.color,
        });
      }
    }

    this.render();
  }

  setCurrentTurn(turn: number): void {
    this.currentTurn = turn;
    this.updateHighlight();
  }

  private render(): void {
    if (this.events.length === 0) {
      this.container.innerHTML = '<div class="timeline-empty">No events</div>';
      return;
    }

    const eventMarkers = this.events.map((e, idx) => {
      const leftPercent = (e.turn / Math.max(1, this.totalTurns - 1)) * 100;
      return `
        <div class="timeline-event"
             data-index="${idx}"
             data-turn="${e.turn}"
             style="left: ${leftPercent}%; color: ${e.color}"
             title="Turn ${e.turn}: ${e.type.replace(/_/g, ' ')}">
          ${e.icon}
        </div>
      `;
    }).join('');

    this.container.innerHTML = `
      <div class="timeline-track">
        <div class="timeline-progress" id="timeline-progress"></div>
        ${eventMarkers}
      </div>
      <div class="timeline-turn-label">
        <span id="timeline-current">0</span> / <span id="timeline-total">${this.totalTurns}</span>
      </div>
    `;

    // Wire click handlers
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

    // Highlight events at current turn
    this.container.querySelectorAll('.timeline-event').forEach(el => {
      const turn = parseInt((el as HTMLElement).dataset.turn || '0', 10);
      if (turn === this.currentTurn) {
        el.classList.add('active');
      } else {
        el.classList.remove('active');
      }
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
    height: 28px;
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
    font-size: 14px;
    cursor: pointer;
    transition: transform 0.15s, text-shadow 0.15s;
    z-index: 1;
    text-shadow: 0 0 2px rgba(0,0,0,0.8);
  }

  .timeline-event:hover {
    transform: translate(-50%, -50%) scale(1.4);
    text-shadow: 0 0 4px currentColor;
  }

  .timeline-event.active {
    transform: translate(-50%, -50%) scale(1.5);
    text-shadow: 0 0 6px currentColor;
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
`;
