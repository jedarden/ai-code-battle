// Event Type Registry Tests
// The registry is the single source of truth shared by the ribbon markers, the
// tooltip, the legend and the generated marker CSS — these tests pin the
// contract that makes that sharing safe.

import { describe, it, expect } from 'vitest';
import {
  EVENT_TYPE_REGISTRY,
  UNKNOWN_EVENT_TYPE,
  getEventTypeDescriptor,
  isKnownEventType,
  eventTypeColorWithAlpha,
} from './event-type-registry';
import type { SignificantEventType } from '../extract-significant-events';

// Every member of the SignificantEventType union, spelled out so a type added
// to the union without a registry entry (or a registry key orphaned by a
// rename) fails here even though the Record typing alone would only catch one
// of the two directions
const ALL_EVENT_TYPES: SignificantEventType[] = [
  'combat',
  'core_capture',
  'energy_milestone',
  'mass_death',
  'momentum_shift',
  'critical_moment',
  'spawn_wave',
];

describe('EVENT_TYPE_REGISTRY', () => {
  it('should have an entry for every event type in the union', () => {
    for (const type of ALL_EVENT_TYPES) {
      expect(EVENT_TYPE_REGISTRY[type], `missing registry entry for ${type}`).toBeDefined();
    }
  });

  it('should have no registry entries outside the union', () => {
    expect(Object.keys(EVENT_TYPE_REGISTRY).sort()).toEqual([...ALL_EVENT_TYPES].sort());
  });

  it('should resolve every event type to a non-empty icon, name and color', () => {
    for (const type of ALL_EVENT_TYPES) {
      const style = getEventTypeDescriptor(type);
      expect(style.icon.trim(), `icon for ${type}`).not.toBe('');
      expect(style.name.trim(), `name for ${type}`).not.toBe('');
      expect(style.color.trim(), `color for ${type}`).not.toBe('');
    }
  });

  it('should give every event type a distinct name and color', () => {
    // The legend shows all entries side by side; two entries sharing a name or
    // a color would read as the same event type
    const entries = Object.values(EVENT_TYPE_REGISTRY);
    expect(new Set(entries.map(e => e.name)).size).toBe(entries.length);
    expect(new Set(entries.map(e => e.color)).size).toBe(entries.length);
  });

  it('should pin the semantic color each event type reads as', () => {
    // The acceptance list for the ribbon names a color family per type: combat
    // red/warning, core capture blue/primary, energy cyan/teal, mass death
    // dark grey, momentum green/growth, critical moment yellow/gold, spawn
    // wave purple/special. The markers, the glow derived from them, the
    // tooltip, the legend and the generated marker CSS all take these values
    // through the registry, and the tests around them only ever read the color
    // back out of it or check that the seven differ — so without this pin a
    // swap between two types would leave the whole suite green while changing
    // what every marker on the ribbon means.
    expect(EVENT_TYPE_REGISTRY.combat.color).toBe('#ef4444'); // Red/warning
    expect(EVENT_TYPE_REGISTRY.core_capture.color).toBe('#3b82f6'); // Blue/primary
    expect(EVENT_TYPE_REGISTRY.energy_milestone.color).toBe('#06b6d4'); // Cyan/teal
    expect(EVENT_TYPE_REGISTRY.mass_death.color).toBe('#6b7280'); // Dark grey/death
    expect(EVENT_TYPE_REGISTRY.momentum_shift.color).toBe('#22c55e'); // Green/growth
    expect(EVENT_TYPE_REGISTRY.critical_moment.color).toBe('#eab308'); // Yellow/gold
    expect(EVENT_TYPE_REGISTRY.spawn_wave.color).toBe('#a855f7'); // Purple/special
  });

  it('should use valid hex colors so derived glows can be computed', () => {
    for (const [type, { color }] of Object.entries(EVENT_TYPE_REGISTRY)) {
      expect(color, `color for ${type}`).toMatch(/^#[0-9a-f]{6}$/i);
    }
  });
});

describe('getEventTypeDescriptor', () => {
  it('should resolve known types to their registry entry', () => {
    expect(getEventTypeDescriptor('combat')).toBe(EVENT_TYPE_REGISTRY.combat);
    expect(getEventTypeDescriptor('spawn_wave')).toBe(EVENT_TYPE_REGISTRY.spawn_wave);
  });

  it('should fall back to the unknown-type entry for a type with no registry entry', () => {
    expect(getEventTypeDescriptor('something_new')).toBe(UNKNOWN_EVENT_TYPE);
    expect(getEventTypeDescriptor('')).toBe(UNKNOWN_EVENT_TYPE);
  });

  it('should resolve the fallback to a non-empty icon, name and color', () => {
    expect(UNKNOWN_EVENT_TYPE.icon.trim()).not.toBe('');
    expect(UNKNOWN_EVENT_TYPE.name.trim()).not.toBe('');
    expect(UNKNOWN_EVENT_TYPE.color.trim()).not.toBe('');
  });
});

describe('isKnownEventType', () => {
  it('should accept every registry type and reject anything else', () => {
    for (const type of ALL_EVENT_TYPES) {
      expect(isKnownEventType(type)).toBe(true);
    }
    expect(isKnownEventType('not_a_type')).toBe(false);
  });
});

describe('eventTypeColorWithAlpha', () => {
  it('should convert a registry color to rgba at the requested alpha', () => {
    expect(eventTypeColorWithAlpha('#ef4444', 0.6)).toBe('rgba(239, 68, 68, 0.6)');
  });

  it('should produce usable channels for every registry color', () => {
    for (const { color } of Object.values(EVENT_TYPE_REGISTRY)) {
      expect(eventTypeColorWithAlpha(color, 0.6)).toMatch(/^rgba\(\d+, \d+, \d+, 0.6\)$/);
    }
  });
});
