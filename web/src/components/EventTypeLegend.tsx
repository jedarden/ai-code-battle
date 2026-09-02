/**
 * EventTypeLegend Component
 *
 * A React component that displays event type → icon/color mapping
 * Shows icon, type name, and associated color for each event type
 *
 * @see SignificantEventType from extract-significant-events.ts
 */

import React from 'react';
import type { SignificantEventType } from '../extract-significant-events';

/**
 * Event type configuration with icon, color, and label
 */
interface EventTypeConfig {
  type: SignificantEventType;
  label: string;
  icon: string;
  color: string;
}

/**
 * Props for EventTypeLegend component
 */
export interface EventTypeLegendProps {
  /** Optional array of event types to display (shows all if not provided) */
  eventTypes?: SignificantEventType[];
  /** Optional additional CSS classes */
  className?: string;
}

/**
 * Event type style definitions matching the event-ribbon.ts configuration
 */
const EVENT_TYPE_STYLES: Record<SignificantEventType, { icon: string; color: string; label: string }> = {
  combat: { icon: '⚔️', color: '#ef4444', label: 'Combat' },
  core_capture: { icon: '🏰', color: '#3b82f6', label: 'Core Capture' },
  energy_milestone: { icon: '💎', color: '#06b6d4', label: 'Energy Milestone' },
  mass_death: { icon: '💀', color: '#6b7280', label: 'Mass Death' },
  momentum_shift: { icon: '📈', color: '#22c55e', label: 'Momentum Shift' },
  critical_moment: { icon: '🌟', color: '#eab308', label: 'Critical Moment' },
  spawn_wave: { icon: '🐣', color: '#a855f7', label: 'Spawn Wave' },
};

/**
 * EventTypeLegend component displays event type mappings with icons and colors
 *
 * @example
 * ```tsx
 * // Display all event types
 * <EventTypeLegend />
 *
 * // Display specific event types
 * <EventTypeLegend eventTypes={['combat', 'core_capture']} />
 * ```
 */
export const EventTypeLegend: React.FC<EventTypeLegendProps> = ({
  eventTypes,
  className = ''
}) => {
  // Use all event types if not provided
  const typesToDisplay = eventTypes || (Object.keys(EVENT_TYPE_STYLES) as SignificantEventType[]);

  // Map event types to their configurations
  const eventConfigs: EventTypeConfig[] = typesToDisplay.map(type => ({
    type,
    ...EVENT_TYPE_STYLES[type]
  }));

  return (
    <div className={`event-type-legend ${className}`.trim()}>
      <div className="event-type-legend-content">
        {eventConfigs.map(({ type, label, icon, color }) => (
          <div key={type} className="event-type-legend-item">
            <span
              className="event-type-legend-icon"
              style={{ color }}
              aria-hidden="true"
            >
              {icon}
            </span>
            <span className="event-type-legend-label">
              {label}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};

export default EventTypeLegend;