# Event Ribbon Structure Documentation

Generated: 2026-09-02  
Task: Explore and document event ribbon structure

## Overview

The event ribbon is a horizontal timeline displaying significant game events proportionally by turn number. It provides both visual navigation (click-to-jump) and informational tooltips for event details.

## Key Files

### Core Components
- **`web/src/components/event-ribbon.ts`** (506 lines)
  - Main EventRibbon class component
  - Handles rendering, positioning, and interaction
  - Includes built-in legend with localStorage persistence
  
- **`web/src/extract-significant-events.ts`** (477 lines)
  - Client-side event extraction from replay data
  - Defines `SignificantEventType` union and event detection logic
  
- **`web/src/components/EventTypeLegend.tsx`** (93 lines)
  - React component for displaying event type → icon/color mapping
  - Reusable across the application
  
- **`web/src/components/event-timeline.ts`** (413 lines)
  - Alternative/older timeline implementation
  - Similar functionality but different DOM structure

### Supporting Files
- **`web/src/types.ts`** - Type definitions for Replay, GameEvent, Position, etc.
- **`web/src/styles/components.css`** - Global styles including `.event-type-legend` (lines 1405-1440)

## Event Type Definitions

### 7 Event Types

All event types defined in `extract-significant-events.ts`:

| Type | Icon | Color | Label | Description |
|------|------|-------|-------|-------------|
| `combat` | ⚔️ | `#ef4444` | Combat | Bot damage/knockout events |
| `core_capture` | 🏰 | `#3b82f6` | Core Capture | Core ownership changes |
| `energy_milestone` | 💎 | `#06b6d4` | Energy Milestone | Energy thresholds crossed |
| `mass_death` | 💀 | `#6b7280` | Mass Death | ≥3 bots dying within 3 turns |
| `momentum_shift` | 📈 | `#22c55e` | Momentum Shift | Score lead changes |
| `critical_moment` | 🌟 | `#eab308` | Critical Moment | Game state phase changes |
| `spawn_wave` | 🐣 | `#a855f7` | Spawn Wave | New bot spawns |

### Type Definitions

```typescript
export type SignificantEventType =
  | 'combat'
  | 'core_capture'
  | 'energy_milestone'
  | 'mass_death'
  | 'momentum_shift'
  | 'critical_moment'
  | 'spawn_wave';

export interface SignificantEvent {
  type: SignificantEventType;
  turn: number;
  description: string;
  botId?: number;
  playerId?: number;
  position?: Position;
  emoji?: string;
}
```

## Data Flow

```
Replay (JSON)
    ↓
extract-significant-events.ts
    - Analyzes turns, events, scores
    - Detects patterns (mass death, momentum shifts)
    ↓
SignificantEvent[]
    ↓
EventRibbon.setEvents(events, totalTurns)
    - Groups events by turn
    - Calculates proportional positions
    - Renders markers with z-index stacking
    ↓
DOM: .event-ribbon > .event-marker[]
```

### Event Extraction Logic

Located in `extract-significant-events.ts`:

1. **Raw events** - Direct from replay turn data
   - `bot_died`, `zone_death`, `combat_death` → `combat`
   - `core_captured` → `core_capture`
   - `bot_spawned` → `spawn_wave`
   - `energy_collected` (when ≥3 in turn) → `energy_milestone`

2. **Computed events** - Derived from pattern detection
   - **Mass death**: ≥3 deaths in 3-turn window
   - **Momentum shift**: Leader changes between turns
   - **Critical moment**: Game end, score delta >15%
   - **Energy milestone**: Player reaches [50, 100, 150] energy

## Current Rendering Approach

### DOM Structure

```html
<div class="event-ribbon">
  <div class="event-ribbon-track"></div>           <!-- Horizontal line -->
  <div class="event-ribbon-markers">
    <div class="event-marker" style="left: X%">
      <div class="event-marker-icon">⚔️</div>
      <div class="event-tooltip">...</div>
    </div>
  </div>
  <div class="event-ribbon-cursor"></div>           <!-- Current turn indicator -->
</div>

<!-- Optional legend below -->
<div class="event-ribbon-legend">
  <div class="event-legend-header">...</div>
  <div class="event-legend-content">
    <div class="event-legend-item">
      <span class="event-legend-icon">⚔️</span>
      <span class="event-legend-label">Combat</span>
    </div>
    <!-- ... other types ... -->
  </div>
</div>
```

### Positioning

**Proportional positioning:**
```typescript
positionPercent = (eventTurn / totalTurns) * 100
```

- Clamped to 0-100 range
- Single turn centers at 50%
- `left` CSS property applied to `.event-marker`

**Vertical offset for overlapping events:**
```typescript
// Events on same turn get vertical offset
offsetStep = 6px
offset = eventIndexOnTurn * offsetStep - (maxOffset / 2)
// Range: -6px to +6px for up to 3 events
transform: translate(-50%, calc(-50% + ${offset}px))
```

## Z-Index Strategy

### Layering Hierarchy

| Element | Base z-index | Hover z-index | Purpose |
|---------|--------------|---------------|---------|
| `.event-ribbon-cursor` | 5 | - | Current turn indicator |
| `.event-marker` | 10 + index | 100 | Event icons |
| `.event-tooltip` | - | 1000 | Hover tooltip |

**Stacking logic:**
```typescript
const zIndex = 10 + index;  // Later events stack higher
marker.style.zIndex = `${zIndex}`;
```

**Hover states:**
```css
.event-marker-clickable:hover {
  z-index: 100;  /* Pop above all markers */
}
.event-tooltip {
  z-index: 1000;  /* Above everything */
}
```

## Legend Patterns

### Built-in Legend (event-ribbon.ts)

**Features:**
- Located below ribbon container
- Header with close button (×)
- Toggle button (☰ Event Types) when hidden
- Flexbox layout with centered items
- **localStorage persistence** for visibility state

**API:**
```typescript
ribbon.renderLegend();
ribbon.hideLegend(savePreference?);
ribbon.showLegend(savePreference?);
ribbon.toggleLegend();
```

**DOM structure:**
```html
<div class="event-ribbon-legend">
  <div class="event-legend-header">
    <span class="event-legend-title">Event Types</span>
    <button class="event-legend-close">×</button>
  </div>
  <div class="event-legend-content">
    <div class="event-legend-item">
      <span class="event-legend-icon" style="color: #ef4444">⚔️</span>
      <span class="event-legend-label">Combat</span>
    </div>
    <!-- ... other types ... -->
  </div>
</div>
<button class="event-legend-toggle">☰ Event Types</button>
```

### React Legend Component (EventTypeLegend.tsx)

**Reusable React component** for displaying event type mappings:

```tsx
<EventTypeLegend eventTypes={['combat', 'core_capture']} />
```

**Features:**
- Shows all types by default, or filter with prop
- Icon + label per type
- CSS color application
- Responsive flexbox layout

**DOM output:**
```html
<div class="event-type-legend">
  <div class="event-type-legend-content">
    <div class="event-type-legend-item">
      <span class="event-type-legend-icon" style="color: #ef4444">⚔️</span>
      <span class="event-type-legend-label">Combat</span>
    </div>
    <!-- ... -->
  </div>
</div>
```

### Global CSS Styles

Located in `components.css` (lines 1405-1440):

```css
.event-type-legend {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: var(--space-md);
}

.event-type-legend-content {
  display: flex;
  gap: var(--space-md);
  flex-wrap: wrap;
  justify-content: center;
}
```

## Icon/Color Consistency

**Single source of truth** in two locations (must stay synchronized):

1. **`event-ribbon.ts`** - `getEventStyle()` method (lines 425-436)
2. **`EventTypeLegend.tsx`** - `EVENT_TYPE_STYLES` const (lines 36-44)

Both define:
```typescript
const STYLES: Record<SignificantEventType, { color, icon, label }> = {
  combat: { icon: '⚔️', color: '#ef4444', label: 'Combat' },
  core_capture: { icon: '🏰', color: '#3b82f6', label: 'Core Capture' },
  // ... etc
};
```

**Used in:**
- `event-ribbon.ts` - Marker rendering
- `EventTypeLegend.tsx` - Legend rendering
- `extract-significant-events.ts` - Emoji assignment in event creation

## Current Tooltips

### Structure

```html
<div class="event-tooltip" role="tooltip">
  <div class="event-tooltip-header">
    <span class="event-tooltip-icon">⚔️</span>
    <span class="event-tooltip-type">Combat</span>
  </div>
  <div class="event-tooltip-body">
    <div class="event-tooltip-description">Player A's bot destroyed</div>
    <div class="event-tooltip-turn">Turn 42</div>
  </div>
  <div class="event-tooltip-arrow"></div>
</div>
```

### Behavior

- **Fixed positioning** to viewport
- **Dynamic overflow handling** - adjusts left/right to stay in viewport
- **Arrow positioning** - points to marker center
- **z-index: 1000** - above all other elements
- **Fade-in animation** (150ms ease-out)
- **Reduced motion support** - instant on `prefers-reduced-motion`

### Positioning Logic

```typescript
tooltipLeft = markerCenter - (tooltipWidth / 2)
// Clamp to viewport edges:
if (tooltipLeft + tooltipWidth > viewportWidth - 8) {
  tooltipLeft = viewportWidth - tooltipWidth - 8;
}
if (tooltipLeft < 8) {
  tooltipLeft = 8;
}
```

## Annotations Integration

**Timeline also supports annotation badges** (from annotation system):

```typescript
interface Annotation {
  turn: number;
  type: 'insight' | 'mistake' | 'idea' | 'highlight';
}

// Annotation icons/colors:
insight: 💎 (blue #3b82f6)
mistake: ⚠️ (red #ef4444)
idea: 💡 (green #22c55e)
highlight: ⭐ (yellow #fbbf24)
```

**Rendered as separate markers** on timeline with `.timeline-annotation` class.

## Styling Conventions

### Icon Size

Consistent 22px icons with 44x44px tap targets (accessibility):

```css
.event-marker-icon {
  font-size: 22px;
  line-height: 1;
  width: 22px;
  height: 22px;
  padding: 11px;  /* 11px × 2 + 22px = 44px tap target */
  margin: -11px;
}
```

### Text Shadows

All event icons get colored text shadows:

```css
.event-marker-icon.combat {
  color: #ef4444 !important;
  text-shadow: 0 0 8px rgba(239, 68, 68, 0.6);
}
```

### Ribbon Dimensions

```css
.event-ribbon {
  height: 48px;
  background: var(--bg-secondary, #0f172a);
  border-top: 1px solid var(--border, #1e293b);
}
```

### Cursor Styling

```css
.event-ribbon-cursor {
  width: 2px;
  background: var(--text-primary, #e2e8f0);
  /* Triangle pointer at top */
}
```

## Acceptance Criteria Status

- ✅ **Identified event ribbon component files** - 4 key files documented
- ✅ **Documented current event rendering approach** - DOM structure, data flow documented
- ✅ **Listed all event types with representations** - 7 types with icons/colors/labels
- ✅ **Identified existing legend/key patterns** - Two patterns documented (built-in + React)
- ✅ **Documented current positioning/z-index approach** - Full layering hierarchy documented

## Deliverable Summary

This document provides complete foundational knowledge for:
1. Implementing layered event rendering (overlapping events handling)
2. Creating reusable legend components
3. Adding new event types with consistent styling
4. Understanding data flow for new features

**Next steps for layering implementation:**
- Current vertical offset is limited to ±6px for 3 events
- For heavy overlap, consider: z-index bands, hover expansion, or clustering
- Legend toggle button pattern is reusable for any overlay UI

**Files ready for modification:**
- `event-ribbon.ts` - Main rendering logic
- `extract-significant-events.ts` - Add new event detection patterns
- `EventTypeLegend.tsx` - Reusable legend component
- `components.css` - Global legend styles
