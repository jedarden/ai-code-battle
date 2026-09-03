// Event Ribbon Component
// Horizontal event timeline displaying SignificantEvents proportionally by turn
// Positioned below the replay canvas as a navigation aid

import type { SignificantEvent } from '../extract-significant-events';
import {
  EVENT_TYPE_REGISTRY,
  UNKNOWN_EVENT_TYPE,
  eventTypeColorWithAlpha,
  getEventTypeDescriptor,
  isKnownEventType,
  type EventTypeDescriptor,
} from './event-type-registry';

// ─────────────────────────────────────────────────────────────────────────────
// Overlap layering (same-turn stacking)
// ─────────────────────────────────────────────────────────────────────────────
// Several events can share a turn, which puts their markers at the same x
// position. Layering rules applied by renderMarkers/createEventMarker:
//
//  1. z-index is scoped per turn: `Z_INDEX_BASE + withinTurnIndex`. Events on
//     *different* turns all sit at Z_INDEX_BASE, so layering never changes how
//     they relate to each other; when their oversized tap targets merely graze,
//     DOM order (turn order) is the deterministic tiebreak.
//  2. Events on the *same* turn stack in array order. `this.events` arrives
//     sorted by turn (see extract-significant-events), so array order within a
//     turn is extraction order — deterministic across renders for a replay.
//  3. Same-turn markers are also offset vertically (a cascade) so every icon
//     stays partially visible. The spread is clamped to STACK_MAX_SPREAD_PX:
//     the ribbon clips overflow (48px tall → ±12px usable around the track
//     once the 22px icon is accounted for), so large groups compress the step
//     instead of pushing outer icons into the clip.
//  4. Hovering raises a marker to Z_INDEX_HOVER so its icon and tooltip are
//     never painted underneath a stacked sibling. This has to happen in JS:
//     the inline z-index from rule 1 beats any stylesheet :hover rule. To keep
//     that guarantee absolute, layered z-indexes are capped one below
//     Z_INDEX_HOVER — a turn would need ~90+ stacked events to outgrow it,
//     but the cap makes the invariant hold by construction rather than by
//     assumption; past the cap DOM order (rule 2) stays the tiebreak.
const Z_INDEX_BASE = 10;          // Above the turn cursor (z-index 5)
const Z_INDEX_HOVER = 100;        // Raised marker, still below tooltips (1000)
const STACK_OFFSET_STEP_PX = 6;   // Vertical gap between stacked markers
const STACK_MAX_SPREAD_PX = 24;   // Clamp: total top-to-bottom spread

// ─────────────────────────────────────────────────────────────────────────────
// Tooltip placement
// ─────────────────────────────────────────────────────────────────────────────
// There is exactly ONE tooltip per ribbon, appended to document.body (see
// buildTooltip). It must live outside .event-ribbon for two reasons, both of
// which make an in-marker tooltip unrenderable no matter how its coordinates
// are computed:
//
//  1. .event-ribbon has overflow: hidden and is 48px tall — a tooltip inside
//     it is clipped to the ribbon box and can never escape.
//  2. .event-marker carries a transform (centering + stack cascade), and a
//     transformed element becomes the containing block for position: fixed
//     descendants — so the tooltip's viewport coordinates would be measured
//     from the marker instead of the viewport.
//
// On body, position: fixed is genuinely viewport-relative and nothing clips
// it. Sharing one element is also what makes repositioning across icons a
// smooth transition rather than a fade-out/fade-in between separate nodes.
const TOOLTIP_GAP_PX = 12;        // Gap between tooltip and hovered marker
const TOOLTIP_EDGE_PADDING_PX = 8; // Minimum padding from the viewport edges
const TOOLTIP_ARROW_INSET_PX = 10; // Min distance from tooltip edge to arrow
// Pointer leaves an icon for a moment when crossing the gap to a stacked
// sibling; keep the tooltip up briefly so the move is a position transition
// instead of a flicker. Cancelled by re-entering any marker.
const TOOLTIP_HIDE_DELAY_MS = 100;

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
  private rootEl!: HTMLDivElement;
  private ribbonEl!: HTMLDivElement;
  private markersContainer!: HTMLDivElement;
  private onEventClick?: (event: SignificantEvent) => void;
  private onTurnClick?: (turn: number) => void;
  private getTurn?: () => number;
  private events: SignificantEvent[] = [];
  private totalTurns: number = 0;
  private legendVisible: boolean = true;
  private legendEl?: HTMLElement;
  private legendContentEl?: HTMLElement;
  private legendCloseButton?: HTMLButtonElement;
  private legendToggleButton?: HTMLButtonElement;
  private readonly STORAGE_KEY = 'event-ribbon-legend-visible';
  private tooltipEl!: HTMLDivElement;
  private tooltipHideTimer?: ReturnType<typeof setTimeout>;

  constructor(options: EventRibbonOptions) {
    this.container = options.container;
    this.onEventClick = options.onEventClick;
    this.onTurnClick = options.onTurnClick;
    this.getTurn = options.getTurn;

    // Load saved legend visibility preference
    this.legendVisible = this.loadLegendPreference();

    this.buildTooltip();
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
    this.renderLegendEntries();
  }

  /**
   * Clear all events from the ribbon.
   */
  public clear(): void {
    this.events = [];
    this.totalTurns = 0;
    this.renderMarkers();
    this.renderLegendEntries();
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
    legend.appendChild(content);
    this.legendContentEl = content;

    // Entries are filled by the same path setEvents/clear use, so the shell
    // (built once, here) stays separate from the entries (rebuilt on every
    // event update) and the key can never hold a stale set
    this.renderLegendEntries();

    // Both go into the stack root: the legend sits directly below the ribbon,
    // and the toggle chip occupies the same slot once the legend collapses, so
    // neither can end up beside the ribbon or on top of another control
    this.rootEl.appendChild(legend);

    // Create toggle button (visible only when legend is hidden)
    this.legendToggleButton = document.createElement('button');
    this.legendToggleButton.className = 'event-legend-toggle';
    this.legendToggleButton.setAttribute('type', 'button');
    this.legendToggleButton.setAttribute('aria-label', 'Show legend');
    this.legendToggleButton.innerHTML = '☰ Event Types';
    this.legendToggleButton.addEventListener('click', () => this.showLegend());
    this.rootEl.appendChild(this.legendToggleButton);

    // Apply saved visibility preference
    if (!this.legendVisible) {
      this.hideLegend(false); // Don't save preference on initial load
    }
  }

  /**
   * Rebuild the legend's entries from the registry plus the event types
   * actually present in the data.
   *
   * Every registry type always gets an entry, so the key reads as a key even
   * before any event arrives. A type arriving in the replay data that the
   * registry doesn't know gets an entry too, resolved to UNKNOWN_EVENT_TYPE —
   * the same descriptor the marker for that event renders — so the key covers
   * everything the ribbon can show and never disagrees with it. A no-op until
   * renderLegend has built the shell (the constructor can run this before any
   * legend exists, via setEvents).
   */
  private renderLegendEntries(): void {
    const content = this.legendContentEl;
    if (!content) return;

    // Data-derived types with no registry entry, in first-appearance order —
    // this.events arrives turn-sorted, so that is also turn order
    const unknownTypes: string[] = [];
    for (const event of this.events) {
      if (!isKnownEventType(event.type) && !unknownTypes.includes(event.type)) {
        unknownTypes.push(event.type);
      }
    }

    // Rebuild in place: emptying the container and refilling keeps the legend,
    // its header and its toggle exactly where they are, so repeated setEvents
    // calls can neither duplicate entries nor leak their nodes
    content.innerHTML = '';

    for (const [type, style] of Object.entries(EVENT_TYPE_REGISTRY)) {
      content.appendChild(this.createLegendItem(type, style, false));
    }
    for (const type of unknownTypes) {
      content.appendChild(this.createLegendItem(type, UNKNOWN_EVENT_TYPE, true));
    }
  }

  /**
   * Build one legend entry: icon and color from the descriptor, which is the
   * same object the marker for this type renders from, so the key and the
   * ribbon can't drift. The type string only ever lands in a dataset field
   * (never in HTML), so an unvalidated replay type can't inject markup.
   */
  private createLegendItem(type: string, style: EventTypeDescriptor, unknown: boolean): HTMLElement {
    const item = document.createElement('div');
    item.className = unknown ? 'event-legend-item event-legend-item-unknown' : 'event-legend-item';
    item.dataset.eventType = type;

    item.innerHTML = `
      <span class="event-legend-icon" style="color: ${style.color}">${style.icon}</span>
      <span class="event-legend-label">${style.name}</span>
    `;

    return item;
  }

  /**
   * Hide the legend.
   * @param savePreference - Whether to save to localStorage (default: true)
   */
  public hideLegend(savePreference: boolean = true): void {
    if (this.legendEl) {
      this.legendEl.classList.add('event-ribbon-legend-hidden');
      this.container.classList.add('event-ribbon-legend-hidden-container');
      this.legendVisible = false;
      if (savePreference) {
        this.saveLegendPreference(false);
      }
    }
  }

  /**
   * Show the legend.
   * @param savePreference - Whether to save to localStorage (default: true)
   */
  public showLegend(savePreference: boolean = true): void {
    if (this.legendEl) {
      this.legendEl.classList.remove('event-ribbon-legend-hidden');
      this.container.classList.remove('event-ribbon-legend-hidden-container');
      this.legendVisible = true;
      if (savePreference) {
        this.saveLegendPreference(true);
      }
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
   * Load legend visibility preference from localStorage.
   * @returns true if legend should be visible, false otherwise (default: true)
   */
  private loadLegendPreference(): boolean {
    try {
      const saved = localStorage.getItem(this.STORAGE_KEY);
      return saved === null ? true : saved === 'true';
    } catch {
      return true; // Default to visible if localStorage fails
    }
  }

  /**
   * Save legend visibility preference to localStorage.
   * @param visible - Whether the legend should be visible
   */
  private saveLegendPreference(visible: boolean): void {
    try {
      localStorage.setItem(this.STORAGE_KEY, String(visible));
    } catch {
      // Silently fail if localStorage is unavailable
    }
  }

  /**
   * Destroy the component and remove from DOM.
   */
  public destroy(): void {
    // The stack root holds the ribbon (and, once renderLegend runs, the legend
    // and its toggle), so removing it takes the whole component out of the DOM
    if (this.rootEl && this.rootEl.parentNode) {
      this.rootEl.parentNode.removeChild(this.rootEl);
    } else if (this.ribbonEl && this.ribbonEl.parentNode) {
      this.ribbonEl.parentNode.removeChild(this.ribbonEl);
    }
    // Also remove legend if it exists
    const legend = this.container.querySelector('.event-ribbon-legend');
    if (legend && legend.parentNode) {
      legend.parentNode.removeChild(legend);
    }
    // Remove the shared tooltip from document.body
    if (this.tooltipEl && this.tooltipEl.parentNode) {
      this.tooltipEl.parentNode.removeChild(this.tooltipEl);
    }
    if (this.tooltipHideTimer !== undefined) {
      clearTimeout(this.tooltipHideTimer);
      this.tooltipHideTimer = undefined;
    }
  }

  // ── Private Methods ─────────────────────────────────────────────────────────────

  private buildDOM(): void {
    // Stack root: the ribbon and the legend keep their vertical order here,
    // inside this component, instead of trusting the parent's layout — the
    // replay page mounts the ribbon into a flex-row scroller (#mobile-timeline),
    // which would otherwise place the legend beside the ribbon (off-screen in
    // the scroll area) rather than below it. position: relative also gives the
    // legend toggle a containing block, so it never anchors to a distant
    // positioned ancestor elsewhere on the page.
    this.rootEl = document.createElement('div');
    this.rootEl.className = 'event-ribbon-root';

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

    this.rootEl.appendChild(this.ribbonEl);
    this.container.appendChild(this.rootEl);
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

    // Group events by turn so co-located markers can be stacked deterministically
    const eventsByTurn = new Map<number, number[]>();
    this.events.forEach((event, index) => {
      const turn = event.turn;
      if (!eventsByTurn.has(turn)) {
        eventsByTurn.set(turn, []);
      }
      eventsByTurn.get(turn)!.push(index);
    });

    // Render a marker for each event, layered per the "Overlap layering" notes
    // at the top of this file
    for (let i = 0; i < this.events.length; i++) {
      const event = this.events[i];
      const turnEvents = eventsByTurn.get(event.turn) || [];
      const withinTurnIndex = turnEvents.indexOf(i);
      const marker = this.createEventMarker(event, withinTurnIndex, turnEvents.length);
      this.markersContainer.appendChild(marker);
    }

    // Update cursor position if we have a getTurn function
    if (this.getTurn) {
      this.updateTurnHighlight();
    }
  }

  private createEventMarker(event: SignificantEvent, withinTurnIndex: number, turnEventCount: number): HTMLDivElement {
    const marker = document.createElement('div');
    marker.className = 'event-marker';
    marker.dataset.turn = event.turn.toString();
    marker.dataset.eventType = event.type;

    // Calculate proportional position: iconLeft = (eventTurn / totalTurns) * 100%
    const positionPercent = this.calculatePosition(event.turn);
    marker.style.left = `${positionPercent}%`;

    // Deterministic per-turn stacking (see "Overlap layering" notes at the top
    // of this file): extraction order within the turn decides who paints on
    // top, while different-turn events all stay at Z_INDEX_BASE. The cap keeps
    // the layered value below Z_INDEX_HOVER so the hover raise always wins.
    const layeredZIndex = `${Math.min(Z_INDEX_BASE + withinTurnIndex, Z_INDEX_HOVER - 1)}`;
    marker.style.zIndex = layeredZIndex;

    // Vertical cascade so co-located icons all stay partially visible
    marker.style.transform = this.getStackedTransform(withinTurnIndex, turnEventCount);

    // Icon and color come from the registry. A type with no entry still
    // resolves — to the unknown-type fallback — so the marker is never empty.
    const eventStyle = getEventTypeDescriptor(event.type);

    // The type class only exists for known types: it selects the per-type CSS
    // rule (generated from the registry, see EVENT_RIBBON_STYLES), and an
    // unknown type arriving in replay data must not reach a class name raw
    const typeClass = isKnownEventType(event.type) ? ` ${event.type}` : '';

    // Render icon (the tooltip itself is a single shared element on
    // document.body — see buildTooltip)
    marker.innerHTML = `
      <div class="event-marker-icon${typeClass}" style="color: ${eventStyle.color}">
        ${event.emoji || eventStyle.icon}
      </div>
    `;

    // Add hover event listeners for the shared tooltip
    const icon = marker.querySelector('.event-marker-icon') as HTMLElement;

    if (icon) {
      // Raise the whole marker on hover so it paints above any stacked
      // sibling, then restore its layered z-index on the way out
      const collapseLayer = (): void => {
        marker.style.zIndex = layeredZIndex;
        this.scheduleTooltipHide();
      };

      icon.addEventListener('mouseenter', () => {
        marker.style.zIndex = `${Z_INDEX_HOVER}`;
        this.showTooltip(marker, event);
      });

      icon.addEventListener('mouseleave', collapseLayer);

      // Also collapse on marker mouse leave to handle edge cases (pointer
      // re-entering the marker outside the icon)
      marker.addEventListener('mouseleave', collapseLayer);
    }

    // Optional click handler — a marker click is both "jump to this event" and
    // "jump to this turn", so both callbacks fire from the same listener.
    // stopPropagation keeps the click off the track's seek handler below, which
    // would otherwise compute the turn from the click coordinates a second time.
    if (this.onEventClick || this.onTurnClick) {
      marker.addEventListener('click', (e) => {
        e.stopPropagation();
        this.onEventClick?.(event);
        this.onTurnClick?.(event.turn);
      });
      marker.style.cursor = 'pointer';
      marker.classList.add('event-marker-clickable');
    }

    return marker;
  }

  /**
   * Transform for a marker, including the vertical cascade used when several
   * events share a turn. A lone event is simply centered; a group spreads
   * symmetrically around the track, with the per-marker step compressed once
   * the group is large enough that the unclamped spread (step × count-1) would
   * push the outer icons into the ribbon's overflow clip (see
   * STACK_MAX_SPREAD_PX).
   */
  private getStackedTransform(withinTurnIndex: number, turnEventCount: number): string {
    if (turnEventCount <= 1) {
      return 'translate(-50%, -50%)';
    }
    const step = Math.min(STACK_OFFSET_STEP_PX, STACK_MAX_SPREAD_PX / (turnEventCount - 1));
    const offset = (withinTurnIndex - (turnEventCount - 1) / 2) * step;
    const rounded = Math.round(offset * 100) / 100;
    return `translate(-50%, calc(-50% + ${rounded}px))`;
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

  /**
   * Create the single shared tooltip and attach it to document.body.
   * See the "Tooltip placement" notes at the top of this file for why it
   * cannot live inside the ribbon.
   */
  private buildTooltip(): void {
    const tooltip = document.createElement('div');
    tooltip.className = 'event-tooltip';
    tooltip.setAttribute('role', 'tooltip');
    tooltip.setAttribute('aria-hidden', 'true');
    tooltip.innerHTML = `
      <div class="event-tooltip-header">
        <span class="event-tooltip-icon"></span>
        <span class="event-tooltip-type"></span>
      </div>
      <div class="event-tooltip-body">
        <div class="event-tooltip-description"></div>
        <div class="event-tooltip-turn"></div>
      </div>
      <div class="event-tooltip-arrow"></div>
    `;
    document.body.appendChild(tooltip);
    this.tooltipEl = tooltip;
  }

  private showTooltip(marker: HTMLElement, event: SignificantEvent): void {
    // Re-entering a marker within the hide grace period keeps the tooltip up
    // (and animates it to the new position) instead of flickering
    if (this.tooltipHideTimer !== undefined) {
      clearTimeout(this.tooltipHideTimer);
      this.tooltipHideTimer = undefined;
    }

    this.renderTooltipContent(event);
    this.tooltipEl.setAttribute('aria-hidden', 'false');
    this.tooltipEl.classList.add('event-tooltip-visible');

    // Position tooltip to avoid viewport overflow
    this.positionTooltip(marker, this.tooltipEl);
  }

  /**
   * Hide the tooltip after a short grace period. Moving between adjacent
   * icons crosses a gap where neither marker is hovered; the delay lets the
   * next mouseenter cancel the hide so the tooltip glides rather than
   * disappearing and reappearing.
   */
  private scheduleTooltipHide(): void {
    if (this.tooltipHideTimer !== undefined) {
      return; // Already scheduled
    }
    this.tooltipHideTimer = setTimeout(() => {
      this.tooltipHideTimer = undefined;
      this.hideTooltip();
    }, TOOLTIP_HIDE_DELAY_MS);
  }

  private hideTooltip(): void {
    if (this.tooltipHideTimer !== undefined) {
      clearTimeout(this.tooltipHideTimer);
      this.tooltipHideTimer = undefined;
    }
    // Make tooltip invisible and inaccessible
    this.tooltipEl.setAttribute('aria-hidden', 'true');
    this.tooltipEl.classList.remove('event-tooltip-visible');

    // Reset the arrow's placement class too: the flipped variant is chosen per
    // show in positionTooltip, but only placement that actually ran gets to
    // choose it again — leaving it applied would render the next top-placed
    // tooltip with a downward-pointing arrow.
    const arrow = this.tooltipEl.querySelector('.event-tooltip-arrow') as HTMLElement | null;
    if (arrow) {
      arrow.classList.remove('event-tooltip-arrow-flipped');
    }
    // Left/top are deliberately kept: the next show overwrites them, and a
    // quick re-show near the old spot starts its position transition from
    // there instead of jumping
  }

  private renderTooltipContent(event: SignificantEvent): void {
    // Type facts (icon fallback, display name) come from the registry
    const style = getEventTypeDescriptor(event.type);
    const icon = this.tooltipEl.querySelector('.event-tooltip-icon') as HTMLElement;
    const type = this.tooltipEl.querySelector('.event-tooltip-type') as HTMLElement;
    const description = this.tooltipEl.querySelector('.event-tooltip-description') as HTMLElement;
    const turn = this.tooltipEl.querySelector('.event-tooltip-turn') as HTMLElement;

    icon.innerHTML = this.escapeHtml(event.emoji || style.icon);
    type.textContent = style.name;
    description.innerHTML = this.escapeHtml(event.description);
    turn.textContent = `Turn ${event.turn}`;
  }

  private positionTooltip(marker: HTMLElement, tooltip: HTMLElement): void {
    const markerRect = marker.getBoundingClientRect();
    const tooltipRect = tooltip.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;

    const GAP = TOOLTIP_GAP_PX;
    const EDGE_PADDING = TOOLTIP_EDGE_PADDING_PX;

    // Calculate available space in all four directions
    const spaceAbove = markerRect.top - EDGE_PADDING;
    const spaceBelow = viewportHeight - markerRect.bottom - EDGE_PADDING;
    const spaceLeft = markerRect.left - EDGE_PADDING;
    const spaceRight = viewportWidth - markerRect.right - EDGE_PADDING;

    // Determine preferred placement based on available space
    // Prioritize vertical placement (top/bottom) over horizontal
    let placement: 'top' | 'bottom' | 'left' | 'right';
    if (spaceAbove >= tooltipRect.height + GAP) {
      placement = 'top';
    } else if (spaceBelow >= tooltipRect.height + GAP) {
      placement = 'bottom';
    } else if (spaceRight >= tooltipRect.width + GAP) {
      placement = 'right';
    } else if (spaceLeft >= tooltipRect.width + GAP) {
      placement = 'left';
    } else {
      // Fallback: choose direction with most space
      const maxSpace = Math.max(spaceAbove, spaceBelow, spaceLeft, spaceRight);
      if (maxSpace === spaceAbove) placement = 'top';
      else if (maxSpace === spaceBelow) placement = 'bottom';
      else if (maxSpace === spaceRight) placement = 'right';
      else placement = 'left';
    }

    // Calculate position based on placement
    let tooltipTop: number;
    let tooltipLeft: number;
    let arrowTop = 'auto';
    let arrowLeft = '50%';
    let arrowBottom = 'auto';
    let arrowRight = 'auto';

    switch (placement) {
      case 'top':
        tooltipTop = markerRect.top - tooltipRect.height - GAP;
        tooltipLeft = markerRect.left + (markerRect.width / 2) - (tooltipRect.width / 2);
        // Clamp horizontal position to prevent overflow
        tooltipLeft = Math.max(EDGE_PADDING, Math.min(tooltipLeft, viewportWidth - tooltipRect.width - EDGE_PADDING));
        // Arrow stays at bottom, pointing up
        arrowBottom = '-6px';
        break;

      case 'bottom':
        tooltipTop = markerRect.bottom + GAP;
        tooltipLeft = markerRect.left + (markerRect.width / 2) - (tooltipRect.width / 2);
        // Clamp horizontal position
        tooltipLeft = Math.max(EDGE_PADDING, Math.min(tooltipLeft, viewportWidth - tooltipRect.width - EDGE_PADDING));
        // Arrow flips to top, pointing down
        arrowTop = '-6px';
        arrowBottom = 'auto';
        break;

      case 'left':
        tooltipTop = markerRect.top + (markerRect.height / 2) - (tooltipRect.height / 2);
        tooltipLeft = markerRect.left - tooltipRect.width - GAP;
        // Clamp vertical position
        tooltipTop = Math.max(EDGE_PADDING, Math.min(tooltipTop, viewportHeight - tooltipRect.height - EDGE_PADDING));
        // Clamp horizontal position (fallback when viewport is narrower than tooltip)
        tooltipLeft = Math.max(EDGE_PADDING, Math.min(tooltipLeft, viewportWidth - tooltipRect.width - EDGE_PADDING));
        // Arrow moves to right side, pointing left
        arrowLeft = 'auto';
        arrowRight = '-6px';
        break;

      case 'right':
        tooltipTop = markerRect.top + (markerRect.height / 2) - (tooltipRect.height / 2);
        tooltipLeft = markerRect.right + GAP;
        // Clamp vertical position
        tooltipTop = Math.max(EDGE_PADDING, Math.min(tooltipTop, viewportHeight - tooltipRect.height - EDGE_PADDING));
        // Clamp horizontal position (fallback when viewport is narrower than tooltip)
        tooltipLeft = Math.max(EDGE_PADDING, Math.min(tooltipLeft, viewportWidth - tooltipRect.width - EDGE_PADDING));
        // Arrow moves to left side, pointing right
        arrowLeft = '-6px';
        arrowRight = 'auto';
        break;
    }

    // Apply positioning. The left/top transition lives in the stylesheet
    // (.event-tooltip) — setting an inline transition here would replace the
    // whole shorthand and kill the opacity/transform fade.
    tooltip.style.left = `${tooltipLeft}px`;
    tooltip.style.top = `${tooltipTop}px`;

    // Update arrow positioning and rotation based on placement
    const arrow = tooltip.querySelector('.event-tooltip-arrow') as HTMLElement;
    if (arrow) {
      // Keep the arrow inside the tooltip: once the tooltip is clamped to the
      // viewport, the marker centre it points at can sit beyond its edge
      const clampArrowOffset = (offset: number, size: number): number =>
        Math.max(TOOLTIP_ARROW_INSET_PX, Math.min(offset, size - TOOLTIP_ARROW_INSET_PX));

      // Calculate arrow offset for horizontal positioning
      if (placement === 'top' || placement === 'bottom') {
        const markerCenter = markerRect.left + (markerRect.width / 2);
        const arrowOffset = clampArrowOffset(markerCenter - tooltipLeft, tooltipRect.width);
        arrow.style.left = `${arrowOffset}px`;
        arrow.style.right = 'auto';
        arrow.style.top = arrowTop;
        arrow.style.bottom = arrowBottom;
        arrow.style.transform = 'translateX(-50%)';
      } else {
        // For left/right placement
        const markerCenter = markerRect.top + (markerRect.height / 2);
        const arrowOffset = clampArrowOffset(markerCenter - tooltipTop, tooltipRect.height);
        arrow.style.top = `${arrowOffset}px`;
        arrow.style.bottom = 'auto';
        arrow.style.left = arrowLeft;
        arrow.style.right = arrowRight;
        arrow.style.transform = 'translateY(-50%)';
      }

      // Update arrow rotation based on placement
      const rotationMap: Record<typeof placement, string> = {
        top: 'rotate(0deg)',
        bottom: 'rotate(180deg)',
        left: 'rotate(90deg)',
        right: 'rotate(-90deg)',
      };
      arrow.style.transform += ` ${rotationMap[placement]}`;

      // The arrow's filled edge is a pseudo-element, which JS can't address
      // directly — bottom placement gets a class that flips it instead
      arrow.className = 'event-tooltip-arrow';
      if (placement === 'bottom') {
        arrow.classList.add('event-tooltip-arrow-flipped');
      }
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// CSS Styles
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Per-type marker colors, generated from the event type registry so a color
 * change there lands in the markers, the tooltip and the legend together.
 * !important beats the inline color set on each icon; the point of the rules
 * is to keep the glow (text-shadow) derived from the same color as the icon.
 */
const EVENT_TYPE_MARKER_CSS = Object.entries(EVENT_TYPE_REGISTRY)
  .map(([type, style]) => `/* ${style.name} */
.event-marker-icon.${type} {
  color: ${style.color} !important;
  text-shadow: 0 0 8px ${eventTypeColorWithAlpha(style.color, 0.6)};
}`)
  .join('\n\n');

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

/* Hover z-raise is applied in JS (see Z_INDEX_HOVER) — the inline z-index set
   by the layering always beats a stylesheet :hover rule. */

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

/* Event type specific colors — generated from the event type registry */
${EVENT_TYPE_MARKER_CSS}

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

/* Stack root — wraps the ribbon, the legend and the legend toggle so their
   vertical order is fixed inside the component, whatever layout the parent
   container imposes on its children */
.event-ribbon-root {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  min-width: 0;
}

/* Legend toggle button — occupies the collapsed legend's slot below the
   ribbon, so it stays discoverable without ever covering a marker */
.event-legend-toggle {
  display: none;
  align-self: flex-end;
  margin: 6px 8px 8px;
  background: var(--bg-secondary, #0f172a);
  border: 1px solid var(--border, #1e293b);
  color: var(--text-secondary, #64748b);
  font-size: 0.75rem;
  padding: 6px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.2s ease, color 0.2s ease, border-color 0.2s ease;
}

.event-legend-toggle:hover {
  background: var(--bg-tertiary, #1e293b);
  color: var(--text-primary, #e2e8f0);
  border-color: var(--border-active, #334155);
}

.event-legend-toggle:active {
  transform: scale(0.98);
}

/* Show toggle button only when legend is hidden */
.event-ribbon-legend-hidden-container .event-legend-toggle {
  display: inline-flex;
  align-items: center;
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
  /* #94a3b8 rather than the #64748b used elsewhere: this text renders at
     0.75rem on --bg-secondary, and the darker fallback only reaches ~3.1:1
     there — under the 4.5:1 small text needs. #94a3b8 is 5.7:1. */
  color: var(--text-secondary, #94a3b8);
}

.event-legend-icon {
  font-size: 16px;
  line-height: 1;
  /* Same scrim the markers carry. The icon fills are the marker colors — they
     have to match, so they cannot be lightened for a light surface — and this
     dark halo keeps every one of them legible if --bg-secondary is themed
     light instead of falling back to #0f172a. */
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.55));
}

/* An event type the registry doesn't know, resolved to the unknown-type
   default. Marked typographically rather than by dimming, which would cost the
   label the contrast the rest of the key is held to. */
.event-legend-item-unknown .event-legend-label {
  font-style: italic;
  border-bottom: 1px dashed currentColor;
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
  /* Never wider than the viewport minus the edge padding, so the clamped
     placement in positionTooltip can always fit the whole tooltip */
  max-width: min(240px, calc(100vw - 16px));
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.05);
  opacity: 0;
  visibility: hidden;
  transform: translateY(8px);
  /* left/top transitions move the tooltip between icons; opacity/transform
     fade it in and out */
  transition: opacity 0.2s ease, transform 0.2s ease, visibility 0.2s ease,
    left 0.2s ease, top 0.2s ease;
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
  transition: left 0.2s ease, top 0.2s ease, right 0.2s ease, bottom 0.2s ease, transform 0.2s ease;
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

/* Arrow flipped state (for bottom placement) */
.event-tooltip-arrow-flipped {
  bottom: auto;
  top: -6px;
  transform: translateX(-50%) rotate(180deg);
}

.event-tooltip-arrow-flipped::before {
  top: auto;
  bottom: -7px;
  border-top: none;
  border-bottom: 5px solid var(--bg-primary, #1e293b);
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
