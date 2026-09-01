// Event Ribbon Component
// Horizontal event timeline displaying SignificantEvents proportionally by turn
// Positioned below the replay canvas as a navigation aid

import type { SignificantEvent } from '../extract-significant-events';

// ─────────────────────────────────────────────────────────────────────────────
// Event Ribbon Component
// ─────────────────────────────────────────────────────────────────────────────

export interface EventRibbonOptions {
  container: HTMLElement;  // Parent element to render into
  onEventClick?: (event: SignificantEvent) => void;  // Optional click handler
  getTurn?: () => number;  // Optional: get current turn for highlighting
}

export class EventRibbon {
  private container: HTMLElement;
  private ribbonEl!: HTMLDivElement;
  private markersContainer!: HTMLDivElement;
  private onEventClick?: (event: SignificantEvent) => void;
  private getTurn?: () => number;
  private events: SignificantEvent[] = [];
  private totalTurns: number = 0;

  constructor(options: EventRibbonOptions) {
    this.container = options.container;
    this.onEventClick = options.onEventClick;
    this.getTurn = options.getTurn;
    this.buildDOM();
  }

  /**
   * Update the events displayed in the ribbon.
   * @param events - Array of significant events to display
   * @param totalTurns - Total number of turns in the replay (for proportional positioning)
   */
  public setEvents(events: SignificantEvent[], totalTurns: number): void {
    this.events = events;
    this.totalTurns = totalTurns;
    this.renderMarkers();
  }

  /**
   * Clear all events from the ribbon.
   */
  public clear(): void {
    this.events = [];
    this.totalTurns = 0;
    this.renderMarkers();
  }

  /**
   * Update the current turn highlight (if getTurn was provided).
   * Call this when the replay turn changes to update the cursor position.
   */
  public updateTurnHighlight(): void {
    if (!this.getTurn) return;

    const currentTurn = this.getTurn();
    this.updateCursorPosition(currentTurn);
  }

  /**
   * Destroy the component and remove from DOM.
   */
  public destroy(): void {
    if (this.ribbonEl && this.ribbonEl.parentNode) {
      this.ribbonEl.parentNode.removeChild(this.ribbonEl);
    }
  }

  // ── Private Methods ─────────────────────────────────────────────────────────────

  private buildDOM(): void {
    // Main ribbon container
    this.ribbonEl = document.createElement('div');
    this.ribbonEl.className = 'event-ribbon';
    this.ribbonEl.innerHTML = `
      <div class="event-ribbon-track"></div>
      <div class="event-ribbon-markers"></div>
      <div class="event-ribbon-cursor"></div>
    `;
    this.container.appendChild(this.ribbonEl);

    // Cache references to key elements
    this.markersContainer = this.ribbonEl.querySelector('.event-ribbon-markers') as HTMLDivElement;
  }

  private renderMarkers(): void {
    // Clear existing markers
    this.markersContainer.innerHTML = '';

    // Handle edge cases
    if (this.events.length === 0 || this.totalTurns <= 0) {
      this.ribbonEl.classList.add('event-ribbon-empty');
      return;
    }

    this.ribbonEl.classList.remove('event-ribbon-empty');

    // Render a marker for each event
    for (const event of this.events) {
      const marker = this.createEventMarker(event);
      this.markersContainer.appendChild(marker);
    }

    // Update cursor position if we have a getTurn function
    if (this.getTurn) {
      this.updateTurnHighlight();
    }
  }

  private createEventMarker(event: SignificantEvent): HTMLDivElement {
    const marker = document.createElement('div');
    marker.className = 'event-marker';
    marker.dataset.turn = event.turn.toString();
    marker.dataset.eventType = event.type;

    // Calculate proportional position: iconLeft = (eventTurn / totalTurns) * 100%
    const positionPercent = this.calculatePosition(event.turn);
    marker.style.left = `${positionPercent}%`;

    // Placeholder marker (no icon yet - that's child 3's work)
    // For now, render a simple circular marker
    marker.innerHTML = `
      <div class="event-marker-dot" title="${this.escapeHtml(event.description)}"></div>
    `;

    // Optional click handler
    if (this.onEventClick) {
      marker.addEventListener('click', (e) => {
        e.stopPropagation();
        this.onEventClick!(event);
      });
      marker.style.cursor = 'pointer';
      marker.classList.add('event-marker-clickable');
    }

    return marker;
  }

  private calculatePosition(turn: number): number {
    // Handle edge cases
    if (this.totalTurns <= 0) {
      return 0; // No turns, position at start
    }

    if (this.totalTurns === 1) {
      return 50; // Single turn, center the marker
    }

    // Standard proportional positioning
    // Clamp to 0-100 range to handle edge cases like turn 0 or turn > totalTurns
    const rawPosition = (turn / this.totalTurns) * 100;
    return Math.max(0, Math.min(100, rawPosition));
  }

  private updateCursorPosition(currentTurn: number): void {
    const cursor = this.ribbonEl.querySelector('.event-ribbon-cursor') as HTMLElement;
    if (!cursor) return;

    // Position cursor proportionally
    const positionPercent = this.calculatePosition(currentTurn);
    cursor.style.left = `${positionPercent}%`;
  }

  private escapeHtml(text: string): string {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// CSS Styles
// ─────────────────────────────────────────────────────────────────────────────

export const EVENT_RIBBON_STYLES = `
/* ─── Event Ribbon (§16.20) ───────────────────────────────────────────────────── */

.event-ribbon {
  position: relative;
  width: 100%;
  height: 48px;
  background: var(--bg-secondary, #0f172a);
  border-top: 1px solid var(--border, #1e293b);
  overflow: hidden;
  display: flex;
  align-items: center;
}

.event-ribbon-track {
  position: absolute;
  top: 50%;
  left: 0;
  right: 0;
  height: 2px;
  background: var(--border, #1e293b);
  transform: translateY(-50%);
}

.event-ribbon-markers {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 100%;
  pointer-events: none;
}

.event-marker {
  position: absolute;
  top: 50%;
  transform: translate(-50%, -50%);
  pointer-events: auto;
  transition: transform 0.15s ease;
}

.event-marker-clickable:hover {
  transform: translate(-50%, -50%) scale(1.3);
  z-index: 10;
}

.event-marker-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--accent, #3b82f6);
  border: 2px solid var(--bg-secondary, #0f172a);
  box-shadow: 0 0 4px rgba(59, 130, 246, 0.5);
  transition: background-color 0.15s ease, box-shadow 0.15s ease;
}

.event-marker-clickable:hover .event-marker-dot {
  background: var(--accent-hover, #60a5fa);
  box-shadow: 0 0 8px rgba(59, 130, 246, 0.8);
}

/* Event type variations (for future icon work) */
.event-marker[data-event-type="combat"] .event-marker-dot {
  background: #ef4444; /* Red */
}

.event-marker[data-event-type="core_capture"] .event-marker-dot {
  background: #f59e0b; /* Amber */
}

.event-marker[data-event-type="energy_milestone"] .event-marker-dot {
  background: #22c55e; /* Green */
}

.event-marker[data-event-type="mass_death"] .event-marker-dot {
  background: #8b5cf6; /* Purple */
}

.event-marker[data-event-type="momentum_shift"] .event-marker-dot {
  background: #06b6d4; /* Cyan */
}

.event-marker[data-event-type="critical_moment"] .event-marker-dot {
  background: #ec4899; /* Pink */
}

.event-marker[data-event-type="spawn_wave"] .event-marker-dot {
  background: #f97316; /* Orange */
}

/* Current turn cursor */
.event-ribbon-cursor {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--text-primary, #e2e8f0);
  transform: translateX(-50%);
  pointer-events: none;
  z-index: 5;
  transition: left 0.1s ease-out;
}

.event-ribbon-cursor::before {
  content: '';
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 0;
  height: 0;
  border-left: 6px solid transparent;
  border-right: 6px solid transparent;
  border-top: 8px solid var(--text-primary, #e2e8f0);
}

/* Empty state */
.event-ribbon-empty {
  opacity: 0.5;
}

.event-ribbon-empty::after {
  content: 'No events to display';
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: var(--text-secondary, #64748b);
  font-size: 0.75rem;
  font-family: monospace;
}

/* Reduced motion */
@media (prefers-reduced-motion: reduce) {
  .event-marker,
  .event-marker-dot,
  .event-ribbon-cursor {
    transition: none;
  }
}
`;
