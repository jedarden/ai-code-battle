// Event Type Registry
// Single source of truth for how each SignificantEventType is presented in the
// event ribbon: its icon glyph, human-readable display name, and color. The
// markers, the shared tooltip, the legend and the per-type marker CSS all read
// from here, so the four can never drift apart (the mapping previously lived
// in three separate inline tables).

import type { SignificantEventType } from '../extract-significant-events';

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

/** Presentation facts for one event type, shared by markers, tooltip and legend */
export interface EventTypeDescriptor {
  icon: string;   // Glyph rendered inside the marker (and in the legend)
  name: string;   // Human-readable display name (tooltip title, legend label)
  color: string;  // Hex color for the icon and the glow derived from it
}

// ─────────────────────────────────────────────────────────────────────────────
// Registry
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Every event type the ribbon can render, keyed by SignificantEventType.
 * Typing the map over the full union makes a missing key (or a stale key left
 * behind by a renamed union member) a compile-time error, so a new event type
 * cannot ship without presentation facts.
 */
export const EVENT_TYPE_REGISTRY: Record<SignificantEventType, EventTypeDescriptor> = {
  combat:           { icon: '⚔️', name: 'Combat',           color: '#ef4444' },  // Red/warning
  core_capture:     { icon: '🏰', name: 'Core Capture',     color: '#3b82f6' },  // Blue/primary
  energy_milestone: { icon: '💎', name: 'Energy Milestone', color: '#06b6d4' },  // Cyan/teal
  mass_death:       { icon: '💀', name: 'Mass Death',       color: '#6b7280' },  // Dark grey/death
  momentum_shift:   { icon: '📈', name: 'Momentum Shift',   color: '#22c55e' },  // Green/growth
  critical_moment:  { icon: '🌟', name: 'Critical Moment',  color: '#eab308' },  // Yellow/gold
  spawn_wave:       { icon: '🐣', name: 'Spawn Wave',       color: '#a855f7' },  // Purple/special
};

/**
 * Fallback for a type with no registry entry. Extraction only ever produces
 * the keys above, but replay data crossing a version boundary can carry
 * anything, so an unknown type resolves here instead of to an empty marker —
 * the event stays visible, hoverable and clickable as a neutral grey dot.
 */
export const UNKNOWN_EVENT_TYPE: EventTypeDescriptor = {
  icon: '•',
  name: 'Event',
  color: '#6b7280',
};

/**
 * Look up the presentation facts for an event type. Takes a plain string so
 * unvalidated replay data can be passed directly; an unknown value resolves
 * to UNKNOWN_EVENT_TYPE rather than undefined.
 */
export function getEventTypeDescriptor(eventType: string): EventTypeDescriptor {
  return (EVENT_TYPE_REGISTRY as Record<string, EventTypeDescriptor>)[eventType] ?? UNKNOWN_EVENT_TYPE;
}

/**
 * True when an event type has a real registry entry (as opposed to resolving
 * to the unknown-type fallback). Callers rendering type-derived CSS classes
 * use this to keep unvalidated data out of the DOM.
 */
export function isKnownEventType(eventType: string): eventType is SignificantEventType {
  return eventType in EVENT_TYPE_REGISTRY;
}

/**
 * Convert a registry hex color to an rgba() string at the given alpha, for
 * glows and tints derived from the same source color.
 */
export function eventTypeColorWithAlpha(hex: string, alpha: number): string {
  const value = hex.replace('#', '');
  return `rgba(${parseInt(value.slice(0, 2), 16)}, ${parseInt(value.slice(2, 4), 16)}, ${parseInt(value.slice(4, 6), 16)}, ${alpha})`;
}
