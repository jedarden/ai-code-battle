// Event Ribbon Component
// Horizontal event timeline displaying SignificantEvents proportionally by turn
// Positioned below the replay canvas as a navigation aid

import type { SignificantEvent, SignificantEventType } from '../extract-significant-events';

// ─────────────────────────────────────────────────────────────────────────────
// Event Ribbon Component
// ─────────────────────────────────────────────────────────────────────────────

export interface EventRibbonOptions {
  container: HTMLElement;  // Parent element to render into
  events?: SignificantEvent[];  // Optional: initial events to display
  totalTurns?: number;  // Optional: total number of turns in the replay
  onEventClick?: (event: SignificantEvent) => void;  // Optional click handler
  onTurnClick?: (turn: number) => void;  // Optional turn click handler
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
  private legendVisible: boolean = true;
  private legendEl?: HTMLElement;
  private legendCloseButton?: HTMLButtonElement;

  constructor(options: EventRibbonOptions) {
    this.container = options.container;
    this.onEventClick = options.onEventClick;
    this.getTurn = options.getTurn;

    this.buildDOM();

    // Initialize events and totalTurns if provided
    if (options.events !== undefined && options.totalTurns !== undefined) {
      this.setEvents(options.events, options.totalTurns);
    } else if (options.events !== undefined) {
      this.events = options.events;
    }
    if (options.totalTurns !== undefined) {
      this.totalTurns = options.totalTurns;
    }
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
   * Render the event type legend below the ribbon.
   * Call this after creating the ribbon to add the legend.
   */
  public renderLegend(): void {
    // Check if legend already exists
    if (this.legendEl) {
      return; // Already rendered
    }

    const legend = document.createElement('div');
    legend.className = 'event-ribbon-legend';
    this.legendEl = legend;

    // Create legend header with close button
    const header = document.createElement('div');
    header.className = 'event-legend-header';

    const headerTitle = document.createElement('span');
    headerTitle.className = 'event-legend-title';
    headerTitle.textContent = 'Event Types';

    this.legendCloseButton = document.createElement('button');
    this.legendCloseButton.className = 'event-legend-close';
    this.legendCloseButton.setAttribute('type', 'button');
    this.legendCloseButton.setAttribute('aria-label', 'Hide legend');
    this.legendCloseButton.innerHTML = '×';
    this.legendCloseButton.addEventListener('click', () => this.hideLegend());

    header.appendChild(headerTitle);
    header.appendChild(this.legendCloseButton);
    legend.appendChild(header);

    // Create legend content container
    const content = document.createElement('div');
    content.className = 'event-legend-content';

    // Get all event types and their styles
    const eventTypes: Array<{ type: SignificantEventType; label: string; icon: string }> = [
      { type: 'combat', label: 'Combat', icon: '⚔️' },
      { type: 'core_capture', label: 'Core Capture', icon: '🏰' },
      { type: 'energy_milestone', label: 'Energy', icon: '💎' },
      { type: 'mass_death', label: 'Mass Death', icon: '💀' },
      { type: 'momentum_shift', label: 'Momentum Shift', icon: '📈' },
      { type: 'critical_moment', label: 'Critical Moment', icon: '🌟' },
      { type: 'spawn_wave', label: 'Spawn Wave', icon: '🐣' },
    ];

    for (const { type, label, icon } of eventTypes) {
      const item = document.createElement('div');
      item.className = 'event-legend-item';

      const style = this.getEventStyle(type);
      item.innerHTML = `
        <span class="event-legend-icon" style="color: ${style.color}">${icon}</span>
        <span class="event-legend-label">${label}</span>
      `;

      content.appendChild(item);
    }

    legend.appendChild(content);
    this.container.appendChild(legend);
  }

  /**
   * Hide the legend.
   */
  public hideLegend(): void {
    if (this.legendEl) {
      this.legendEl.classList.add('event-ribbon-legend-hidden');
      this.legendVisible = false;
    }
  }

  /**
   * Show the legend.
   */
  public showLegend(): void {
    if (this.legendEl) {
      this.legendEl.classList.remove('event-ribbon-legend-hidden');
      this.legendVisible = true;
    }
  }

  /**
   * Toggle legend visibility.
   */
  public toggleLegend(): void {
    if (this.legendVisible) {
      this.hideLegend();
    } else {
      this.showLegend();
    }
  }

  /**
   * Destroy the component and remove from DOM.
   */
  public destroy(): void {
    if (this.ribbonEl && this.ribbonEl.parentNode) {
      this.ribbonEl.parentNode.removeChild(this.ribbonEl);
    }
    // Also remove legend if it exists
    const legend = this.container.querySelector('.event-ribbon-legend');
    if (legend && legend.parentNode) {
      legend.parentNode.removeChild(legend);
    }
  }

  // ── Private Methods ─────────────────────────────────────────────────────────────

  private buildDOM(): void {
    // Main ribbon container
    this.ribbonEl = document.createElement('div');
    this.ribbonEl.className = 'event-ribbon';

    // Create child elements programmatically to ensure they're properly constructed
    const track = document.createElement('div');
    track.className = 'event-ribbon-track';

    this.markersContainer = document.createElement('div');
    this.markersContainer.className = 'event-ribbon-markers';

    const cursor = document.createElement('div');
    cursor.className = 'event-ribbon-cursor';

    this.ribbonEl.appendChild(track);
    this.ribbonEl.appendChild(this.markersContainer);
    this.ribbonEl.appendChild(cursor);

    this.container.appendChild(this.ribbonEl);
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

    // Group events by turn for layering
    const eventsByTurn = new Map<number, number[]>();
    this.events.forEach((event, index) => {
      const turn = event.turn;
      if (!eventsByTurn.has(turn)) {
        eventsByTurn.set(turn, []);
      }
      eventsByTurn.get(turn)!.push(index);
    });

    // Render a marker for each event with proper z-index stacking and visual offset
    for (let i = 0; i < this.events.length; i++) {
      const event = this.events[i];
      const marker = this.createEventMarker(event, i);

      // Apply vertical offset for events on the same turn
      const turnEvents = eventsByTurn.get(event.turn) || [];
      const eventIndexOnTurn = turnEvents.indexOf(i);
      if (turnEvents.length > 1) {
        // Offset vertically: -6px per event to create stacking effect
        // Range: -6px (top) to +6px (bottom) for up to 3 events
        const offsetStep = 6;
        const maxOffset = (turnEvents.length - 1) * offsetStep;
        const offset = eventIndexOnTurn * offsetStep - (maxOffset / 2);
        marker.style.transform = `translate(-50%, calc(-50% + ${offset}px))`;
      }

      this.markersContainer.appendChild(marker);
    }

    // Update cursor position if we have a getTurn function
    if (this.getTurn) {
      this.updateTurnHighlight();
    }
  }

  private createEventMarker(event: SignificantEvent, index: number): HTMLDivElement {
    const marker = document.createElement('div');
    marker.className = 'event-marker';
    marker.dataset.turn = event.turn.toString();
    marker.dataset.eventType = event.type;

    // Calculate proportional position: iconLeft = (eventTurn / totalTurns) * 100%
    const positionPercent = this.calculatePosition(event.turn);
    marker.style.left = `${positionPercent}%`;

    // Calculate z-index for overlapping events on the same turn
    // Later events in the array get higher z-index to stack properly
    const zIndex = 10 + index;
    marker.style.zIndex = `${zIndex}`;

    // Get icon and color for event type
    const eventStyle = this.getEventStyle(event.type);

    // Get event type label for tooltip
    const eventTypeLabel = this.getEventTypeLabel(event.type);

    // Store the original transform for hover preservation
    const baseTransform = marker.style.transform || 'translate(-50%, -50%)';
    marker.dataset.baseTransform = baseTransform;

    // Render icon with proper styling and tooltip
    marker.innerHTML = `
      <div class="event-marker-icon ${event.type}" style="color: ${eventStyle.color}">
        ${event.emoji || eventStyle.defaultIcon}
      </div>
      <div class="event-tooltip" role="tooltip" aria-hidden="true">
        <div class="event-tooltip-header">
          <span class="event-tooltip-icon">${event.emoji || eventStyle.defaultIcon}</span>
          <span class="event-tooltip-type">${eventTypeLabel}</span>
        </div>
        <div class="event-tooltip-body">
          <div class="event-tooltip-description">${this.escapeHtml(event.description)}</div>
          <div class="event-tooltip-turn">Turn ${event.turn}</div>
        </div>
        <div class="event-tooltip-arrow"></div>
      </div>
    `;

    // Add hover event listeners for tooltip
    const icon = marker.querySelector('.event-marker-icon') as HTMLElement;
    const tooltip = marker.querySelector('.event-tooltip') as HTMLElement;

    if (icon && tooltip) {
      icon.addEventListener('mouseenter', () => {
        this.showTooltip(marker, tooltip);
      });

      icon.addEventListener('mouseleave', () => {
        this.hideTooltip(tooltip);
      });

      // Also hide tooltip on marker mouse leave to handle edge cases
      marker.addEventListener('mouseleave', () => {
        this.hideTooltip(tooltip);
      });
    }

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

  private getEventStyle(eventType: SignificantEventType): { color: string; defaultIcon: string } {
    const styles: Record<SignificantEventType, { color: string; defaultIcon: string }> = {
      combat: { color: '#ef4444', defaultIcon: '⚔️' },                    // Red/warning
      core_capture: { color: '#3b82f6', defaultIcon: '🏰' },             // Blue/primary
      energy_milestone: { color: '#06b6d4', defaultIcon: '💎' },          // Cyan/teal
      mass_death: { color: '#6b7280', defaultIcon: '💀' },               // Dark grey/death
      momentum_shift: { color: '#22c55e', defaultIcon: '📈' },             // Green/growth
      critical_moment: { color: '#eab308', defaultIcon: '🌟' },           // Yellow/gold
      spawn_wave: { color: '#a855f7', defaultIcon: '🐣' },                // Purple/special
    };
    return styles[eventType] || { color: '#6b7280', defaultIcon: '•' };
  }

  private getEventTypeLabel(eventType: SignificantEventType): string {
    const labels: Record<SignificantEventType, string> = {
      combat: 'Combat',
      core_capture: 'Core Capture',
      energy_milestone: 'Energy Milestone',
      mass_death: 'Mass Death',
      momentum_shift: 'Momentum Shift',
      critical_moment: 'Critical Moment',
      spawn_wave: 'Spawn Wave',
    };
    return labels[eventType] || 'Event';
  }

  private showTooltip(marker: HTMLElement, tooltip: HTMLElement): void {
    // Make tooltip visible and accessible
    tooltip.setAttribute('aria-hidden', 'false');
    tooltip.classList.add('event-tooltip-visible');

    // Position tooltip to avoid viewport overflow
    this.positionTooltip(marker, tooltip);
  }

  private hideTooltip(tooltip: HTMLElement): void {
    // Hide tooltip and make it inaccessible
    tooltip.setAttribute('aria-hidden', 'true');
    tooltip.classList.remove('event-tooltip-visible');
    // Reset inline styles for positioning
    tooltip.style.left = '';
    tooltip.style.transform = '';
    // Reset arrow positioning
    const arrow = tooltip.querySelector('.event-tooltip-arrow') as HTMLElement;
    if (arrow) {
      arrow.style.left = '';
    }
  }

  private positionTooltip(marker: HTMLElement, tooltip: HTMLElement): void {
    const markerRect = marker.getBoundingClientRect();
    const tooltipRect = tooltip.getBoundingClientRect();
    const viewportWidth = window.innerWidth;

    // Calculate tooltip position (above the marker by default)
    const tooltipTop = markerRect.top - tooltipRect.height - 12; // 12px gap
    let tooltipLeft = markerRect.left + (markerRect.width / 2) - (tooltipRect.width / 2);

    // Handle edge case: tooltip overflowing right edge
    if (tooltipLeft + tooltipRect.width > viewportWidth - 8) {
      tooltipLeft = viewportWidth - tooltipRect.width - 8;
    }

    // Handle edge case: tooltip overflowing left edge
    if (tooltipLeft < 8) {
      tooltipLeft = 8;
    }

    // Apply positioning
    tooltip.style.left = `${tooltipLeft}px`;
    tooltip.style.top = `${tooltipTop}px`;

    // Position arrow to point to the marker center
    const arrow = tooltip.querySelector('.event-tooltip-arrow') as HTMLElement;
    if (arrow) {
      const markerCenter = markerRect.left + (markerRect.width / 2);
      const arrowLeft = markerCenter - tooltipLeft;
      arrow.style.left = `${arrowLeft}px`;
    }
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

.event-marker-clickable {
  cursor: pointer;
}

.event-marker-clickable:hover {
  z-index: 100;
}

.event-marker-clickable:hover .event-marker-icon {
  transform: scale(1.3);
}

/* Icon styling */
.event-marker-icon {
  /* Consistent 22px icon size (middle of 20-24px range) */
  font-size: 22px;
  line-height: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  text-align: center;
  user-select: none;
  /* Responsive tap target: 44x44px minimum for touch (11px padding on each side) */
  padding: 11px;
  margin: -11px;
  border-radius: 4px;
  transition: transform 0.15s ease, filter 0.15s ease;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.3));
  /* Ensure icon stays centered within ribbon */
  box-sizing: content-box;
}

.event-marker-clickable:hover .event-marker-icon {
  filter: drop-shadow(0 4px 8px rgba(0, 0, 0, 0.5));
}

/* Event type specific colors */
.event-marker-icon.combat {
  color: #ef4444 !important;
  text-shadow: 0 0 8px rgba(239, 68, 68, 0.6);
}

.event-marker-icon.core_capture {
  color: #3b82f6 !important;
  text-shadow: 0 0 8px rgba(59, 130, 246, 0.6);
}

.event-marker-icon.energy_milestone {
  color: #06b6d4 !important;
  text-shadow: 0 0 8px rgba(6, 182, 212, 0.6);
}

.event-marker-icon.mass_death {
  color: #6b7280 !important;
  text-shadow: 0 0 8px rgba(107, 114, 128, 0.6);
}

.event-marker-icon.momentum_shift {
  color: #22c55e !important;
  text-shadow: 0 0 8px rgba(34, 197, 94, 0.6);
}

.event-marker-icon.critical_moment {
  color: #eab308 !important;
  text-shadow: 0 0 8px rgba(234, 179, 8, 0.6);
}

.event-marker-icon.spawn_wave {
  color: #a855f7 !important;
  text-shadow: 0 0 8px rgba(168, 85, 247, 0.6);
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

/* Event legend (optional) */
.event-ribbon-legend {
  background: var(--bg-secondary, #0f172a);
  border-top: 1px solid var(--border, #1e293b);
  font-size: 0.75rem;
  transition: max-height 0.3s ease, opacity 0.3s ease, margin 0.3s ease;
  max-height: 200px;
  overflow: hidden;
  opacity: 1;
  margin-top: 4px;
  border-radius: 0 0 8px 8px;
}

.event-ribbon-legend-hidden {
  max-height: 0;
  opacity: 0;
  margin: 0;
  padding-top: 0;
  padding-bottom: 0;
  border-top: none;
}

.event-legend-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border, #1e293b);
}

.event-legend-title {
  font-weight: 600;
  color: var(--text-primary, #e2e8f0);
  text-transform: uppercase;
  font-size: 0.7rem;
  letter-spacing: 0.05em;
}

.event-legend-close {
  background: transparent;
  border: none;
  color: var(--text-secondary, #64748b);
  font-size: 1.2rem;
  line-height: 1;
  cursor: pointer;
  padding: 0;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: color 0.15s ease, background 0.15s ease;
}

.event-legend-close:hover {
  color: var(--text-primary, #e2e8f0);
  background: rgba(255, 255, 255, 0.1);
}

.event-legend-close:active {
  transform: scale(0.95);
}

.event-legend-content {
  display: flex;
  gap: 12px;
  padding: 8px 12px;
  flex-wrap: wrap;
  justify-content: center;
}

.event-legend-item {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--text-secondary, #64748b);
}

.event-legend-icon {
  font-size: 16px;
  line-height: 1;
}

/* Tooltip styles (§14.8) */
.event-tooltip {
  position: fixed;
  z-index: 1000;
  background: var(--bg-primary, #1e293b);
  border: 1px solid var(--border, #334155);
  border-radius: 8px;
  padding: 8px 12px;
  min-width: 160px;
  max-width: 240px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.05);
  opacity: 0;
  visibility: hidden;
  transform: translateY(8px);
  transition: opacity 0.2s ease, transform 0.2s ease, visibility 0.2s ease;
  pointer-events: none;
  font-family: system-ui, -apple-system, sans-serif;
  font-size: 13px;
  line-height: 1.4;
}

.event-tooltip-visible {
  opacity: 1;
  visibility: visible;
  transform: translateY(0);
}

.event-tooltip-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border, #334155);
  margin-bottom: 6px;
}

.event-tooltip-icon {
  font-size: 16px;
  line-height: 1;
}

.event-tooltip-type {
  font-weight: 600;
  color: var(--text-primary, #f1f5f9);
  text-transform: capitalize;
}

.event-tooltip-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.event-tooltip-description {
  color: var(--text-secondary, #94a3b8);
  word-wrap: break-word;
  overflow-wrap: break-word;
}

.event-tooltip-turn {
  color: var(--text-tertiary, #64748b);
  font-size: 12px;
  font-family: monospace;
  margin-top: 2px;
}

.event-tooltip-arrow {
  position: absolute;
  bottom: -6px;
  left: 50%;
  transform: translateX(-50%);
  width: 0;
  height: 0;
  border-left: 6px solid transparent;
  border-right: 6px solid transparent;
  border-top: 6px solid var(--border, #334155);
}

.event-tooltip-arrow::before {
  content: '';
  position: absolute;
  top: -7px;
  left: -5px;
  width: 0;
  height: 0;
  border-left: 5px solid transparent;
  border-right: 5px solid transparent;
  border-top: 5px solid var(--bg-primary, #1e293b);
}

/* Hover state for icon */
.event-marker-icon {
  cursor: help;
  transition: transform 0.15s ease, filter 0.15s ease;
}

.event-marker-icon:hover {
  transform: scale(1.15);
  filter: brightness(1.2);
}

/* Reduced motion for tooltips */
@media (prefers-reduced-motion: reduce) {
  .event-tooltip {
    transition: opacity 0.1s ease, visibility 0.1s ease;
  }

  .event-marker-icon {
    transition: none;
  }
}

/* Reduced motion */
@media (prefers-reduced-motion: reduce) {
  .event-marker,
  .event-marker-icon,
  .event-ribbon-cursor {
    transition: none;
  }
}

/* Responsive legend adjustments */
@media (max-width: 640px) {
  .event-legend-content {
    gap: 8px;
    padding: 6px 8px;
  }

  .event-legend-item {
    font-size: 0.7rem;
  }

  .event-legend-icon {
    font-size: 14px;
  }

  .event-legend-header {
    padding: 6px 8px;
  }

  .event-legend-title {
    font-size: 0.65rem;
  }
}

@media (max-width: 480px) {
  .event-legend-content {
    gap: 6px;
    justify-content: flex-start;
    overflow-x: auto;
    flex-wrap: nowrap;
  }

  .event-legend-item {
    flex-shrink: 0;
    padding-right: 8px;
  }

  .event-legend-item:last-child {
    padding-right: 0;
  }
}
`;
