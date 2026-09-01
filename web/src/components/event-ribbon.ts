// Event Ribbon - Horizontal event strip with proportional positioning
// Part of replay event ribbon UI (child 2 of multi-part implementation)

import type { SignificantEvent } from '../extract-significant-events';

export interface EventRibbonOptions {
  events: SignificantEvent[];
  totalTurns: number;
  onEventClick?: (event: SignificantEvent) => void;
  onTurnClick?: (turn: number) => void;
}

export class EventRibbon {
  private container: HTMLElement;
  private events: SignificantEvent[] = [];
  private totalTurns: number = 0;
  private onEventClick?: (event: SignificantEvent) => void;
  private onTurnClick?: (turn: number) => void;
  private currentTurn: number = 0;

  constructor(container: HTMLElement, options?: EventRibbonOptions) {
    this.container = container;
    this.onEventClick = options?.onEventClick;
    this.onTurnClick = options?.onTurnClick;

    if (options?.events) {
      this.setEvents(options.events, options.totalTurns);
    } else {
      this.render();
    }
  }

  setEvents(events: SignificantEvent[], totalTurns: number): void {
    this.events = events;
    this.totalTurns = totalTurns;
    this.render();
  }

  setCurrentTurn(turn: number): void {
    this.currentTurn = turn;
    this.updateHighlight();
  }

  private render(): void {
    // Handle edge cases
    if (this.totalTurns <= 0) {
      this.container.innerHTML = '<div class="ribbon-empty">No turns</div>';
      return;
    }

    // Create event markers with proportional positioning
    const eventMarkers = this.events.map((event, idx) => {
      // Calculate proportional left position
      const leftPercent = this.calculateLeftPosition(event.turn);

      // Create marker element
      return `
        <div class="ribbon-marker"
             data-index="${idx}"
             data-turn="${event.turn}"
             data-type="${event.type}"
             style="left: ${leftPercent}%"
             title="Turn ${event.turn}: ${event.description}">
          <div class="ribbon-marker-icon"></div>
          <div class="ribbon-marker-tooltip">${event.description}</div>
        </div>
      `;
    }).join('');

    // Create ribbon structure
    this.container.innerHTML = `
      <div class="event-ribbon">
        <div class="ribbon-track">
          <div class="ribbon-progress" id="ribbon-progress"></div>
          ${eventMarkers}
        </div>
        <div class="ribbon-turn-label">
          <span id="ribbon-current">0</span> / <span id="ribbon-total">${this.totalTurns}</span>
        </div>
      </div>
    `;

    // Wire up click handlers
    this.attachEventHandlers();
    this.updateHighlight();
  }

  private calculateLeftPosition(turn: number): number {
    // Handle edge cases
    if (this.totalTurns <= 0) return 0;
    if (this.totalTurns === 1) return 50; // Center position for single-turn game

    // Standard proportional positioning: (turn / totalTurns) * 100
    // Events on turn 0 should be at the far left (0%)
    return (turn / this.totalTurns) * 100;
  }

  private attachEventHandlers(): void {
    // Event marker clicks
    this.container.querySelectorAll('.ribbon-marker').forEach(marker => {
      marker.addEventListener('click', (e) => {
        e.stopPropagation();
        const idx = parseInt((marker as HTMLElement).dataset.index || '0', 10);
        const turn = parseInt((marker as HTMLElement).dataset.turn || '0', 10);

        if (this.events[idx] && this.onEventClick) {
          this.onEventClick(this.events[idx]);
        }

        if (this.onTurnClick) {
          this.onTurnClick(turn);
        }
      });

      // Hover effects
      marker.addEventListener('mouseenter', () => {
        marker.classList.add('ribbon-marker-hover');
      });

      marker.addEventListener('mouseleave', () => {
        marker.classList.remove('ribbon-marker-hover');
      });
    });

    // Track click for seeking to any turn
    const track = this.container.querySelector('.ribbon-track');
    if (track) {
      track.addEventListener('click', (e: Event) => {
        const rect = track.getBoundingClientRect();
        const x = (e as any).clientX - rect.left;
        const percent = x / rect.width;
        const turn = Math.floor(percent * this.totalTurns);

        if (this.onTurnClick) {
          this.onTurnClick(Math.max(0, Math.min(turn, this.totalTurns - 1)));
        }
      });
    }
  }

  private updateHighlight(): void {
    const progress = this.container.querySelector('#ribbon-progress') as HTMLElement;
    const currentLabel = this.container.querySelector('#ribbon-current') as HTMLElement;

    // Update progress indicator
    if (progress) {
      const percent = this.calculateLeftPosition(this.currentTurn);
      progress.style.width = `${percent}%`;
    }

    // Update turn label
    if (currentLabel) {
      currentLabel.textContent = String(this.currentTurn);
    }

    // Highlight active event markers
    this.container.querySelectorAll('.ribbon-marker').forEach(marker => {
      const turn = parseInt((marker as HTMLElement).dataset.turn || '0', 10);
      marker.classList.toggle('ribbon-marker-active', turn === this.currentTurn);
    });
  }
}

// CSS styles for event ribbon (inject into document)
export const EVENT_RIBBON_STYLES = `
  .event-ribbon {
    width: 100%;
    background-color: var(--bg-secondary, #1e293b);
    border-radius: 8px;
    padding: 12px;
    box-sizing: border-box;
  }

  .ribbon-track {
    position: relative;
    height: 32px;
    background-color: var(--bg-tertiary, #334155);
    border-radius: 4px;
    cursor: pointer;
    overflow: hidden;
    margin-bottom: 8px;
  }

  .ribbon-progress {
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    background-color: var(--accent, #3b82f6);
    opacity: 0.2;
    border-radius: 4px;
    transition: width 0.1s ease-out;
    pointer-events: none;
  }

  .ribbon-marker {
    position: absolute;
    top: 50%;
    transform: translate(-50%, -50%);
    cursor: pointer;
    z-index: 2;
    transition: transform 0.15s ease-out;
  }

  .ribbon-marker-icon {
    width: 16px;
    height: 16px;
    background-color: var(--accent, #3b82f6);
    border-radius: 50%;
    border: 2px solid var(--bg-secondary, #1e293b);
    box-shadow: 0 0 4px rgba(0, 0, 0, 0.3);
    transition: all 0.15s ease-out;
  }

  .ribbon-marker:hover .ribbon-marker-icon {
    transform: scale(1.3);
    background-color: var(--accent-hover, #60a5fa);
    box-shadow: 0 0 8px rgba(59, 130, 246, 0.5);
  }

  .ribbon-marker-active .ribbon-marker-icon {
    transform: scale(1.5);
    background-color: var(--accent-active, #93c5fd);
    box-shadow: 0 0 12px rgba(59, 130, 246, 0.7);
    border-color: var(--accent, #3b82f6);
  }

  .ribbon-marker-tooltip {
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    background-color: rgba(0, 0, 0, 0.9);
    color: #fff;
    padding: 6px 10px;
    border-radius: 4px;
    font-size: 12px;
    white-space: nowrap;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.15s ease-out;
    margin-bottom: 4px;
    min-width: 100px;
    text-align: center;
  }

  .ribbon-marker:hover .ribbon-marker-tooltip,
  .ribbon-marker-active .ribbon-marker-tooltip {
    opacity: 1;
  }

  .ribbon-turn-label {
    text-align: center;
    font-size: 12px;
    color: var(--text-muted, #94a3b8);
    margin-top: 4px;
  }

  .ribbon-empty {
    text-align: center;
    color: var(--text-muted, #94a3b8);
    font-size: 14px;
    padding: 16px;
    background-color: var(--bg-tertiary, #334155);
    border-radius: 4px;
  }

  /* Event type specific colors (can be customized) */
  .ribbon-marker[data-type="combat"] .ribbon-marker-icon {
    background-color: #f97316;
  }

  .ribbon-marker[data-type="core_capture"] .ribbon-marker-icon {
    background-color: #3b82f6;
  }

  .ribbon-marker[data-type="energy_milestone"] .ribbon-marker-icon {
    background-color: #fbbf24;
  }

  .ribbon-marker[data-type="mass_death"] .ribbon-marker-icon {
    background-color: #ef4444;
  }

  .ribbon-marker[data-type="momentum_shift"] .ribbon-marker-icon {
    background-color: #8b5cf6;
  }

  .ribbon-marker[data-type="critical_moment"] .ribbon-marker-icon {
    background-color: #ec4899;
  }

  .ribbon-marker[data-type="spawn_wave"] .ribbon-marker-icon {
    background-color: #22c55e;
  }
`;
